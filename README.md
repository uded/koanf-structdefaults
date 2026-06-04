# koanf-structdefaults

A [koanf](https://github.com/knadh/koanf) provider that reads `koanf-default:"…"` struct tags and emits a nested `map[string]any` of parsed defaults. Load it as the lowest-priority layer and let file, env, and flag providers override naturally.

Zero production dependencies. Distributed as a standalone Go module.

[![CI](https://github.com/uded/koanf-structdefaults/actions/workflows/ci.yml/badge.svg)](https://github.com/uded/koanf-structdefaults/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/uded/koanf-structdefaults.svg)](https://pkg.go.dev/github.com/uded/koanf-structdefaults)
[![Go Report Card](https://goreportcard.com/badge/github.com/uded/koanf-structdefaults)](https://goreportcard.com/report/github.com/uded/koanf-structdefaults)
[![Release](https://img.shields.io/github/v/release/uded/koanf-structdefaults?sort=semver)](https://github.com/uded/koanf-structdefaults/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE)

## Related projects

This provider composes with two sibling packages in the same family:

- **[koanf-validate](https://github.com/uded/koanf-validate)** — validate the assembled koanf config against struct-tag rules. The natural *post-load gate* after this provider's defaults have been merged with file/env/flag layers.
- **[koanf-etcd](https://github.com/uded/koanf-etcd)** — production-grade koanf v2 Provider for etcd v3 (auth/TLS, nested output, watch with reconnect/resume/resync, debounce, BYO `*clientv3.Client`). A natural *runtime override* layer above this provider in the load order.

Recommended load order: `koanf-structdefaults` → file → `koanf-etcd` → env, with `koanf-validate` as the post-load gate.

## Why

The conventional way to declare defaults in a koanf-based app is a hand-built `confmap.Provider`:

```go
k.Load(confmap.Provider(map[string]any{
    "server.port":    8080,
    "server.host":    "localhost",
    "server.timeout": "30s",
}, "."), nil)
```

That works, but the defaults drift away from the struct, lose type-safety (`any` everywhere), and duplicate every field name. This provider lets defaults live next to the field they describe:

```go
type Config struct {
    Server struct {
        Host    string        `koanf:"host"    koanf-default:"localhost"`
        Port    int           `koanf:"port"    koanf-default:"8080"`
        Timeout time.Duration `koanf:"timeout" koanf-default:"30s"`
    } `koanf:"server"`
    LogLevel string `koanf:"log_level" koanf-default:"info"`
}
```

## Install

```bash
go get github.com/uded/koanf-structdefaults
```

Requires Go 1.23+.

## Usage

```go
package main

import (
    "log"
    "time"

    "github.com/knadh/koanf/parsers/yaml"
    "github.com/knadh/koanf/providers/env"
    "github.com/knadh/koanf/providers/file"
    "github.com/knadh/koanf/v2"

    "github.com/uded/koanf-structdefaults"
)

type Config struct {
    Server struct {
        Host    string        `koanf:"host"    koanf-default:"localhost"`
        Port    int           `koanf:"port"    koanf-default:"${PORT:-8080}"`
        Timeout time.Duration `koanf:"timeout" koanf-default:"30s"`
    } `koanf:"server"`
    LogLevel string `koanf:"log_level" koanf-default:"info"`
}

func main() {
    k := koanf.New(".")

    // Layer 1 (lowest priority): declared defaults.
    p, err := structdefaults.New(&Config{}, structdefaults.Options{Delim: "."})
    if err != nil {
        log.Fatal(err)
    }
    if err := k.Load(p, nil); err != nil {
        log.Fatal(err)
    }
    // Layer 2: file overrides defaults.
    _ = k.Load(file.Provider("config.yaml"), yaml.Parser())
    // Layer 3 (highest): env overrides everything.
    _ = k.Load(env.Provider("APP_", ".", nil), nil)

    var cfg Config
    if err := k.Unmarshal("", &cfg); err != nil {
        log.Fatal(err)
    }
}
```

## Options

```go
type Options struct {
    Delim      string    // required ("." for typical configs)
    PathTag    string    // default: "koanf"
    DefaultTag string    // default: "koanf-default"
    Lookup     EnvLookup // default: os.LookupEnv
    Strict     bool      // default: false
}
```

| Field | Effect |
|---|---|
| `Delim` | Path separator. Empty value → `ErrInvalidConfig` from `New`. |
| `PathTag` / `DefaultTag` | Override the tag names used to name fields and read defaults. Empty → library default. |
| `Lookup` | Custom env-var resolver. Pass `nil` to use `os.LookupEnv`. Useful for hermetic tests, secret stores, or precedence layering. |
| `Strict` | When `true`, `New` walks the entire struct eagerly and surfaces any error at construction time rather than at the first `Read`. |

## Supported field types

| Type | Notes |
|---|---|
| `string`, `bool` | parsed via `strconv` |
| `int`, `int8/16/32/64` | parsed via `strconv.ParseInt` with the matching bit size |
| `uint`, `uint8/16/32/64` | parsed via `strconv.ParseUint` |
| `float32`, `float64` | parsed via `strconv.ParseFloat` |
| `time.Duration` | parsed via `time.ParseDuration` (e.g. `"30s"`, `"1h15m"`) |
| Any `encoding.TextUnmarshaler` | both value-receiver and pointer-receiver |
| Nested `struct` | walked recursively |
| Pointer-to-struct | walked via a temporary instance — your input struct is **never** mutated |
| Anonymous embedded struct | flattened (squash) unless it carries an explicit `koanf:"name"` tag |

## Tag semantics

| Tag | Behavior |
|---|---|
| `koanf:"name"` | path segment is `name` |
| `koanf:"-"` | field is skipped entirely (overrides any `koanf-default`) |
| `koanf:""` or absent | path segment is the Go field name |
| `koanf-default:"value"` | declared default for this field |
| `koanf-default:""` | empty-string default (only meaningful for `string`) |
| `koanf-default:` absent | no entry emitted (output is sparse) |

## Environment variable substitution

Tag values may include POSIX-style `${VAR}` and `${VAR:-fallback}` references. Substitution runs **before** type parsing, so it works for any field type:

```go
type Config struct {
    Host    string        `koanf:"host"    koanf-default:"${HOST:-localhost}"`
    Port    int           `koanf:"port"    koanf-default:"${PORT:-8080}"`
    Timeout time.Duration `koanf:"timeout" koanf-default:"${TIMEOUT:-30s}"`
    Region  string        `koanf:"region"  koanf-default:"${REGION}"` // no fallback
}
```

| Form | Behavior |
|---|---|
| `${VAR}` | Resolves `VAR`. If unset, returns `ErrUnsetEnv`. |
| `${VAR:-fallback}` | Uses `VAR` if set; otherwise the literal `fallback` (which may be empty: `${VAR:-}`). |
| `$$` … literal `$` | Not yet supported — request via issue if needed. |

Substitution is **single-pass and non-recursive**: env-var values are not re-scanned for `${...}`. This is intentional — prevents indirect env-var resolution.

### Custom lookup

For hermetic tests or secret stores (Vault, AWS Secrets Manager), pass a custom `Lookup`:

```go
p, err := structdefaults.New(&cfg, structdefaults.Options{
    Delim: ".",
    Lookup: func(name string) (string, bool) {
        return vaultClient.Get(name)
    },
})
```

### Security note

Struct-tag values are **compiled into your binary** and visible in `strings ./binary`. Never embed secrets directly in `koanf-default:"…"` — use `${VAR}` substitution and resolve them from your environment or secret store at runtime. Errors from this library may include env-var **names** (not values); route library errors through your standard log-redaction pipeline.

## Strict mode

By default, parse errors and unset env vars surface from `Read()` at koanf load time. Set `Strict: true` to force eager validation at construction:

```go
p, err := structdefaults.New(&cfg, structdefaults.Options{
    Delim:  ".",
    Strict: true,
})
// err is non-nil if any default fails to parse, any env var is unset
// without a fallback, or the struct contains a cyclic type.
```

`Strict` is a startup-time correctness gate: catches typos in tag values before the app starts serving traffic.

## Errors

All errors are sentinel-wrapped via `%w`; match with `errors.Is` or `errors.As`:

| Sentinel | When |
|---|---|
| `ErrInvalidConfig` | `Options.Delim` is empty (returned from `New`). |
| `ErrInvalidInput` | target is `nil`, a non-struct, or a nil pointer-to-struct (returned from `New`). |
| `ErrCyclicType` | walker encountered a struct type that recursively references itself. |
| `ErrInvalidValue` | a tag value cannot be parsed for an otherwise-supported type (e.g. `koanf-default:"8O8O"` on `int`). Use `errors.As` to recover the underlying `*strconv.NumError` etc. |
| `ErrUnsupportedType` | the field's Go type cannot carry a default at all (slice, map, channel, func). Programmer error: fix the struct. |
| `ErrUnsetEnv` | `${VAR}` reference with no fallback and the env var is unset. |
| `ErrUnsupported` | returned from `ReadBytes()` (this provider is `Read()`-only). |

## Output shape

`Read()` returns a nested `map[string]any` whose tree shape mirrors the koanf path layout:

```go
map[string]any{
    "server": map[string]any{
        "host":    "localhost",
        "port":    8080,
        "timeout": time.Duration(30 * time.Second),
    },
    "log_level": "info",
}
```

This matches what every other koanf provider emits and merges correctly with overrides from file, env, and flag layers.

## What it doesn't do

- **Slice / map defaults.** Comma-split syntax has too many escaping pitfalls; for the rare slice/map default, build it via `confmap` or initialize the field directly. A future `koanf-default-json:"[1,2,3]"` tag is on the roadmap.
- **Validation (`required:"…"`).** Different concern; out of scope.
- **Mutating your input struct.** Read-only by design.

## License

MIT — see [LICENSE](./LICENSE).
