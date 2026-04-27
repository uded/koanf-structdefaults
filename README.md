# koanf-structdefaults

A [koanf](https://github.com/knadh/koanf) provider that reads `koanf-default:"…"` struct tags and emits a nested `map[string]any` of parsed defaults — load it as the lowest-priority layer and let file, env, and flag providers override naturally.

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

Requires Go 1.25+.

## Usage

```go
package main

import (
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
        Port    int           `koanf:"port"    koanf-default:"8080"`
        Timeout time.Duration `koanf:"timeout" koanf-default:"30s"`
    } `koanf:"server"`
    LogLevel string `koanf:"log_level" koanf-default:"info"`
}

func main() {
    k := koanf.New(".")

    // Layer 1 (lowest priority): declared defaults.
    k.Load(structdefaults.Provider(&Config{}, "."), nil)
    // Layer 2: file overrides defaults.
    k.Load(file.Provider("config.yaml"), yaml.Parser())
    // Layer 3 (highest): env overrides everything.
    k.Load(env.Provider("APP_", ".", nil), nil)

    var cfg Config
    if err := k.Unmarshal("", &cfg); err != nil {
        panic(err)
    }
}
```

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

## Custom tag names

```go
// Read paths from `mapstructure:"…"` and defaults from `default:"…"`:
p := structdefaults.ProviderWithTags(&Config{}, "mapstructure", "default", ".")
```

Empty-string for either tag falls back to the library default.

## Output shape

`Read()` returns a nested `map[string]any` whose tree shape mirrors the koanf path layout (split on the configured delim):

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

This matches what every other koanf provider emits and merges correctly with overrides from file, env, and flag layers. The module still has **zero production dependencies** — the nesting is built directly during the walk, no helper library needed.

## What it doesn't do

- **Slice / map defaults.** Comma-split syntax has too many escaping pitfalls; for the rare slice/map default, build it via `confmap` or initialize the field directly. Future versions may add a structured syntax (e.g. `koanf-default-json:"[1,2,3]"`).
- **Validation (`required:"…"`).** Different concern; out of scope.
- **Mutating your input struct.** Read-only by design.

## License

MIT — see [LICENSE](./LICENSE).
