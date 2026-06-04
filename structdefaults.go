// Package structdefaults provides a koanf provider that reads koanf-default
// struct tags and emits a nested map[string]any of default values whose tree
// shape mirrors the koanf path layout. Load it as the first (lowest-priority)
// layer so that file, env, and flag providers override it naturally.
package structdefaults

import (
	"fmt"
	"os"
	"reflect"
	"strings"
)

const (
	defaultPathTag    = "koanf"
	defaultDefaultTag = "koanf-default"
)

// EnvLookup resolves an environment variable name to its value. Implementations
// must return (value, true) when the variable is set (even to an empty string)
// and ("", false) when it is unset. The default lookup is os.LookupEnv.
//
// Implementations must be safe for concurrent use; the provider holds a
// reference and may call it from any goroutine that triggers a Read.
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

	// Strict, when true, eagerly walks the entire struct at construction time
	// and surfaces any error (cyclic types, parse failures, unset env vars
	// without fallback) from New rather than waiting for the first Read call.
	Strict bool
}

// StructDefaults walks struct tags to produce a nested map of defaults
// suitable for koanf.Load. It is immutable after construction; safe for
// concurrent Read calls.
type StructDefaults struct {
	target any
	opts   Options
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
		if _, err := p.Read(); err != nil {
			return nil, err
		}
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
func (p *StructDefaults) Read() (map[string]any, error) {
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
	}
	if err := w.walk(v, "", ""); err != nil {
		return nil, err
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
}

// walk recursively visits every field of the struct value v. configPath is
// the delim-joined path accumulated so far; goPath is the dot-joined Go
// field path used in error messages.
func (w *walker) walk(v reflect.Value, configPath, goPath string) error {
	t := v.Type()

	// Cycle guard: two values of the same Go type share an identical
	// reflect.Type, so this catches both direct self-reference (Node.Next *Node)
	// and mutual recursion (A->B->A).
	if _, cycling := w.visiting[t]; cycling {
		path := configPath
		if path == "" {
			path = "<root>"
		}
		return fmt.Errorf("%w: %s (config path %q, Go field %s)",
			ErrCyclicType, t, path, goPath)
	}
	w.visiting[t] = struct{}{}
	defer delete(w.visiting, t)

	for i := range t.NumField() {
		field := t.Field(i)

		// Respect koanf:"-" — skip the field entirely.
		ptag := field.Tag.Get(w.pathTag)
		if ptag == "-" {
			continue
		}

		segment := pathSegment(field, ptag)
		cfgPath := joinPath(configPath, segment, w.delim)
		gp := joinGoPath(goPath, field.Name)

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
			// materialized in v.Field(i); only pointer fields need a
			// fresh zero instance because the original pointer may be nil.
			var sub reflect.Value
			if field.Type.Kind() == reflect.Pointer {
				sub = reflect.New(elemType).Elem()
			} else {
				sub = v.Field(i)
			}
			if err := w.walk(sub, recursePath, gp); err != nil {
				return err
			}
			continue
		}

		// Leaf field: only emit if DefaultTag is present.
		rawDefault, hasDefault := field.Tag.Lookup(w.defaultTag)
		if !hasDefault {
			continue
		}

		// Substitute ${VAR} / ${VAR:-fallback} before type dispatch so the
		// substitution is uniform across all field types.
		expanded, err := substituteEnv(rawDefault, w.lookup)
		if err != nil {
			return fmt.Errorf("%w (config path %q, Go field %s)", err, cfgPath, gp)
		}

		parsed, err := parseValue(field.Type, expanded, cfgPath, gp)
		if err != nil {
			return err
		}
		w.emit(cfgPath, parsed)
	}
	return nil
}

// emit places value at the delim-split path inside w.out, creating
// intermediate nested maps as needed. koanf's merge expects nested maps;
// emitting flat keys with delim characters in them collides
// non-deterministically with nested inputs from other providers during
// koanf's final Flatten pass.
func (w *walker) emit(configPath string, value any) {
	parts := strings.Split(configPath, w.delim)
	cur := w.out
	for i, part := range parts {
		if i == len(parts)-1 {
			cur[part] = value
			return
		}
		next, ok := cur[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			cur[part] = next
		}
		cur = next
	}
}

// pathSegment returns the config path segment for a field. It uses the
// pathTag value when non-empty, otherwise falls back to the Go field name.
func pathSegment(field reflect.StructField, ptag string) string {
	if ptag != "" {
		return ptag
	}
	return field.Name
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
