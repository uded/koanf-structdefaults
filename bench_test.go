package structdefaults_test

import (
	"testing"
	"time"

	sd "github.com/uded/koanf-structdefaults"
)

// ---- benchmark structs -------------------------------------------------------

// benchShallow is a single-level struct with 8 leaf fields covering the most
// common primitive types: string, int, bool, float64, and time.Duration.
type benchShallow struct {
	Host     string        `koanf:"host"     koanf-default:"localhost"`
	Port     int           `koanf:"port"     koanf-default:"8080"`
	Debug    bool          `koanf:"debug"    koanf-default:"false"`
	Workers  int           `koanf:"workers"  koanf-default:"4"`
	MaxConns int           `koanf:"maxconns" koanf-default:"100"`
	Rate     float64       `koanf:"rate"     koanf-default:"1.5"`
	Timeout  time.Duration `koanf:"timeout"  koanf-default:"30s"`
	Label    string        `koanf:"label"    koanf-default:"default"`
}

// benchMiddle / benchDeep mirror the DeepNested shape from the main test file,
// extended to three levels of indirection.
type benchDeepL3 struct {
	Score  int    `koanf:"score"  koanf-default:"99"`
	Remark string `koanf:"remark" koanf-default:"ok"`
}

type benchDeepL2 struct {
	Name  string     `koanf:"name"  koanf-default:"middle"`
	Count int        `koanf:"count" koanf-default:"7"`
	Inner benchDeepL3 `koanf:"inner"`
}

type benchDeepL1 struct {
	Title string       `koanf:"title" koanf-default:"top"`
	Level benchDeepL2  `koanf:"level"`
}

// benchEnvSubst has several fields using ${VAR:-fallback} syntax.  The
// deterministic lookup function below resolves two of them so the walker
// exercises both the substituted and fallback code paths without touching
// os.Setenv (which is not safe under -race + t.Parallel).
type benchEnvSubst struct {
	DSN      string        `koanf:"dsn"      koanf-default:"${DB_DSN:-postgres://localhost/app}"`
	LogLevel string        `koanf:"loglevel" koanf-default:"${LOG_LEVEL:-info}"`
	Port     int           `koanf:"port"     koanf-default:"${PORT:-9090}"`
	Timeout  time.Duration `koanf:"timeout"  koanf-default:"${TIMEOUT:-15s}"`
	Workers  int           `koanf:"workers"  koanf-default:"${WORKERS:-2}"`
}

// benchLookup is a deterministic env lookup used by BenchmarkEnvSubstitution.
// It resolves DB_DSN and LOG_LEVEL so that two fields take the substituted path
// and three fall back to the inline default.
func benchLookup(name string) (string, bool) {
	switch name {
	case "DB_DSN":
		return "postgres://bench-host/benchdb", true
	case "LOG_LEVEL":
		return "warn", true
	default:
		return "", false
	}
}

// ---- benchmarks -------------------------------------------------------------

// BenchmarkShallow measures Read() on a flat struct with 8 leaf fields.
// sd.New is called once outside the loop because construction is cheap and
// the hot path we want to measure is the walker inside Read().
func BenchmarkShallow(b *testing.B) {
	p, err := sd.New(&benchShallow{}, sd.Options{Delim: "."})
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := p.Read(); err != nil {
			b.Fatalf("Read: %v", err)
		}
	}
}

// BenchmarkNested measures Read() on a 3-deep nested struct (6 leaf fields
// across 3 levels of nesting) to capture the overhead of recursive descent and
// nested map construction.
func BenchmarkNested(b *testing.B) {
	p, err := sd.New(&benchDeepL1{}, sd.Options{Delim: "."})
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := p.Read(); err != nil {
			b.Fatalf("Read: %v", err)
		}
	}
}

// BenchmarkEnvSubstitution measures Read() when every field value passes
// through the ${VAR:-fallback} substitution logic.  A deterministic lookup
// function is injected so the benchmark is race-free and reproducible across
// runs without manipulating the OS environment.
func BenchmarkEnvSubstitution(b *testing.B) {
	p, err := sd.New(&benchEnvSubst{}, sd.Options{
		Delim:  ".",
		Lookup: benchLookup,
	})
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := p.Read(); err != nil {
			b.Fatalf("Read: %v", err)
		}
	}
}
