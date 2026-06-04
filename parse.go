package structdefaults

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

var (
	durationType        = reflect.TypeFor[time.Duration]()
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
)

// parseCtx bundles the path strings used purely for error context.
// Carrying them as a named type keeps the parse* signatures short
// (one value-bearing arg + one context arg, satisfying the project's
// input-struct convention) and lets call sites read
// ctx.wrapInvalid(err) instead of repeating wrapInvalidValue(...).
type parseCtx struct {
	configPath, goPath string
}

// wrapInvalid produces an error chain that satisfies both
// errors.Is(err, ErrInvalidValue) and errors.As(err, &<underlyingType>),
// while ensuring the surface Error() string does not echo the raw
// post-substitution value. Without redaction, a default like
// koanf-default:"${DB_PASSWORD}" on a typed field would surface the
// resolved secret through err.Error(). See redactCause for the
// redaction strategy.
func (c parseCtx) wrapInvalid(cause error) error {
	return fmt.Errorf("%w: config path %q (Go field %s): %w",
		ErrInvalidValue, c.configPath, c.goPath, redactCause(cause))
}

// parseValue converts the raw tag string into a typed Go value for the given
// reflect.Type, following the dispatch order in the design spec:
//
//  1. Empty value on a non-string type → ErrInvalidValue with a clear
//     "omit the tag" message; protects users who wrote koanf-default:""
//     intending "no default" on an int / duration / etc.
//  2. time.Duration → time.ParseDuration (must come before TextUnmarshaler;
//     time.Duration is int64 and does not implement the interface).
//  3. encoding.TextUnmarshaler (value- or pointer-receiver).
//  4. Primitive kinds via strconv.
//  5. Otherwise: ErrUnsupportedType (the type cannot carry a default at all).
//
// Steps 2–4 wrap parse failures with ErrInvalidValue so callers can
// errors.Is(err, ErrInvalidValue). Step 5 returns ErrUnsupportedType
// (programmer error). Underlying parse errors are preserved via %w so
// errors.As(err, &target) reaches them, except that value-bearing fields
// of well-known error types (notably *strconv.NumError.Num) are
// redacted to prevent secret leakage.
func parseValue(fieldType reflect.Type, raw string, ctx parseCtx) (any, error) {
	// Empty default on a non-string typed field is almost always a mistake
	// (the user wrote koanf-default:"" intending "no default" but got an
	// unparseable empty string). Intercept with a clearer message before
	// the downstream parser produces a generic "parse failed" — but allow
	// TextUnmarshaler implementations to decide for themselves.
	if raw == "" && fieldType.Kind() != reflect.String && !isTextUnmarshaler(fieldType) {
		return nil, fmt.Errorf("%w: empty default value is not valid for type %s (config path %q, Go field %s); omit the koanf-default tag to skip this field",
			ErrInvalidValue, fieldType, ctx.configPath, ctx.goPath)
	}

	if fieldType == durationType {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, ctx.wrapInvalid(err)
		}
		return d, nil
	}

	if isTextUnmarshaler(fieldType) {
		return parseTextUnmarshaler(fieldType, raw, ctx)
	}

	return parsePrimitive(fieldType, raw, ctx)
}

// parseTextUnmarshaler instantiates a fresh *fieldType and invokes
// UnmarshalText on it. The pointer-receiver call covers both
// value-receiver and pointer-receiver implementations because *T's method
// set always contains T's methods.
//
// Caller must have verified isTextUnmarshaler(fieldType) first.
func parseTextUnmarshaler(fieldType reflect.Type, raw string, ctx parseCtx) (any, error) {
	ptr := reflect.New(fieldType)
	tu := ptr.Interface().(encoding.TextUnmarshaler)
	if err := tu.UnmarshalText([]byte(raw)); err != nil {
		return nil, ctx.wrapInvalid(err)
	}
	return ptr.Elem().Interface(), nil
}

// parsePrimitive handles string, bool, signed ints, unsigned ints, and floats.
// Failure paths return ErrInvalidValue wrapped with the underlying strconv
// error preserved via %w (subject to redaction — see redactCause).
func parsePrimitive(fieldType reflect.Type, raw string, ctx parseCtx) (any, error) {
	switch fieldType.Kind() {
	case reflect.String:
		return raw, nil
	case reflect.Bool:
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, ctx.wrapInvalid(err)
		}
		return v, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return parseSignedInt(fieldType, raw, ctx)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return parseUnsignedInt(fieldType, raw, ctx)
	case reflect.Float32, reflect.Float64:
		return parseFloat(fieldType, raw, ctx)
	default:
		return nil, fmt.Errorf("%w: config path %q (Go field %s) has unsupported type %s",
			ErrUnsupportedType, ctx.configPath, ctx.goPath, fieldType)
	}
}

// intBits maps each signed-int Kind to the bit size strconv.ParseInt expects.
var intBits = map[reflect.Kind]int{
	reflect.Int:   64,
	reflect.Int8:  8,
	reflect.Int16: 16,
	reflect.Int32: 32,
	reflect.Int64: 64,
}

// uintBits maps each unsigned-int Kind to the bit size strconv.ParseUint expects.
var uintBits = map[reflect.Kind]int{
	reflect.Uint:   64,
	reflect.Uint8:  8,
	reflect.Uint16: 16,
	reflect.Uint32: 32,
	reflect.Uint64: 64,
}

func parseSignedInt(fieldType reflect.Type, raw string, ctx parseCtx) (any, error) {
	v, err := strconv.ParseInt(raw, 10, intBits[fieldType.Kind()])
	if err != nil {
		return nil, ctx.wrapInvalid(err)
	}
	out := reflect.New(fieldType).Elem()
	out.SetInt(v)
	return out.Interface(), nil
}

func parseUnsignedInt(fieldType reflect.Type, raw string, ctx parseCtx) (any, error) {
	v, err := strconv.ParseUint(raw, 10, uintBits[fieldType.Kind()])
	if err != nil {
		return nil, ctx.wrapInvalid(err)
	}
	out := reflect.New(fieldType).Elem()
	out.SetUint(v)
	return out.Interface(), nil
}

func parseFloat(fieldType reflect.Type, raw string, ctx parseCtx) (any, error) {
	bits := 64
	if fieldType.Kind() == reflect.Float32 {
		bits = 32
	}
	v, err := strconv.ParseFloat(raw, bits)
	if err != nil {
		return nil, ctx.wrapInvalid(err)
	}
	out := reflect.New(fieldType).Elem()
	out.SetFloat(v)
	return out.Interface(), nil
}
