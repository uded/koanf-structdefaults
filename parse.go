package structdefaults

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

var durationType = reflect.TypeOf(time.Duration(0))
var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()

// parseValue converts the raw tag string into a typed Go value for the given
// reflect.Type, following the dispatch order in spec §6.
//
// Returns (parsedValue, nil) on success or (nil, wrapped ErrUnsupportedType)
// on failure. The configPath and goPath are included in the error for context.
func parseValue(fieldType reflect.Type, raw, configPath, goPath string) (any, error) {
	// 1. time.Duration — must come before the TextUnmarshaler check because
	//    time.Duration is int64 under the hood and does not implement the interface.
	if fieldType == durationType {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("%w: config path %q (Go field %s): %v",
				ErrUnsupportedType, configPath, goPath, err)
		}
		return d, nil
	}

	// 2. encoding.TextUnmarshaler — value-receiver or pointer-receiver.
	if v, ok, err := tryTextUnmarshaler(fieldType, raw, configPath, goPath); ok {
		return v, err
	}

	// 3. Primitive kinds via strconv.
	return parsePrimitive(fieldType, raw, configPath, goPath)
}

// tryTextUnmarshaler checks whether fieldType (or *fieldType) implements
// encoding.TextUnmarshaler and, if so, unmarshals raw into a new value of that
// type. Returns (value, true, err) if the interface is implemented, or
// (nil, false, nil) if it is not.
func tryTextUnmarshaler(fieldType reflect.Type, raw, configPath, goPath string) (any, bool, error) {
	// Direct implementation (value receiver on fieldType).
	if fieldType.Implements(textUnmarshalerType) {
		v := reflect.New(fieldType).Elem()
		tu := v.Addr().Interface().(encoding.TextUnmarshaler)
		if err := tu.UnmarshalText([]byte(raw)); err != nil {
			return nil, true, fmt.Errorf("%w: config path %q (Go field %s): %v",
				ErrUnsupportedType, configPath, goPath, err)
		}
		return v.Interface(), true, nil
	}

	// Pointer-receiver implementation: *fieldType implements the interface.
	ptrType := reflect.PointerTo(fieldType)
	if ptrType.Implements(textUnmarshalerType) {
		ptr := reflect.New(fieldType)
		tu := ptr.Interface().(encoding.TextUnmarshaler)
		if err := tu.UnmarshalText([]byte(raw)); err != nil {
			return nil, true, fmt.Errorf("%w: config path %q (Go field %s): %v",
				ErrUnsupportedType, configPath, goPath, err)
		}
		// Return the concrete value (not the pointer) so the map holds the struct.
		return ptr.Elem().Interface(), true, nil
	}

	return nil, false, nil
}

// parsePrimitive handles string, bool, signed ints, unsigned ints, and floats.
func parsePrimitive(fieldType reflect.Type, raw, configPath, goPath string) (any, error) {
	wrap := func(err error) error {
		return fmt.Errorf("%w: config path %q (Go field %s): %v",
			ErrUnsupportedType, configPath, goPath, err)
	}

	switch fieldType.Kind() { //nolint:exhaustive // remaining kinds are unsupported
	case reflect.String:
		return raw, nil

	case reflect.Bool:
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, wrap(err)
		}
		return v, nil

	case reflect.Int:
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, wrap(err)
		}
		return int(v), nil
	case reflect.Int8:
		v, err := strconv.ParseInt(raw, 10, 8)
		if err != nil {
			return nil, wrap(err)
		}
		return int8(v), nil
	case reflect.Int16:
		v, err := strconv.ParseInt(raw, 10, 16)
		if err != nil {
			return nil, wrap(err)
		}
		return int16(v), nil
	case reflect.Int32:
		v, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return nil, wrap(err)
		}
		return int32(v), nil
	case reflect.Int64:
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, wrap(err)
		}
		return int64(v), nil

	case reflect.Uint:
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return nil, wrap(err)
		}
		return uint(v), nil
	case reflect.Uint8:
		v, err := strconv.ParseUint(raw, 10, 8)
		if err != nil {
			return nil, wrap(err)
		}
		return uint8(v), nil
	case reflect.Uint16:
		v, err := strconv.ParseUint(raw, 10, 16)
		if err != nil {
			return nil, wrap(err)
		}
		return uint16(v), nil
	case reflect.Uint32:
		v, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return nil, wrap(err)
		}
		return uint32(v), nil
	case reflect.Uint64:
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return nil, wrap(err)
		}
		return uint64(v), nil

	case reflect.Float32:
		v, err := strconv.ParseFloat(raw, 32)
		if err != nil {
			return nil, wrap(err)
		}
		return float32(v), nil
	case reflect.Float64:
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, wrap(err)
		}
		return float64(v), nil

	default:
		return nil, fmt.Errorf("%w: config path %q (Go field %s) has unsupported type %s",
			ErrUnsupportedType, configPath, goPath, fieldType)
	}
}
