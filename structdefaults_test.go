package structdefaults_test

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	sd "github.com/uded/koanf-structdefaults"
)

// compile-time assertion: *StructDefaults satisfies koanf.Provider
var _ koanf.Provider = (*sd.StructDefaults)(nil)

// ---- helpers ----------------------------------------------------------------

func mustRead(t *testing.T, p *sd.StructDefaults) map[string]any {
	t.Helper()
	m, err := p.Read()
	if err != nil {
		t.Fatalf("Read() unexpected error: %v", err)
	}
	return m
}

// mustNew constructs a provider with the conventional `.` delim, failing the
// test if construction errors. Use the long form `sd.New(t, sd.Options{...})`
// when the test exercises non-default options.
func mustNew(t *testing.T, target any) *sd.StructDefaults {
	t.Helper()
	p, err := sd.New(target, sd.Options{Delim: "."})
	if err != nil {
		t.Fatalf("New(...) unexpected error: %v", err)
	}
	return p
}

// getPath traverses a nested map[string]any using a dot-separated key path.
// Returns (value, true) if found, (nil, false) otherwise.
func getPath(m map[string]any, path string) (any, bool) {
	parts := strings.SplitN(path, ".", 2)
	v, ok := m[parts[0]]
	if !ok {
		return nil, false
	}
	if len(parts) == 1 {
		return v, true
	}
	nested, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	return getPath(nested, parts[1])
}

// assertPath checks that m has the given nested path equal to want.
func assertPath(t *testing.T, m map[string]any, path string, want any) {
	t.Helper()
	got, ok := getPath(m, path)
	if !ok {
		t.Fatalf("path %q missing from map", path)
	}
	if got != want {
		t.Errorf("m[%q] = %v (%T), want %v (%T)", path, got, got, want, want)
	}
}

// hasPath reports whether the given nested path exists in m.
func hasPath(m map[string]any, path string) bool {
	_, ok := getPath(m, path)
	return ok
}

// ---- custom TextUnmarshaler type (pointer receiver) -------------------------

type Color struct{ R, G, B uint8 }

// UnmarshalText implements encoding.TextUnmarshaler via pointer receiver.
// Accepts "R,G,B" notation, e.g. "255,128,0".
func (c *Color) UnmarshalText(b []byte) error {
	parts := splitComma(string(b))
	if len(parts) != 3 {
		return errors.New("bad color: " + string(b))
	}
	r, err := atoiSimple(parts[0])
	if err != nil {
		return err
	}
	g, err := atoiSimple(parts[1])
	if err != nil {
		return err
	}
	bl, err := atoiSimple(parts[2])
	if err != nil {
		return err
	}
	c.R, c.G, c.B = uint8(r), uint8(g), uint8(bl)
	return nil
}

func splitComma(s string) []string {
	var out []string
	cur := ""
	for _, ch := range s {
		if ch == ',' {
			out = append(out, cur)
			cur = ""
		} else {
			cur += string(ch)
		}
	}
	out = append(out, cur)
	return out
}

func atoiSimple(s string) (int, error) {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, errors.New("not a digit: " + string(ch))
		}
		n = n*10 + int(ch-'0')
	}
	return n, nil
}

// ---- test structs -----------------------------------------------------------

type ScalarDefaults struct {
	S   string  `koanf:"s"   koanf-default:"hello"`
	B   bool    `koanf:"b"   koanf-default:"true"`
	I   int     `koanf:"i"   koanf-default:"-1"`
	I8  int8    `koanf:"i8"  koanf-default:"-8"`
	I16 int16   `koanf:"i16" koanf-default:"-16"`
	I32 int32   `koanf:"i32" koanf-default:"-32"`
	I64 int64   `koanf:"i64" koanf-default:"-64"`
	U   uint    `koanf:"u"   koanf-default:"1"`
	U8  uint8   `koanf:"u8"  koanf-default:"8"`
	U16 uint16  `koanf:"u16" koanf-default:"16"`
	U32 uint32  `koanf:"u32" koanf-default:"32"`
	U64 uint64  `koanf:"u64" koanf-default:"64"`
	F32 float32 `koanf:"f32" koanf-default:"3.2"`
	F64 float64 `koanf:"f64" koanf-default:"6.4"`
}

type DurationStruct struct {
	Timeout time.Duration `koanf:"timeout" koanf-default:"30s"`
}

type IPStruct struct {
	Addr net.IP `koanf:"addr" koanf-default:"192.168.1.1"`
}

type ColorStruct struct {
	Primary Color `koanf:"primary" koanf-default:"255,128,0"`
}

type NestedL2 struct {
	Value int `koanf:"value" koanf-default:"42"`
}

type NestedL1 struct {
	Name  string   `koanf:"name"  koanf-default:"middle"`
	Inner NestedL2 `koanf:"inner"`
}

type DeepNested struct {
	Level NestedL1 `koanf:"level"`
}

type PtrInner struct {
	X int `koanf:"x" koanf-default:"99"`
}

type WithPtr struct {
	Inner *PtrInner `koanf:"inner"`
}

type Embedded struct {
	EmbeddedField string `koanf:"ef" koanf-default:"emb"`
}

type SquashParent struct {
	Embedded
	Own string `koanf:"own" koanf-default:"mine"`
}

type NamedEmbedParent struct {
	Embedded `koanf:"nested"`
	Own      string `koanf:"own" koanf-default:"mine"`
}

type SkipField struct {
	Skip    string `koanf:"-"       koanf-default:"should-not-appear"`
	Present string `koanf:"present" koanf-default:"here"`
}

type NoKoanfTag struct {
	GoName string `koanf-default:"fallback"`
}

type EmptyStringDefault struct {
	S string `koanf:"s" koanf-default:""`
}

type BadInt struct {
	N int `koanf:"n" koanf-default:"8O8O"`
}

type BadDuration struct {
	D time.Duration `koanf:"d" koanf-default:"forty seconds"`
}

type UnsupportedSlice struct {
	Sl []string `koanf:"sl" koanf-default:"a,b,c"`
}

type EmptyStruct struct{}

type CustomTagStruct struct {
	Host string `mypath:"host" mydefault:"myhost"`
	Port int    `mypath:"port" mydefault:"9090"`
}

// ---- tests ------------------------------------------------------------------

func TestScalars(t *testing.T) {
	t.Parallel()
	m := mustRead(t, mustNew(t, &ScalarDefaults{}))

	cases := []struct {
		key  string
		want any
	}{
		{"s", "hello"},
		{"b", true},
		{"i", int(-1)},
		{"i8", int8(-8)},
		{"i16", int16(-16)},
		{"i32", int32(-32)},
		{"i64", int64(-64)},
		{"u", uint(1)},
		{"u8", uint8(8)},
		{"u16", uint16(16)},
		{"u32", uint32(32)},
		{"u64", uint64(64)},
		{"f32", float32(3.2)},
		{"f64", float64(6.4)},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()
			assertPath(t, m, tc.key, tc.want)
		})
	}
}

func TestDuration(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		m := mustRead(t, mustNew(t, &DurationStruct{}))
		assertPath(t, m, "timeout", 30*time.Second)
	})
	t.Run("parse_error", func(t *testing.T) {
		t.Parallel()
		_, err := mustNew(t, &BadDuration{}).Read()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, sd.ErrInvalidValue) {
			t.Errorf("expected ErrInvalidValue, got: %v", err)
		}
	})
}

func TestTextUnmarshaler(t *testing.T) {
	t.Parallel()
	t.Run("net.IP_value_receiver_via_pointer", func(t *testing.T) {
		t.Parallel()
		m := mustRead(t, mustNew(t, &IPStruct{}))
		got, ok := getPath(m, "addr")
		if !ok {
			t.Fatal("key 'addr' missing")
		}
		ip, ok2 := got.(net.IP)
		if !ok2 {
			t.Fatalf("expected net.IP, got %T", got)
		}
		want := net.ParseIP("192.168.1.1")
		if !ip.Equal(want) {
			t.Errorf("got %v, want %v", ip, want)
		}
	})
	t.Run("custom_pointer_receiver", func(t *testing.T) {
		t.Parallel()
		m := mustRead(t, mustNew(t, &ColorStruct{}))
		got, ok := getPath(m, "primary")
		if !ok {
			t.Fatal("key 'primary' missing")
		}
		c, ok2 := got.(Color)
		if !ok2 {
			t.Fatalf("expected Color, got %T", got)
		}
		if c.R != 255 || c.G != 128 || c.B != 0 {
			t.Errorf("got %+v, want {255,128,0}", c)
		}
	})
}

func TestNestedStructs(t *testing.T) {
	t.Parallel()
	m := mustRead(t, mustNew(t, &DeepNested{}))

	cases := []struct {
		path string
		want any
	}{
		{"level.name", "middle"},
		{"level.inner.value", int(42)},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			assertPath(t, m, tc.path, tc.want)
		})
	}
}

func TestPointerToStruct(t *testing.T) {
	t.Parallel()
	t.Run("nil_pointer_field", func(t *testing.T) {
		t.Parallel()
		input := &WithPtr{Inner: nil}
		m := mustRead(t, mustNew(t, input))
		if input.Inner != nil {
			t.Error("Read() mutated input: Inner should remain nil")
		}
		assertPath(t, m, "inner.x", int(99))
	})
	t.Run("non_nil_pointer_field", func(t *testing.T) {
		t.Parallel()
		input := &WithPtr{Inner: &PtrInner{X: 7}}
		m := mustRead(t, mustNew(t, input))
		// Default value, not the field's actual value.
		assertPath(t, m, "inner.x", int(99))
		if input.Inner.X != 7 {
			t.Error("Read() mutated input: Inner.X should remain 7")
		}
	})
}

func TestAnonymousEmbedded(t *testing.T) {
	t.Parallel()
	t.Run("squash", func(t *testing.T) {
		t.Parallel()
		m := mustRead(t, mustNew(t, &SquashParent{}))
		// ef should be at top level (squashed), not under an "Embedded" key.
		if !hasPath(m, "ef") {
			t.Error("expected 'ef' at top level (squash)")
		}
		if !hasPath(m, "own") {
			t.Error("expected 'own' at top level")
		}
	})
	t.Run("named_embed_nests", func(t *testing.T) {
		t.Parallel()
		m := mustRead(t, mustNew(t, &NamedEmbedParent{}))
		// ef should be nested under "nested".
		if !hasPath(m, "nested.ef") {
			t.Errorf("expected 'nested.ef', got keys: %v", flatKeys(m, ""))
		}
		if !hasPath(m, "own") {
			t.Error("expected 'own' at top level")
		}
	})
}

func TestSkipField(t *testing.T) {
	t.Parallel()
	m := mustRead(t, mustNew(t, &SkipField{}))
	// Neither the Go field name nor any synthetic key should appear.
	if hasPath(m, "skip") || hasPath(m, "Skip") {
		t.Error("skipped field should not appear in output")
	}
	assertPath(t, m, "present", "here")
}

func TestMissingKoanfTag(t *testing.T) {
	t.Parallel()
	m := mustRead(t, mustNew(t, &NoKoanfTag{}))
	got, ok := getPath(m, "GoName")
	if !ok || got != "fallback" {
		t.Errorf("expected m['GoName']='fallback', got %v (keys: %v)", got, flatKeys(m, ""))
	}
}

func TestEmptyStringDefault(t *testing.T) {
	t.Parallel()
	m := mustRead(t, mustNew(t, &EmptyStringDefault{}))
	got, ok := getPath(m, "s")
	if !ok {
		t.Fatal("key 's' missing — empty-string default must be emitted")
	}
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

// TestDelimInTagRejected verifies that a tag value containing Options.Delim
// returns ErrInvalidTag rather than silently nesting under the delim
// boundaries. Without this validation a tag like `koanf:"foo.bar"` with
// Delim="." would produce an unintended foo→bar tree fragment that
// collides with sibling fields.
func TestDelimInTagRejected(t *testing.T) {
	t.Parallel()
	type bad struct {
		X int `koanf:"foo.bar" koanf-default:"42"`
	}
	_, err := mustNew(t, &bad{}).Read()
	if err == nil {
		t.Fatal("expected ErrInvalidTag, got nil")
	}
	if !errors.Is(err, sd.ErrInvalidTag) {
		t.Errorf("want ErrInvalidTag, got %v", err)
	}
	if !strings.Contains(err.Error(), "foo.bar") {
		t.Errorf("error should name the offending tag value, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "X") {
		t.Errorf("error should name the Go field, got %q", err.Error())
	}
}

// TestEmitDetectsCollidingPaths verifies that emit rejects two flavors of
// struct-tag bug rather than silently corrupting the output map:
//   - duplicate-final: two fields contribute the same full path.
//   - leaf-then-nest: a primitive field's path is a prefix of a later
//     sub-struct's path.
// Both shapes return ErrInvalidTag; the underlying bug is a struct
// definition mistake (overlapping path-tags) and the error message names
// the offending path.
// TestInterfaceFieldUnsupported verifies that an interface{}-typed field
// carrying a koanf-default tag surfaces ErrUnsupportedType, since the
// library cannot pick a concrete type to parse the default into.
func TestInterfaceFieldUnsupported(t *testing.T) {
	t.Parallel()
	type bad struct {
		X any `koanf:"x" koanf-default:"anything"`
	}
	_, err := mustNew(t, &bad{}).Read()
	if err == nil {
		t.Fatal("expected ErrUnsupportedType, got nil")
	}
	if !errors.Is(err, sd.ErrUnsupportedType) {
		t.Errorf("want ErrUnsupportedType, got %v", err)
	}
}

// TestAnonymousPointerEmbed verifies that an anonymous embedded *T squashes
// into the parent path just like an anonymous embedded T. The walker
// allocates a fresh zero instance via reflect.New so the original pointer
// may safely be nil at construction time — idiomatic for *sync.Mutex,
// *http.Request, and similar embeds.
func TestAnonymousPointerEmbed(t *testing.T) {
	t.Parallel()
	type Inner struct {
		Port int `koanf:"port" koanf-default:"8080"`
	}
	type Parent struct {
		*Inner
	}
	m := mustRead(t, mustNew(t, &Parent{}))
	got, ok := getPath(m, "port")
	if !ok {
		t.Fatalf("expected squashed key 'port' at root, got %v", m)
	}
	if got != 8080 {
		t.Errorf("port: got %v (%T), want 8080", got, got)
	}
}

// TestEmptyDefaultOnNonStringTypeRejected verifies that koanf-default:""
// on a typed field returns an explanatory ErrInvalidValue rather than the
// generic strconv-style parse failure. Empty defaults are only meaningful
// for string fields; on any other primitive, ints, durations, etc., the
// user almost certainly meant "no default" and should omit the tag.
func TestEmptyDefaultOnNonStringTypeRejected(t *testing.T) {
	t.Parallel()
	type bad struct {
		Port int `koanf:"port" koanf-default:""`
	}
	_, err := mustNew(t, &bad{}).Read()
	if err == nil {
		t.Fatal("expected ErrInvalidValue, got nil")
	}
	if !errors.Is(err, sd.ErrInvalidValue) {
		t.Errorf("want ErrInvalidValue, got %v", err)
	}
	if !strings.Contains(err.Error(), "empty default") {
		t.Errorf("error should explain the empty-default condition, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "omit") {
		t.Errorf("error should suggest the remediation (omit the tag), got %q", err.Error())
	}
}

func TestEmitDetectsCollidingPaths(t *testing.T) {
	t.Parallel()
	t.Run("duplicate_final_path", func(t *testing.T) {
		t.Parallel()
		type bad struct {
			A int `koanf:"foo" koanf-default:"1"`
			B int `koanf:"foo" koanf-default:"2"`
		}
		_, err := mustNew(t, &bad{}).Read()
		if err == nil {
			t.Fatal("expected ErrInvalidTag for duplicate path, got nil")
		}
		if !errors.Is(err, sd.ErrInvalidTag) {
			t.Errorf("want ErrInvalidTag, got %v", err)
		}
		if !strings.Contains(err.Error(), `"foo"`) {
			t.Errorf("error should name the colliding path, got %q", err.Error())
		}
	})

	t.Run("leaf_then_nest", func(t *testing.T) {
		t.Parallel()
		type inner struct {
			Port int `koanf:"port" koanf-default:"8080"`
		}
		type bad struct {
			Server string `koanf:"server" koanf-default:"localhost"`
			Sub    inner  `koanf:"server"`
		}
		_, err := mustNew(t, &bad{}).Read()
		if err == nil {
			t.Fatal("expected ErrInvalidTag for leaf-then-nest collision, got nil")
		}
		if !errors.Is(err, sd.ErrInvalidTag) {
			t.Errorf("want ErrInvalidTag, got %v", err)
		}
		if !strings.Contains(err.Error(), "server") {
			t.Errorf("error should name the offending segment, got %q", err.Error())
		}
	})
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   any
		wantErr error
	}{
		{"bad_int", &BadInt{}, sd.ErrInvalidValue},
		{"bad_duration", &BadDuration{}, sd.ErrInvalidValue},
		{"unsupported_slice", &UnsupportedSlice{}, sd.ErrUnsupportedType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := mustNew(t, tc.input).Read()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("expected %v, got: %v", tc.wantErr, err)
			}
		})
	}
}

// TestParseErrorPreservesUnderlying verifies that the %w error chain produced
// by parsePrimitive lets callers reach the underlying strconv error via
// errors.As, not just the ErrInvalidValue sentinel — AND that the raw input
// value has been stripped from the wrapped *strconv.NumError so secrets do
// not leak through the error surface.
func TestParseErrorPreservesUnderlying(t *testing.T) {
	t.Parallel()
	_, err := mustNew(t, &BadInt{}).Read()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sd.ErrInvalidValue) {
		t.Errorf("errors.Is(err, ErrInvalidValue) failed: %v", err)
	}
	var numErr *strconv.NumError
	if !errors.As(err, &numErr) {
		t.Fatalf("errors.As(err, *strconv.NumError) failed: %v", err)
	}
	if numErr.Num != "" {
		t.Errorf("strconv.NumError.Num = %q, want empty (value should be redacted)", numErr.Num)
	}
	if numErr.Func == "" {
		t.Errorf("strconv.NumError.Func should be preserved, got empty")
	}
	if strings.Contains(err.Error(), "8O8O") {
		t.Errorf("err.Error() leaked raw input value: %q", err.Error())
	}
}

// TestParseErrorRedactsEnvValue verifies the security contract that values
// resolved from ${VAR} substitution do not leak into err.Error() when the
// resulting string fails to parse for the target field's type. This is the
// realistic failure mode for koanf-default:"${DB_PASSWORD}" or similar
// secret-bearing defaults bound to typed fields.
func TestParseErrorRedactsEnvValue(t *testing.T) {
	t.Parallel()
	type secretCfg struct {
		Port int `koanf:"port" koanf-default:"${SUPER_SECRET}"`
	}
	const secret = "p@ssw0rd-not-an-int-AB123"
	lookup := func(name string) (string, bool) {
		if name == "SUPER_SECRET" {
			return secret, true
		}
		return "", false
	}
	p, err := sd.New(&secretCfg{}, sd.Options{Delim: ".", Lookup: lookup})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = p.Read()
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !errors.Is(err, sd.ErrInvalidValue) {
		t.Fatalf("want ErrInvalidValue, got %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("err.Error() leaked secret %q: full message = %q", secret, err.Error())
	}
}

func TestEmptyStruct(t *testing.T) {
	t.Parallel()
	m := mustRead(t, mustNew(t, &EmptyStruct{}))
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

func TestCustomTags(t *testing.T) {
	t.Parallel()
	t.Run("custom_path_and_default_tags", func(t *testing.T) {
		t.Parallel()
		p, err := sd.New(&CustomTagStruct{}, sd.Options{
			Delim:      ".",
			PathTag:    "mypath",
			DefaultTag: "mydefault",
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		m := mustRead(t, p)
		assertPath(t, m, "host", "myhost")
		assertPath(t, m, "port", int(9090))
	})
	t.Run("empty_string_falls_back_to_defaults", func(t *testing.T) {
		t.Parallel()
		// Empty string for either tag should fall back to the library defaults.
		p, err := sd.New(&ScalarDefaults{}, sd.Options{Delim: ".", PathTag: "", DefaultTag: ""})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		m := mustRead(t, p)
		if !hasPath(m, "s") {
			t.Error("expected 's' key with default koanf tag fallback")
		}
	})
}

func TestInvalidInput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input any
	}{
		{"nil", nil},
		{"int", 42},
		{"string", "hello"},
		{"nil_ptr", (*ScalarDefaults)(nil)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Invalid target now surfaces at New, not Read.
			_, err := sd.New(tc.input, sd.Options{Delim: "."})
			if err == nil {
				t.Fatal("expected error from New, got nil")
			}
			if !errors.Is(err, sd.ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got: %v", err)
			}
		})
	}
}

// TestStrictModeFrozen verifies that Strict treats the configuration as
// frozen at construction time: env-var (or any Lookup-source) changes after
// New are not picked up by subsequent Read calls. This is the observable
// consequence of caching the eager-walk result.
func TestStrictModeFrozen(t *testing.T) {
	t.Parallel()
	type cfg struct {
		Greeting string `koanf:"greeting" koanf-default:"${TONE}"`
	}
	tone := "polite"
	lookup := func(name string) (string, bool) {
		if name == "TONE" {
			return tone, true
		}
		return "", false
	}
	p, err := sd.New(&cfg{}, sd.Options{Delim: ".", Strict: true, Lookup: lookup})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Mutate the underlying source. Strict should ignore the change.
	tone = "rude"
	m, err := p.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	got, ok := getPath(m, "greeting")
	if !ok {
		t.Fatal("greeting missing from cached map")
	}
	if got != "polite" {
		t.Errorf("Strict cache should return %q, got %q", "polite", got)
	}
}

// TestStrictModeSurfacesAllErrors verifies that a Strict construction with
// multiple bad fields returns one errors.Join containing every problem,
// rather than fail-fast on the first. This is the observable benefit of
// accumulate-mode walking.
func TestStrictModeSurfacesAllErrors(t *testing.T) {
	t.Parallel()
	type cfg struct {
		Port    int    `koanf:"port"    koanf-default:"not-an-int"`
		Timeout int    `koanf:"timeout" koanf-default:"also-bad"`
		Region  string `koanf:"region"  koanf-default:"${UNSET_VAR}"`
	}
	emptyLookup := func(string) (string, bool) { return "", false }
	_, err := sd.New(&cfg{}, sd.Options{Delim: ".", Strict: true, Lookup: emptyLookup})
	if err == nil {
		t.Fatal("expected joined errors, got nil")
	}
	if !errors.Is(err, sd.ErrInvalidValue) {
		t.Errorf("want ErrInvalidValue reachable via errors.Is, got %v", err)
	}
	if !errors.Is(err, sd.ErrUnsetEnv) {
		t.Errorf("want ErrUnsetEnv reachable via errors.Is, got %v", err)
	}
	// Three errors joined by errors.Join produce two newline separators.
	if strings.Count(err.Error(), "\n") < 2 {
		t.Errorf("expected at least 3 errors joined, got: %s", err.Error())
	}
}

func TestReadBytes(t *testing.T) {
	t.Parallel()
	_, err := mustNew(t, &EmptyStruct{}).ReadBytes()
	if !errors.Is(err, sd.ErrUnsupported) {
		t.Errorf("expected ErrUnsupported, got: %v", err)
	}
}

func TestKoanfIntegration(t *testing.T) {
	t.Parallel()
	type ServerCfg struct {
		Host    string        `koanf:"host"    koanf-default:"localhost"`
		Port    int           `koanf:"port"    koanf-default:"8080"`
		Timeout time.Duration `koanf:"timeout" koanf-default:"30s"`
	}
	type AppCfg struct {
		Server   ServerCfg `koanf:"server"`
		LogLevel string    `koanf:"log_level" koanf-default:"info"`
	}

	k := koanf.New(".")

	// Layer 1: struct defaults (lowest priority).
	if err := k.Load(mustNew(t, &AppCfg{}), nil); err != nil {
		t.Fatalf("load defaults: %v", err)
	}

	// Layer 2: override via confmap (higher priority).
	override := map[string]any{
		"server.port": 9999,
		"log_level":   "debug",
	}
	if err := k.Load(confmap.Provider(override, "."), nil); err != nil {
		t.Fatalf("load override: %v", err)
	}

	// Defaults should hold where not overridden.
	if got := k.String("server.host"); got != "localhost" {
		t.Errorf("server.host: got %q, want 'localhost'", got)
	}
	if got := k.Duration("server.timeout"); got != 30*time.Second {
		t.Errorf("server.timeout: got %v, want 30s", got)
	}
	// Override should win.
	if got := k.Int("server.port"); got != 9999 {
		t.Errorf("server.port: got %d, want 9999", got)
	}
	if got := k.String("log_level"); got != "debug" {
		t.Errorf("log_level: got %q, want 'debug'", got)
	}
}

// ---- utility ----------------------------------------------------------------

// ---- cycle guard -----------------------------------------------------------

type cyclicNode struct {
	Name string      `koanf:"name" koanf-default:"x"`
	Next *cyclicNode `koanf:"next"`
}

type cyclicTree struct {
	Name  string      `koanf:"name" koanf-default:"root"`
	Left  *cyclicTree `koanf:"left"`
	Right *cyclicTree `koanf:"right"`
}

// Three distinct types chained linearly — deep but acyclic.
type chainL1 struct {
	V    string  `koanf:"v" koanf-default:"l1"`
	Down chainL2 `koanf:"down"`
}
type chainL2 struct {
	V    string   `koanf:"v" koanf-default:"l2"`
	Down *chainL3 `koanf:"down"`
}
type chainL3 struct {
	V string `koanf:"v" koanf-default:"l3"`
}

func TestCyclicType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input any
	}{
		{"linked_list_self_ref", &cyclicNode{}},
		{"tree_self_ref", &cyclicTree{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := mustNew(t, tc.input).Read()
			if !errors.Is(err, sd.ErrCyclicType) {
				t.Fatalf("expected ErrCyclicType, got: %v", err)
			}
			if !strings.Contains(err.Error(), "config path") {
				t.Errorf("error missing config path context: %v", err)
			}
		})
	}
}

func TestNonCyclicDeepNesting(t *testing.T) {
	t.Parallel()
	m, err := mustNew(t, &chainL1{}).Read()
	if err != nil {
		t.Fatalf("acyclic deep nesting must not trip cycle guard: %v", err)
	}
	assertPath(t, m, "v", "l1")
	assertPath(t, m, "down.v", "l2")
	assertPath(t, m, "down.down.v", "l3")
}

// ---- Options validation ----------------------------------------------------

func TestEmptyDelimErrors(t *testing.T) {
	t.Parallel()
	type cfg struct {
		S string `koanf:"s" koanf-default:"x"`
	}
	_, err := sd.New(&cfg{}, sd.Options{Delim: ""})
	if !errors.Is(err, sd.ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for empty delim, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Delim") {
		t.Errorf("error should mention Delim: %v", err)
	}
}

func TestStrictModeEager(t *testing.T) {
	t.Parallel()
	// Strict=true: parse failures surface from New, not Read.
	type cfg struct {
		N int `koanf:"n" koanf-default:"not-a-number"`
	}
	_, err := sd.New(&cfg{}, sd.Options{Delim: ".", Strict: true})
	if !errors.Is(err, sd.ErrInvalidValue) {
		t.Fatalf("Strict mode must surface ErrInvalidValue from New: got %v", err)
	}
}

func TestNonStrictDefersErrors(t *testing.T) {
	t.Parallel()
	// Strict=false (default): bad defaults still construct; error surfaces at Read.
	type cfg struct {
		N int `koanf:"n" koanf-default:"not-a-number"`
	}
	p, err := sd.New(&cfg{}, sd.Options{Delim: "."})
	if err != nil {
		t.Fatalf("non-strict New must not error on bad defaults: %v", err)
	}
	_, err = p.Read()
	if !errors.Is(err, sd.ErrInvalidValue) {
		t.Errorf("expected ErrInvalidValue from Read, got: %v", err)
	}
}

// ---- diagnostic helpers ----------------------------------------------------

// flatKeys returns all leaf keys in a nested map as dot-joined strings,
// useful for diagnostic messages.
func flatKeys(m map[string]any, prefix string) []string {
	var out []string
	for k, v := range m {
		full := k
		if prefix != "" {
			full = prefix + "." + k
		}
		if nested, ok := v.(map[string]any); ok {
			out = append(out, flatKeys(nested, full)...)
		} else {
			out = append(out, full)
		}
	}
	return out
}
