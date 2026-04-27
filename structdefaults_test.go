package structdefaults_test

import (
	"errors"
	"net"
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

// getPath looks up a delim-joined key in the flat map produced by Read().
func getPath(m map[string]any, path string) (any, bool) {
	v, ok := m[path]
	return v, ok
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
	m := mustRead(t, sd.Provider(&ScalarDefaults{}, "."))

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
		m := mustRead(t, sd.Provider(&DurationStruct{}, "."))
		assertPath(t, m, "timeout", 30*time.Second)
	})
	t.Run("parse_error", func(t *testing.T) {
		t.Parallel()
		_, err := sd.Provider(&BadDuration{}, ".").Read()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, sd.ErrUnsupportedType) {
			t.Errorf("expected ErrUnsupportedType, got: %v", err)
		}
	})
}

func TestTextUnmarshaler(t *testing.T) {
	t.Parallel()
	t.Run("net.IP_value_receiver_via_pointer", func(t *testing.T) {
		t.Parallel()
		m := mustRead(t, sd.Provider(&IPStruct{}, "."))
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
		m := mustRead(t, sd.Provider(&ColorStruct{}, "."))
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
	m := mustRead(t, sd.Provider(&DeepNested{}, "."))

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
		m := mustRead(t, sd.Provider(input, "."))
		if input.Inner != nil {
			t.Error("Read() mutated input: Inner should remain nil")
		}
		assertPath(t, m, "inner.x", int(99))
	})
	t.Run("non_nil_pointer_field", func(t *testing.T) {
		t.Parallel()
		input := &WithPtr{Inner: &PtrInner{X: 7}}
		m := mustRead(t, sd.Provider(input, "."))
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
		m := mustRead(t, sd.Provider(&SquashParent{}, "."))
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
		m := mustRead(t, sd.Provider(&NamedEmbedParent{}, "."))
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
	m := mustRead(t, sd.Provider(&SkipField{}, "."))
	// Neither the Go field name nor any synthetic key should appear.
	if hasPath(m, "skip") || hasPath(m, "Skip") {
		t.Error("skipped field should not appear in output")
	}
	assertPath(t, m, "present", "here")
}

func TestMissingKoanfTag(t *testing.T) {
	t.Parallel()
	m := mustRead(t, sd.Provider(&NoKoanfTag{}, "."))
	got, ok := getPath(m, "GoName")
	if !ok || got != "fallback" {
		t.Errorf("expected m['GoName']='fallback', got %v (keys: %v)", got, flatKeys(m, ""))
	}
}

func TestEmptyStringDefault(t *testing.T) {
	t.Parallel()
	m := mustRead(t, sd.Provider(&EmptyStringDefault{}, "."))
	got, ok := getPath(m, "s")
	if !ok {
		t.Fatal("key 's' missing — empty-string default must be emitted")
	}
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   any
		wantErr error
	}{
		{"bad_int", &BadInt{}, sd.ErrUnsupportedType},
		{"bad_duration", &BadDuration{}, sd.ErrUnsupportedType},
		{"unsupported_slice", &UnsupportedSlice{}, sd.ErrUnsupportedType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := sd.Provider(tc.input, ".").Read()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("expected %v, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestEmptyStruct(t *testing.T) {
	t.Parallel()
	m := mustRead(t, sd.Provider(&EmptyStruct{}, "."))
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

func TestCustomTags(t *testing.T) {
	t.Parallel()
	t.Run("custom_path_and_default_tags", func(t *testing.T) {
		t.Parallel()
		p := sd.ProviderWithTags(&CustomTagStruct{}, "mypath", "mydefault", ".")
		m := mustRead(t, p)
		assertPath(t, m, "host", "myhost")
		assertPath(t, m, "port", int(9090))
	})
	t.Run("empty_string_falls_back_to_defaults", func(t *testing.T) {
		t.Parallel()
		// Empty string for either tag should fall back to the library defaults.
		p := sd.ProviderWithTags(&ScalarDefaults{}, "", "", ".")
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
			_, err := sd.Provider(tc.input, ".").Read()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, sd.ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got: %v", err)
			}
		})
	}
}

func TestReadBytes(t *testing.T) {
	t.Parallel()
	_, err := sd.Provider(&EmptyStruct{}, ".").ReadBytes()
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
	if err := k.Load(sd.Provider(&AppCfg{}, "."), nil); err != nil {
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

// flatKeys returns the keys of the flat map, useful for diagnostic messages.
func flatKeys(m map[string]any, _ string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
