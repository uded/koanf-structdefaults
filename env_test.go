package structdefaults

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// ---- unit: substituteEnv ----------------------------------------------------

func TestSubstituteEnvUnit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		input      string
		env        map[string]string
		want       string
		wantErrIs  error
		wantErrMsg string
	}{
		{
			name:  "no_substitution",
			input: "plain string",
			want:  "plain string",
		},
		{
			name:  "single_var_set",
			input: "${HOST}",
			env:   map[string]string{"HOST": "example.com"},
			want:  "example.com",
		},
		{
			name:  "fallback_used_when_unset",
			input: "${HOST:-localhost}",
			want:  "localhost",
		},
		{
			name:  "fallback_ignored_when_set",
			input: "${HOST:-localhost}",
			env:   map[string]string{"HOST": "example.com"},
			want:  "example.com",
		},
		{
			name:  "explicit_empty_fallback",
			input: "${MISSING:-}",
			want:  "",
		},
		{
			name:  "multiple_vars_in_one_string",
			input: "${HOST}:${PORT}",
			env:   map[string]string{"HOST": "h", "PORT": "9000"},
			want:  "h:9000",
		},
		{
			name:  "var_set_to_empty_string_no_fallback",
			input: "${EMPTY}",
			env:   map[string]string{"EMPTY": ""},
			want:  "",
		},
		{
			name:  "unmatched_open_kept_literal",
			input: "${UNFINISHED",
			want:  "${UNFINISHED",
		},
		{
			name:  "invalid_var_name_kept_literal",
			input: "${1ABC}",
			want:  "${1ABC}",
		},
		{
			name:  "empty_braces_kept_literal",
			input: "${}",
			want:  "${}",
		},
		{
			name:  "fallback_can_contain_dashes_and_dots",
			input: "${X:-1.2.3-rc.4}",
			want:  "1.2.3-rc.4",
		},
		{
			name:       "unset_no_fallback_errors",
			input:      "${MISSING}",
			wantErrIs:  ErrUnsetEnv,
			wantErrMsg: "MISSING",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lookup := func(name string) (string, bool) {
				v, ok := tc.env[name]
				return v, ok
			}
			got, err := substituteEnv(tc.input, lookup)
			if tc.wantErrIs != nil {
				if !errors.Is(err, tc.wantErrIs) {
					t.Fatalf("err = %v, want errors.Is(%v)", err, tc.wantErrIs)
				}
				if tc.wantErrMsg != "" && err != nil && !strings.Contains(err.Error(), tc.wantErrMsg) {
					t.Errorf("err = %q, want substring %q", err.Error(), tc.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ---- integration: env substitution applied through Provider.Read() ----------

type envCfg struct {
	Server struct {
		Host    string        `koanf:"host"    koanf-default:"${HOST:-localhost}"`
		Port    int           `koanf:"port"    koanf-default:"${PORT:-8080}"`
		Timeout time.Duration `koanf:"timeout" koanf-default:"${TIMEOUT:-30s}"`
	} `koanf:"server"`
	Region string `koanf:"region" koanf-default:"${REGION}"`
}

func TestEnvIntegration_FallbacksUsed(t *testing.T) {
	t.Parallel()

	p := Provider(&envCfg{}, ".").WithLookup(func(string) (string, bool) {
		return "", false
	})

	// REGION has no fallback and is unset → expect ErrUnsetEnv.
	_, err := p.Read()
	if !errors.Is(err, ErrUnsetEnv) {
		t.Fatalf("expected ErrUnsetEnv, got: %v", err)
	}
}

func TestEnvIntegration_AllResolved(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"HOST":    "prod.example.com",
		"PORT":    "9000",
		"TIMEOUT": "1m",
		"REGION":  "eu-west-1",
	}
	p := Provider(&envCfg{}, ".").WithLookup(func(name string) (string, bool) {
		v, ok := env[name]
		return v, ok
	})

	m, err := p.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	server := m["server"].(map[string]any)
	if got := server["host"]; got != "prod.example.com" {
		t.Errorf("server.host = %v, want prod.example.com", got)
	}
	if got := server["port"]; got != 9000 {
		t.Errorf("server.port = %v, want 9000", got)
	}
	if got := server["timeout"]; got != time.Minute {
		t.Errorf("server.timeout = %v, want 1m", got)
	}
	if got := m["region"]; got != "eu-west-1" {
		t.Errorf("region = %v, want eu-west-1", got)
	}
}

func TestEnvIntegration_FallbacksWhenUnset(t *testing.T) {
	t.Parallel()

	// Only REGION is set; the others fall back.
	p := Provider(&envCfg{}, ".").WithLookup(func(name string) (string, bool) {
		if name == "REGION" {
			return "eu-west-1", true
		}
		return "", false
	})

	m, err := p.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	server := m["server"].(map[string]any)
	if got := server["host"]; got != "localhost" {
		t.Errorf("server.host = %v, want localhost", got)
	}
	if got := server["port"]; got != 8080 {
		t.Errorf("server.port = %v, want 8080", got)
	}
	if got := server["timeout"]; got != 30*time.Second {
		t.Errorf("server.timeout = %v, want 30s", got)
	}
}

func TestEnvIntegration_DefaultLookupIsOSEnv(t *testing.T) {
	// This test mutates the process env; cannot be t.Parallel().
	t.Setenv("STRUCTDEFAULTS_TEST_HOST", "from-os")

	type cfg struct {
		Host string `koanf:"host" koanf-default:"${STRUCTDEFAULTS_TEST_HOST}"`
	}
	p := Provider(&cfg{}, ".")
	m, err := p.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got := m["host"]; got != "from-os" {
		t.Errorf("host = %v, want from-os", got)
	}
}

func TestEnvIntegration_ErrorWrappingIncludesPath(t *testing.T) {
	t.Parallel()

	type cfg struct {
		Server struct {
			Host string `koanf:"host" koanf-default:"${UNSET_VAR}"`
		} `koanf:"server"`
	}

	p := Provider(&cfg{}, ".").WithLookup(func(string) (string, bool) {
		return "", false
	})
	_, err := p.Read()
	if !errors.Is(err, ErrUnsetEnv) {
		t.Fatalf("want ErrUnsetEnv, got: %v", err)
	}
	msg := err.Error()
	for _, want := range []string{"UNSET_VAR", "server.host", "Server.Host"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing substring %q", msg, want)
		}
	}
}
