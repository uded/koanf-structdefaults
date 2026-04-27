// Package structdefaults provides a koanf provider that reads koanf-default
// struct tags and emits a flat map[string]any of default values keyed by their
// delim-joined config paths. Load it as the first (lowest-priority) layer so
// that file, env, and flag providers override it naturally.
package structdefaults

import (
	"fmt"
	"reflect"
)

const (
	defaultPathTag    = "koanf"
	defaultDefaultTag = "koanf-default"
)

// StructDefaults implements koanf.Provider by walking struct tags.
type StructDefaults struct {
	s          any
	pathTag    string
	defaultTag string
	delim      string
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
	}
}

// ReadBytes is not supported. Returns ErrUnsupported.
func (p *StructDefaults) ReadBytes() ([]byte, error) {
	return nil, ErrUnsupported
}

// Read walks the struct tags and returns a flat map[string]any keyed by
// delim-joined config paths. Only fields with an explicit koanf-default tag
// contribute entries (sparse output). Returns ErrInvalidInput if the stored
// value is not a non-nil pointer to a struct.
func (p *StructDefaults) Read() (map[string]any, error) {
	v, err := resolveInput(p.s)
	if err != nil {
		return nil, err
	}

	out := make(map[string]any)
	if err := walk(v, "", "", out, p.pathTag, p.defaultTag, p.delim); err != nil {
		return nil, err
	}
	return out, nil
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

// walk recursively visits every field of the struct value v, collecting
// defaults into out. configPath is the delim-joined path accumulated so far;
// goPath is the dot-joined Go field path for error messages.
func walk(v reflect.Value, configPath, goPath string, out map[string]any, pathTag, defaultTag, delim string) error {
	t := v.Type()

	for i := range t.NumField() {
		field := t.Field(i)

		// Respect koanf:"-" — skip the field entirely.
		ptag := field.Tag.Get(pathTag)
		if ptag == "-" {
			continue
		}

		// Build the config path segment for this field.
		segment := pathSegment(field, ptag)

		// Build the paths for this field.
		cfgPath := joinPath(configPath, segment, delim)
		gp := joinGoPath(goPath, field.Name)

		// Anonymous embedded struct: squash unless it has an explicit koanf tag
		// or it implements TextUnmarshaler (in which case it is a leaf).
		if field.Anonymous && ptag == "" {
			elemType, isStruct := derefToStruct(field.Type)
			if isStruct && !isTextUnmarshaler(field.Type) {
				// Squash: walk as if fields were on the parent, keeping parent paths.
				tmp := reflect.New(elemType).Elem()
				if err := walk(tmp, configPath, gp, out, pathTag, defaultTag, delim); err != nil {
					return err
				}
				continue
			}
		}

		// Recurse into struct or pointer-to-struct fields, but only when the
		// type does not implement encoding.TextUnmarshaler — those are treated
		// as opaque leaves parsed by parseValue (spec §6, rule 2).
		elemType, isStruct := derefToStruct(field.Type)
		if isStruct && !isTextUnmarshaler(field.Type) {
			// Allocate a fresh zero value for traversal — never touch user input.
			tmp := reflect.New(elemType).Elem()
			if err := walk(tmp, cfgPath, gp, out, pathTag, defaultTag, delim); err != nil {
				return err
			}
			continue
		}

		// Leaf field: only emit if koanf-default tag is present.
		rawDefault, hasDefault := field.Tag.Lookup(defaultTag)
		if !hasDefault {
			continue
		}

		parsed, err := parseValue(field.Type, rawDefault, cfgPath, gp)
		if err != nil {
			return err
		}
		out[cfgPath] = parsed
	}
	return nil
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

// derefToStruct dereferences a pointer type if needed and reports whether the
// resulting type is a struct. Returns (elemType, true) for struct or
// pointer-to-struct types, (t, false) otherwise.
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
