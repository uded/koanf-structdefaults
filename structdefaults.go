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

// LookupFunc resolves an environment variable name to its value. Implementations
// should return (value, true) when the variable is set (even to an empty string)
// and ("", false) when it is unset. The default lookup is os.LookupEnv.
type LookupFunc func(name string) (string, bool)

// StructDefaults implements koanf.Provider by walking struct tags.
type StructDefaults struct {
	s          any
	pathTag    string
	defaultTag string
	delim      string
	lookup     LookupFunc
}

// Provider returns a StructDefaults provider using the conventional tag names
// ("koanf" for paths, "koanf-default" for defaults).
func Provider(s any, delim string) *StructDefaults {
	return ProviderWithTags(s, defaultPathTag, defaultDefaultTag, delim)
}

// ProviderWithTags returns a StructDefaults provider with caller-supplied tag
// names. An empty string for either tag falls back to the library default.
func ProviderWithTags(s any, pathTag, defaultTag, delim string) *StructDefaults {
	if pathTag == "" {
		pathTag = defaultPathTag
	}
	if defaultTag == "" {
		defaultTag = defaultDefaultTag
	}
	return &StructDefaults{
		s:          s,
		pathTag:    pathTag,
		defaultTag: defaultTag,
		delim:      delim,
		lookup:     os.LookupEnv,
	}
}

// WithLookup overrides the env-var lookup function used to resolve ${VAR}
// references in koanf-default tags. Pass nil to restore os.LookupEnv. Configure
// before the first Read; the provider is otherwise read-only.
func (p *StructDefaults) WithLookup(fn LookupFunc) *StructDefaults {
	if fn == nil {
		fn = os.LookupEnv
	}
	p.lookup = fn
	return p
}

// ReadBytes is not supported. Returns ErrUnsupported.
func (p *StructDefaults) ReadBytes() ([]byte, error) {
	return nil, ErrUnsupported
}

// Read walks the struct tags and returns a nested map[string]any whose tree
// shape mirrors the koanf path layout (split on the configured delim). Only
// fields with an explicit koanf-default tag contribute entries (sparse
// output). Returns ErrInvalidInput if the stored value is not a non-nil
// pointer to a struct.
func (p *StructDefaults) Read() (map[string]any, error) {
	v, err := resolveInput(p.s)
	if err != nil {
		return nil, err
	}

	w := &walker{
		pathTag:    p.pathTag,
		defaultTag: p.defaultTag,
		delim:      p.delim,
		lookup:     p.lookup,
		out:        make(map[string]any),
		visiting:   make(map[reflect.Type]struct{}),
	}
	if err := w.walk(v, "", ""); err != nil {
		return nil, err
	}
	return w.out, nil
}

// resolveInput validates that s is a non-nil pointer to a struct and returns
// the dereferenced reflect.Value.
func resolveInput(s any) (reflect.Value, error) {
	if s == nil {
		return reflect.Value{}, fmt.Errorf("%w: got nil", ErrInvalidInput)
	}
	v := reflect.ValueOf(s)
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
	lookup                     LookupFunc
	out                        map[string]any
	// visiting tracks struct types currently on the recursion stack for
	// cycle detection — type Node struct { Next *Node } would otherwise
	// recurse forever.
	visiting map[reflect.Type]struct{}
}

// walk recursively visits every field of the struct value v. configPath is
// the delim-joined path accumulated so far; goPath is the dot-joined Go
// field path used for error messages.
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

		// Anonymous embedded struct: squash unless it has an explicit path tag
		// or it implements TextUnmarshaler (in which case it is a leaf).
		if field.Anonymous && ptag == "" {
			elemType, isStruct := derefToStruct(field.Type)
			if isStruct && !isTextUnmarshaler(field.Type) {
				tmp := reflect.New(elemType).Elem()
				if err := w.walk(tmp, configPath, gp); err != nil {
					return err
				}
				continue
			}
		}

		// Recurse into struct or pointer-to-struct fields, but only when the
		// type does not implement encoding.TextUnmarshaler — those are treated
		// as opaque leaves parsed by parseValue.
		elemType, isStruct := derefToStruct(field.Type)
		if isStruct && !isTextUnmarshaler(field.Type) {
			tmp := reflect.New(elemType).Elem()
			if err := w.walk(tmp, cfgPath, gp); err != nil {
				return err
			}
			continue
		}

		// Leaf field: only emit if koanf-default tag is present.
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
		emit(w.out, cfgPath, parsed, w.delim)
	}
	return nil
}

// emit places value at the delim-split path inside out, creating intermediate
// nested maps as needed. koanf's merge expects nested maps; emitting flat
// keys with delim characters in them collides non-deterministically with
// nested inputs from other providers during koanf's final Flatten pass.
func emit(out map[string]any, configPath string, value any, delim string) {
	parts := strings.Split(configPath, delim)
	cur := out
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

// pathSegment returns the config path segment for a field. It uses the pathTag
// value when non-empty, otherwise falls back to the Go field name.
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
