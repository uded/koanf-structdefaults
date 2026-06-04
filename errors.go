package structdefaults

import "errors"

// ErrUnsupported is returned by ReadBytes because the structdefaults provider
// operates on in-memory structs, not byte streams.
var ErrUnsupported = errors.New("structdefaults: ReadBytes is not supported")

// ErrUnsupportedType is returned when a field's Go type cannot carry a
// koanf-default value at all — e.g. slice, map, channel, or function fields
// with a default tag. This is a programmer error: fix the struct definition.
var ErrUnsupportedType = errors.New("structdefaults: unsupported field type")

// ErrInvalidValue is returned when a field's type IS supported but the tag
// value (or its post-substitution form) cannot be parsed — e.g. a malformed
// integer, a bad duration string, or a TextUnmarshaler that rejected the
// input. This is typically an operator/config error: fix the tag value or
// the env var feeding it. Use errors.As to recover the underlying parse
// error (*strconv.NumError, etc.).
var ErrInvalidValue = errors.New("structdefaults: invalid default value")

// ErrInvalidInput is returned when the target passed to New is nil, a
// non-struct, or a nil pointer-to-struct.
var ErrInvalidInput = errors.New("structdefaults: input must be a non-nil pointer to a struct")

// ErrInvalidConfig is returned by New when the Options struct is invalid —
// e.g. Delim is empty. This indicates a programmer error in the call site.
var ErrInvalidConfig = errors.New("structdefaults: invalid Options")

// ErrUnsetEnv is returned when a default value contains ${VAR} and the
// referenced environment variable is unset with no fallback provided.
// Use ${VAR:-} to opt into an empty-string fallback.
var ErrUnsetEnv = errors.New("structdefaults: env var unset with no fallback")

// ErrCyclicType is returned when the walker encounters a struct type that
// recursively references itself (directly or transitively). Loading defaults
// for such a type would cause unbounded recursion at startup.
var ErrCyclicType = errors.New("structdefaults: cyclic struct type")

// ErrLookupPanic is returned when a custom EnvLookup implementation panics
// during ${VAR} resolution. The library converts the panic into an error
// to honor Read's (map, error) return contract. Use
// errors.Is(err, ErrLookupPanic) to distinguish a misbehaving adapter
// (Vault, AWS Secrets Manager, etc.) from a missing variable
// (ErrUnsetEnv).
var ErrLookupPanic = errors.New("structdefaults: env lookup panicked")

// ErrInvalidTag is returned when a struct tag value cannot be used as a
// path segment — currently because it contains Options.Delim, which
// would silently create unintended nesting levels in the output map and
// collide non-deterministically with sibling fields. Fix the struct's
// koanf:"…" tag to use a single segment per field; if you need a nested
// path use one field per level.
var ErrInvalidTag = errors.New("structdefaults: invalid struct tag")
