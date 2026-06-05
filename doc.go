// Package structdefaults is a koanf v2 provider that emits configuration
// defaults declared inline on a Go struct via `koanf-default:"…"` tags.
//
// Each field's default is parsed into its destination type and assembled
// into a nested map[string]any whose tree shape mirrors the koanf path
// layout, so the provider loads cleanly as the lowest-priority layer
// beneath file, env, and flag providers.
//
// Tag values support POSIX-style ${VAR} and ${VAR:-fallback} environment
// substitution, evaluated before type parsing, and an opt-in Strict mode
// validates the entire struct eagerly at construction.
//
// See https://github.com/uded/koanf-structdefaults for the full README,
// recipes, and the sibling koanf-validate and koanf-etcd providers.
package structdefaults
