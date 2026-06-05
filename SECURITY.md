# Security policy

## Reporting a vulnerability

If you believe you have found a security issue in `koanf-structdefaults`,
please report it through GitHub's
[private vulnerability advisory](https://github.com/uded/koanf-structdefaults/security/advisories/new)
flow rather than opening a public issue. The maintainer will acknowledge
receipt within five working days and aim to confirm the issue (or
explain why it isn't one) within ten working days.

If GitHub's private advisory flow is not available to you, email the
maintainer at the address listed in the repo's [LICENSE](./LICENSE) /
`git log` author lines with a subject line beginning `[koanf-structdefaults
security]`. PGP is not required.

Please include:

- A description of the vulnerability and its impact.
- A minimal reproducer (Go code, struct definitions, the call that
  triggers the issue, environment variables involved).
- The version of `koanf-structdefaults` and `koanf/v2` involved.
- Any mitigations you have already identified.

## What's in scope

This library reads `koanf-default:"…"` struct tags via reflection and
emits a nested `map[string]any` for `koanf.Load`. Scope includes:

- **Secret leakage through the public error surface.** Tag values may
  reference environment variables via `${VAR}` substitution; the
  library is expected to surface env-var **names** but never resolved
  **values** in `err.Error()`. Report any path that leaks a resolved
  value through `errors.Is` / `errors.As` chains or the surface error
  string.
- **Reflection panics escaping into the host process.** The walker
  must convert every panic from `reflect`, `strconv`, `time.ParseDuration`,
  a custom `EnvLookup`, or a `TextUnmarshaler` implementation into a
  normal `(map, error)` return.
- **Resource exhaustion via adversarial struct shapes.** Cyclic struct
  types must be caught by `ErrCyclicType`. Pathologically deep nesting
  or large tag-value strings must not cause unbounded memory growth.
- **Struct-tag injection / ReDoS in `substituteEnv`.** The env-var
  parser is hand-rolled, single-pass, and non-recursive specifically
  to deny these classes of attack; report any input that triggers
  catastrophic backtracking, infinite loop, or OOM.
- **Concurrent-safety violations under documented public-API usage.**
  `Read` must be safe for concurrent callers; `Options.Lookup`
  implementations may be invoked from any goroutine.

The library does not handle authentication, authorization, network
traffic, persistent storage, or downstream config consumption;
vulnerabilities in those layers belong to your application, to
`koanf` itself, or to upstream dependencies.

## Supported versions

Security fixes target the latest minor release line. Pre-1.0 the
project moves quickly — please upgrade to the latest minor before
reporting an issue unless the report concerns the latest release.

| Version | Status |
|---|---|
| `0.x` (current) | Active security support |
| `< 0.x` | Not supported — upgrade to current |

## Coordinated disclosure

Once a fix is ready, it ships in a patch release. The advisory is
published the same day on GitHub Security Advisories and referenced
in the release notes. Credit is given to the reporter unless
anonymity is explicitly requested.
