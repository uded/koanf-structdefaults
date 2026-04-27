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

// parseValue converts the raw tag string into a typed Go value for the given
// reflect.Type, following the dispatch order in the design spec:
//
//  1. time.Duration → time.ParseDuration (must come before TextUnmarshaler;
//     time.Duration is int64 and does not implement the interface).
//  2. encoding.TextUnmarshaler (value- or pointer-receiver).
//  3. Primitive kinds via strconv.
//  4. Otherwise: ErrUnsupportedType (the type cannot carry a default at all).
//
// Steps 1–3 wrap parse failures with ErrInvalidValue so callers can
// errors.Is(err, ErrInvalidValue). Step 4 returns ErrUnsupportedType
// (programmer error). Underlying parse errors are preserved via %w so
// errors.As(err, &target) reaches them.
func parseValue(fieldType reflect.Type, raw, configPath, goPath string) (any, error) {
	if fieldType == durationType {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, wrapInvalidValue(configPath, goPath, err)
		}
		return d, nil
	}

	if v, ok, err := tryTextUnmarshaler(fieldType, raw, configPath, goPath); ok {
		return v, err
	}

	return parsePrimitive(fieldType, raw, configPath, goPath)
}

// tryTextUnmarshaler checks whether *fieldType implements
// encoding.TextUnmarshaler and, if so, unmarshals raw into a new value of
// that type. Returns (value, true, err) if the interface is implemented, or
// (nil, false, nil) if it is not. Both receiver styles are handled by the
// pointer-receiver branch since *T's method set always contains T's methods.
func tryTextUnmarshaler(fieldType reflect.Type, raw, configPath, goPath string) (any, bool, error) {
	ptrType := reflect.PointerTo(fieldType)
	if !ptrType.Implements(textUnmarshalerType) {
		return nil, false, nil
	}
	ptr := reflect.New(fieldType)
	tu := ptr.Interface().(encoding.TextUnmarshaler)
	if err := tu.UnmarshalText([]byte(raw)); err != nil {
		return nil, true, wrapInvalidValue(configPath, goPath, err)
	}
	return ptr.Elem().Interface(), true, nil
}

// parsePrimitive handles string, bool, signed ints, unsigned ints, and floats.
// Failure paths return ErrInvalidValue wrapped with the underlying strconv
// error preserved via %w.
func parsePrimitive(fieldType reflect.Type, raw, configPath, goPath string) (any, error) {
	switch fieldType.Kind() {
	case reflect.String:
		return raw, nil
	case reflect.Bool:
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, wrapInvalidValue(configPath, goPath, err)
		}
		return v, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return parseSignedInt(fieldType, raw, configPath, goPath)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return parseUnsignedInt(fieldType, raw, configPath, goPath)
	case reflect.Float32, reflect.Float64:
		return parseFloat(fieldType, raw, configPath, goPath)
	default:
		return nil, fmt.Errorf("%w: config path %q (Go field %s) has unsupported type %s",
			ErrUnsupportedType, configPath, goPath, fieldType)
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

func parseSignedInt(fieldType reflect.Type, raw, configPath, goPath string) (any, error) {
	v, err := strconv.ParseInt(raw, 10, intBits[fieldType.Kind()])
	if err != nil {
		return nil, wrapInvalidValue(configPath, goPath, err)
	}
	out := reflect.New(fieldType).Elem()
	out.SetInt(v)
	return out.Interface(), nil
}

func parseUnsignedInt(fieldType reflect.Type, raw, configPath, goPath string) (any, error) {
	v, err := strconv.ParseUint(raw, 10, uintBits[fieldType.Kind()])
	if err != nil {
		return nil, wrapInvalidValue(configPath, goPath, err)
	}
	out := reflect.New(fieldType).Elem()
	out.SetUint(v)
	return out.Interface(), nil
}

func parseFloat(fieldType reflect.Type, raw, configPath, goPath string) (any, error) {
	bits := 64
	if fieldType.Kind() == reflect.Float32 {
		bits = 32
	}
	v, err := strconv.ParseFloat(raw, bits)
	if err != nil {
		return nil, wrapInvalidValue(configPath, goPath, err)
	}
	out := reflect.New(fieldType).Elem()
	out.SetFloat(v)
	return out.Interface(), nil
}

// wrapInvalidValue produces an error chain that satisfies both
// errors.Is(err, ErrInvalidValue) and errors.As(err, &<underlyingType>).
func wrapInvalidValue(configPath, goPath string, cause error) error {
	return fmt.Errorf("%w: config path %q (Go field %s): %w",
		ErrInvalidValue, configPath, goPath, cause)
}
