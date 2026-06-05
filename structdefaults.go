// Package structdefaults provides a koanf provider that reads koanf-default
// struct tags and emits a nested map[string]any of default values whose tree
// shape mirrors the koanf path layout. Load it as the first (lowest-priority)
// layer so that file, env, and flag providers override it naturally.
package structdefaults

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
)

const (
	defaultPathTag    = "koanf"
	defaultDefaultTag = "koanf-default"
)

// EnvLookup resolves an environment variable name to its value. It is
// called once per ${VAR} reference found in koanf-default tag values
// during a walk; the resolved value substitutes in before the per-type
// parser sees it. See the README's "Environment variable substitution"
// section for the full ${VAR} / ${VAR:-fallback} grammar.
//
// Return (value, true) when the variable is set (even to an empty string)
// and ("", false) when it is unset. The default is os.LookupEnv. Custom
// lookups commonly target hermetic tests, secret stores (Vault, AWS
// Secrets Manager), or precedence layering.
//
// Implementations must be safe for concurrent use; the provider holds a
// reference and may call it from any goroutine that triggers a Read.
// Implementations that panic are recovered and surfaced as
// ErrLookupPanic, but for robust error reporting prefer to recover
// internally and return ("", false) on transient failures so callers
// see the standard ErrUnsetEnv path.
type EnvLookup func(name string) (string, bool)

// Options configures a StructDefaults provider. All fields are optional except
// Delim. Zero values trigger sensible defaults documented per field.
type Options struct {
	// Delim is the path separator used both to interpret the path tag values
	// and to nest entries in the output map. Required; empty Delim returns
	// ErrInvalidConfig from New.
	Delim string

	// PathTag is the struct tag whose value names the config path segment for
	// each field. Defaults to "koanf".
	PathTag string

	// DefaultTag is the struct tag whose value declares the field's default.
	// Defaults to "koanf-default".
	DefaultTag string

	// Lookup resolves ${VAR} references found in DefaultTag values. Defaults
	// to os.LookupEnv. Pass a custom function for hermetic tests, secret
	// stores (Vault, AWS Secrets Manager), or precedence layering.
	Lookup EnvLookup

	// Strict, when true, eagerly walks the entire struct at construction
	// time and surfaces every error (cyclic types, parse failures, unset
	// env vars without fallback, invalid tags) from New as a single
	// errors.Join — so one boot diagnoses all misconfigurations at once
	// rather than requiring repeated restarts. The validated map is
	// cached and returned from subsequent Read calls, treating the
	// configuration as frozen at construction time.
	Strict bool
}

// StructDefaults walks struct tags to produce a nested map of defaults
// suitable for koanf.Load. It is immutable after construction; safe for
// concurrent Read calls.
//
// When Options.Strict is true, New eagerly walks the target struct and
// caches the resulting map. Subsequent Read calls return the cached map
// without re-walking, which means env-var changes after construction are
// NOT picked up in Strict mode — Strict treats the configuration as
// frozen at boot. Callers must not mutate the returned map (it is the
// same instance returned to every Read).
//
// When Strict is false, every Read walks fresh so the most recent env-var
// state is observed; the returned map is a new instance per call and may
// be mutated freely.
type StructDefaults struct {
	target any
	opts   Options
	// cache is populated during Strict-mode construction and returned
	// from subsequent Read calls; nil for non-Strict providers.
	cache map[string]any
}

// New constructs a StructDefaults provider. It validates Options and the
// target struct shape, applying defaults for any zero-valued option fields.
// When Options.Strict is true, it additionally performs a full walk of the
// target struct so that any default-parsing or env-substitution errors
// surface immediately rather than at the first Read call.
//
// Returns ErrInvalidConfig if Options.Delim is empty, ErrInvalidInput if
// target is not a non-nil pointer to a struct, or any error produced by an
// eager Strict-mode walk (ErrCyclicType, ErrInvalidValue, ErrUnsetEnv,
// ErrUnsupportedType).
func New(target any, opts Options) (*StructDefaults, error) {
	if opts.Delim == "" {
		return nil, fmt.Errorf("%w: Options.Delim is required", ErrInvalidConfig)
	}
	if opts.PathTag == "" {
		opts.PathTag = defaultPathTag
	}
	if opts.DefaultTag == "" {
		opts.DefaultTag = defaultDefaultTag
	}
	if opts.Lookup == nil {
		opts.Lookup = os.LookupEnv
	}

	if _, err := resolveInput(target); err != nil {
		return nil, err
	}

	p := &StructDefaults{target: target, opts: opts}

	if opts.Strict {
		// Eager walk in accumulate mode so every misconfiguration in
		// the struct surfaces in a single errors.Join instead of one
		// per restart. Cache the validated map so subsequent Read
		// calls do not re-walk.
		out, err := p.walkAll(true)
		if err != nil {
			return nil, err
		}
		p.cache = out
	}

	return p, nil
}

// ReadBytes is not supported. Returns ErrUnsupported.
func (p *StructDefaults) ReadBytes() ([]byte, error) {
	return nil, ErrUnsupported
}

// Read walks the struct tags and returns a nested map[string]any whose tree
// shape mirrors the koanf path layout (split on Options.Delim). Only fields
// with an explicit DefaultTag contribute entries (sparse output).
//
// When Options.Strict is true, Read returns the cached map from the eager
// New-time walk; the same map instance is returned to every Read call and
// callers must not mutate it. When Strict is false, each Read walks fresh
// (errors fail-fast on the first encountered) and returns a new map
// instance.
func (p *StructDefaults) Read() (map[string]any, error) {
	if p.cache != nil {
		return p.cache, nil
	}
	return p.walkAll(false)
}

// walkAll runs the walker against the target struct in fail-fast mode
// (accumulate=false) or accumulate mode (accumulate=true). In
// accumulate mode every non-cycle error is collected; the final return
// is errors.Join of all collected errors. Cycle errors always bubble up
// immediately because continuing past a cycle would recurse without
// bound.
func (p *StructDefaults) walkAll(accumulate bool) (map[string]any, error) {
	v, err := resolveInput(p.target)
	if err != nil {
		return nil, err
	}

	w := &walker{
		pathTag:    p.opts.PathTag,
		defaultTag: p.opts.DefaultTag,
		delim:      p.opts.Delim,
		lookup:     p.opts.Lookup,
		out:        make(map[string]any),
		visiting:   make(map[reflect.Type]struct{}),
		accumulate: accumulate,
	}
	if err := w.walk(v, "", ""); err != nil {
		return nil, err
	}
	if len(w.errs) > 0 {
		return nil, errors.Join(w.errs...)
	}
	return w.out, nil
}

// resolveInput validates that target is a non-nil pointer to a struct and
// returns the dereferenced reflect.Value.
func resolveInput(target any) (reflect.Value, error) {
	if target == nil {
		return reflect.Value{}, fmt.Errorf("%w: got nil", ErrInvalidInput)
	}
	v := reflect.ValueOf(target)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return reflect.Value{}, fmt.Errorf("%w: got nil pointer", ErrInvalidInput)
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("%w: got %s", ErrInvalidInput, v.Kind())
	}
	return v, nil
}

// walker carries the immutable per-Read configuration through the recursive
// traversal. Holding state here keeps walk's signature small.
type walker struct {
	pathTag, defaultTag, delim string
	lookup                     EnvLookup
	out                        map[string]any
	// visiting tracks struct types currently on the recursion stack for
	// cycle detection — type Node struct { Next *Node } would otherwise
	// recurse forever.
	visiting map[reflect.Type]struct{}
	// accumulate, when true, makes the walker collect non-cycle errors
	// in errs and continue walking instead of returning early. Used by
	// Strict mode so a single New call surfaces every misconfiguration.
	accumulate bool
	errs       []error
}

// fail records err. In accumulate mode it appends to w.errs and returns
// nil so the walker continues to the next field; in fail-fast mode it
// returns err for immediate propagation up the recursion stack.
func (w *walker) fail(err error) error {
	if w.accumulate {
		w.errs = append(w.errs, err)
		return nil
	}
	return err
}

// walk recursively visits every field of the struct value v. configPath is
// the delim-joined path accumulated so far; goPath is the dot-joined Go
// field path used in error messages.
func (w *walker) walk(v reflect.Value, configPath, goPath string) error {
	t := v.Type()

	// Cycle guard: two values of the same Go type share an identical
	// reflect.Type, so this catches both direct self-reference (Node.Next *Node)
	// and mutual recursion (A->B->A). Always fail-fast — continuing past a
	// cycle would recurse without bound regardless of accumulate mode.
	if _, cycling := w.visiting[t]; cycling {
		path := configPath
		if path == "" {
			path = "<root>"
		}
		return fmt.Errorf("%w: %s at config path %q (Go field %s)",
			ErrCyclicType, t, path, goPath)
	}
	w.visiting[t] = struct{}{}
	defer delete(w.visiting, t)

	for i := range t.NumField() {
		if err := w.walkField(v, t.Field(i), i, configPath, goPath); err != nil {
			return err
		}
	}
	return nil
}

// walkField processes a single struct field at index i in v. It returns a
// non-nil error only in fail-fast mode (and for cycle errors propagated
// from a recursive walk); in accumulate mode all non-cycle errors are
// appended to w.errs and walkField returns nil so the caller continues.
func (w *walker) walkField(v reflect.Value, field reflect.StructField, i int, configPath, goPath string) error {
	ptag := field.Tag.Get(w.pathTag)
	// koanf:"-" (exactly the single character) skips the field entirely,
	// overriding any koanf-default tag. Only the exact value "-" triggers
	// the skip; koanf:"--" or koanf:"-,omitempty" are treated as literal
	// path segments, not skips.
	if ptag == "-" {
		return nil
	}

	// Use the tag value as the path segment; fall back to the Go field name.
	segment := ptag
	if segment == "" {
		segment = field.Name
	}
	gp := joinGoPath(goPath, field.Name)
	if strings.Contains(segment, w.delim) {
		return w.fail(fmt.Errorf("%w: tag value %q contains delim %q (Go field %s)",
			ErrInvalidTag, segment, w.delim, gp))
	}
	cfgPath := joinPath(configPath, segment, w.delim)

	// Recurse into struct or pointer-to-struct fields unless they implement
	// encoding.TextUnmarshaler (those are leaves parsed by parseValue).
	// Anonymous embedded fields without an explicit path tag squash into
	// the parent path; everything else nests under cfgPath.
	elemType, isStruct := derefToStruct(field.Type)
	if isStruct && !isTextUnmarshaler(field.Type) {
		recursePath := cfgPath
		if field.Anonymous && ptag == "" {
			recursePath = configPath
		}
		// For non-pointer struct fields the value is already
		// materialized in v.Field(i); only pointer fields need a fresh
		// zero instance because the original pointer may be nil.
		var sub reflect.Value
		if field.Type.Kind() == reflect.Pointer {
			sub = reflect.New(elemType).Elem()
		} else {
			sub = v.Field(i)
		}
		return w.walk(sub, recursePath, gp)
	}

	// Leaf field: only emit if DefaultTag is present.
	rawDefault, hasDefault := field.Tag.Lookup(w.defaultTag)
	if !hasDefault {
		return nil
	}

	// Substitute ${VAR} / ${VAR:-fallback} before type dispatch so the
	// substitution is uniform across all field types.
	expanded, err := substituteEnv(rawDefault, w.lookup)
	if err != nil {
		return w.fail(fmt.Errorf("%w (config path %q, Go field %s)", err, cfgPath, gp))
	}

	parsed, err := parseValue(field.Type, expanded, parseCtx{configPath: cfgPath, goPath: gp})
	if err != nil {
		return w.fail(err)
	}
	if err := w.emit(cfgPath, parsed); err != nil {
		return w.fail(err)
	}
	return nil
}

// emit places value at the delim-split path inside w.out, creating
// intermediate nested maps as needed. koanf's merge expects nested maps;
// emitting flat keys with delim characters in them collides
// non-deterministically with nested inputs from other providers during
// koanf's final Flatten pass.
//
// Returns ErrInvalidTag wrapped with location context if the path would
// overwrite an existing entry — either descending through an existing
// leaf (intermediate-part collision) or replacing an existing value at
// the final segment (duplicate-path collision). Both shapes signal a
// struct-tag bug (two fields contributing to overlapping paths) that
// would otherwise corrupt w.out silently.
func (w *walker) emit(configPath string, value any) error {
	parts := strings.Split(configPath, w.delim)
	cur := w.out
	for i, part := range parts {
		if i == len(parts)-1 {
			if existing, ok := cur[part]; ok {
				return fmt.Errorf("%w: duplicate config path %q (existing entry of type %T)",
					ErrInvalidTag, configPath, existing)
			}
			cur[part] = value
			return nil
		}
		next, ok := cur[part]
		if !ok {
			sub := make(map[string]any)
			cur[part] = sub
			cur = sub
			continue
		}
		nextMap, isMap := next.(map[string]any)
		if !isMap {
			return fmt.Errorf("%w: path %q overlaps existing leaf at %q (type %T)",
				ErrInvalidTag, configPath, strings.Join(parts[:i+1], w.delim), next)
		}
		cur = nextMap
	}
	return nil
}

// joinPath concatenates parent and child path segments with delim.
// If parent is empty the child is returned as-is (root level).
func joinPath(parent, child, delim string) string {
	if parent == "" {
		return child
	}
	return parent + delim + child
}

// joinGoPath concatenates parent and child Go field names with ".".
func joinGoPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

// derefToStruct dereferences a pointer type if needed and reports whether
// the resulting type is a struct.
func derefToStruct(t reflect.Type) (reflect.Type, bool) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t, t.Kind() == reflect.Struct
}

// isTextUnmarshaler reports whether t or *t implements encoding.TextUnmarshaler.
func isTextUnmarshaler(t reflect.Type) bool {
	return t.Implements(textUnmarshalerType) || reflect.PointerTo(t).Implements(textUnmarshalerType)
}
