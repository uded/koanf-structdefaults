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

// ErrUnsetEnv is returned when a default value contains ${VAR} and the
// referenced environment variable is unset with no fallback provided.
// Use ${VAR:-} to opt into an empty-string fallback.
var ErrUnsetEnv = errors.New("structdefaults: env var unset with no fallback")

// ErrCyclicType is returned when the walker encounters a struct type that
// recursively references itself (directly or transitively). Loading defaults
// for such a type would cause unbounded recursion at startup.
var ErrCyclicType = errors.New("structdefaults: cyclic struct type")
