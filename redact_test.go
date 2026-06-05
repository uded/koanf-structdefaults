package structdefaults_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	sd "github.com/uded/koanf-structdefaults"
)

// TestRedactionPropertyAcrossFieldTypes is a regression guard for the
// redaction-allowlist class of bug. It asserts two properties for every
// supported field type:
//
//  1. Surface contract: err.Error() must never echo the canary value
//     resolved via ${VAR}, regardless of which parser fails.
//  2. Typed-As contract: when the error chain reaches a well-known
//     stdlib error type (currently *strconv.NumError and
//     *time.ParseError), the value-bearing field on that type must be
//     scrubbed so callers using errors.As cannot recover the secret
//     through the introspection path either.
//
// Without this property test, adding support for a new third-party
// error type (e.g. *json.SyntaxError, *url.Error) could silently
// regress the redaction guarantee — the existing implementation only
// explicitly redacts *strconv.NumError.Num.
func TestRedactionPropertyAcrossFieldTypes(t *testing.T) {
	t.Parallel()
	const canary = "__SECRET_CANARY_a83bc47__"
	lookup := func(name string) (string, bool) {
		if name == "CANARY" {
			return canary, true
		}
		return "", false
	}

	cases := []struct {
		name    string
		target  any
		numErr  bool // expect *strconv.NumError reachable via errors.As; Num must be empty
		timeErr bool // expect *time.ParseError reachable via errors.As; Value must be empty
	}{
		{name: "bool", target: &struct {
			V bool `koanf:"v" koanf-default:"${CANARY}"`
		}{}, numErr: true},
		{name: "int", target: &struct {
			V int `koanf:"v" koanf-default:"${CANARY}"`
		}{}, numErr: true},
		{name: "int8", target: &struct {
			V int8 `koanf:"v" koanf-default:"${CANARY}"`
		}{}, numErr: true},
		{name: "int16", target: &struct {
			V int16 `koanf:"v" koanf-default:"${CANARY}"`
		}{}, numErr: true},
		{name: "int32", target: &struct {
			V int32 `koanf:"v" koanf-default:"${CANARY}"`
		}{}, numErr: true},
		{name: "int64", target: &struct {
			V int64 `koanf:"v" koanf-default:"${CANARY}"`
		}{}, numErr: true},
		{name: "uint", target: &struct {
			V uint `koanf:"v" koanf-default:"${CANARY}"`
		}{}, numErr: true},
		{name: "uint8", target: &struct {
			V uint8 `koanf:"v" koanf-default:"${CANARY}"`
		}{}, numErr: true},
		{name: "uint16", target: &struct {
			V uint16 `koanf:"v" koanf-default:"${CANARY}"`
		}{}, numErr: true},
		{name: "uint32", target: &struct {
			V uint32 `koanf:"v" koanf-default:"${CANARY}"`
		}{}, numErr: true},
		{name: "uint64", target: &struct {
			V uint64 `koanf:"v" koanf-default:"${CANARY}"`
		}{}, numErr: true},
		{name: "float32", target: &struct {
			V float32 `koanf:"v" koanf-default:"${CANARY}"`
		}{}, numErr: true},
		{name: "float64", target: &struct {
			V float64 `koanf:"v" koanf-default:"${CANARY}"`
		}{}, numErr: true},
		{name: "duration", target: &struct {
			V time.Duration `koanf:"v" koanf-default:"${CANARY}"`
		}{}, timeErr: true},
		{name: "text_unmarshaler", target: &struct {
			V Color `koanf:"v" koanf-default:"${CANARY}"`
		}{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, err := sd.New(tc.target, sd.Options{Delim: ".", Lookup: lookup})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			_, err = p.Read()
			if err == nil {
				t.Fatal("expected parse error for unparseable canary, got nil")
			}
			if strings.Contains(err.Error(), canary) {
				t.Errorf("err.Error() leaked canary: %q", err.Error())
			}
			if tc.numErr {
				var ne *strconv.NumError
				if errors.As(err, &ne) && ne.Num != "" {
					t.Errorf("*strconv.NumError.Num leaked canary: %q", ne.Num)
				}
			}
			if tc.timeErr {
				var pe *time.ParseError
				if errors.As(err, &pe) {
					if pe.Value != "" {
						t.Errorf("*time.ParseError.Value leaked canary: %q", pe.Value)
					}
					if strings.Contains(pe.ValueElem, canary) || strings.Contains(pe.Message, canary) {
						t.Errorf("*time.ParseError leaked via ValueElem/Message: %+v", pe)
					}
				}
			}
		})
	}
}
