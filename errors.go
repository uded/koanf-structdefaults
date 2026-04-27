// Package structdefaults provides a koanf provider that reads koanf-default
// struct tags and emits a flat map[string]any of parsed default values.
package structdefaults

import "errors"

// ErrUnsupported is returned by ReadBytes because the structdefaults provider
// operates on in-memory structs, not byte streams.
var ErrUnsupported = errors.New("structdefaults: ReadBytes is not supported")

// ErrUnsupportedType is returned when a field carries a koanf-default tag but
// its type cannot be parsed (e.g. slices, maps, channels).
var ErrUnsupportedType = errors.New("structdefaults: unsupported field type")

// ErrInvalidInput is returned by Read when the value passed to Provider or
// ProviderWithTags is nil, a non-struct, or a nil pointer-to-struct.
var ErrInvalidInput = errors.New("structdefaults: input must be a non-nil pointer to a struct")
