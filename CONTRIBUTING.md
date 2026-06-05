# Contributing to koanf-structdefaults

Thanks for considering a contribution. This document covers the practical
mechanics; if anything here gets in your way, open an issue and we'll fix
the document, not the obstacle.

## Filing issues

- **Bug reports**: include a minimal Go reproducer — a struct definition,
  the `New`/`Read` call sequence, the observed error vs. the expected
  output. Race-detector traces are especially welcome.
- **Feature requests**: describe the use case before the API. The library's
  guiding principle is to read `koanf-default:"…"` struct tags and emit a
  nested map suitable for `koanf.Load` — proposals that fit that scope ship
  faster. Out of scope: validation (see [koanf-validate](https://github.com/uded/koanf-validate)),
  data sources (see [koanf-etcd](https://github.com/uded/koanf-etcd) and
  the bundled koanf providers).
- **Security**: see [SECURITY.md](./SECURITY.md). Do **not** open public
  issues for vulnerability reports.

## Development setup

Requires Go 1.23+ (matches `koanf/v2`'s MSRV) and
[Task](https://taskfile.dev) for the build harness.

```bash
git clone https://github.com/uded/koanf-structdefaults.git
cd koanf-structdefaults
task                  # default → task test
```

Useful task targets:

| Task | What it runs |
|---|---|
| `task fmt` | `go fmt ./...` |
| `task vet` | `go vet ./...` |
| `task test` | `go test -race ./...` |
| `task cover` | race tests + HTML coverage report at `coverage.html` |
| `task bench` | `go test -bench=. -benchmem ./...` |
| `task lint` | `vet` + `gofmt` check + `golangci-lint run` |
| `task vuln` | `govulncheck ./...` |
| `task tidy` | `go mod tidy` |
| `task ci` | full pipeline: `lint` → `test` → `vuln` |

`task ci` must be green before opening a pull request. CI runs the same
pipeline on Go 1.23, 1.24, and 1.25.

## Pull-request flow

1. Open an issue first for non-trivial changes so we can agree on the
   design before code lands.
2. Branch from `main`, keep commits focused — one logical change per commit.
3. PR titles follow [Conventional Commits](https://www.conventionalcommits.org/)
   (`feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `perf:`, `chore:`,
   `ci:`, `build:`, `revert:`). The PR-title workflow enforces this; the
   release-drafter consumes it to categorize the auto-drafted changelog.
4. Subject (after `type:`) starts lowercase and does not end with a period.
5. Tests are required for new behavior. Coverage targets the existing
   ~96% line floor — don't ship code that drops it without justification.
6. The walker is reflection-heavy; new field-type support belongs in
   `parse.go` with a dedicated test in `structdefaults_test.go`'s
   table-driven layout.
7. Public API additions (new `Options` fields, new sentinel errors)
   need godoc and a README mention. The `v1.x` API is **strictly
   additive** — renames, removals, and signature changes are reserved
   for a future `v2` module-path bump. Design new surface assuming
   you'll live with it for years.

## Commit and code style

- `gofmt` and `go vet` clean (CI enforces both).
- `golangci-lint` clean against the version pinned in
  `.github/workflows/ci.yml`.
- Cyclomatic complexity ≤ 15 per function (`gocyclo -over 15 .` is part
  of the local checklist; extract a helper rather than bumping the bar).
- No `Co-Authored-By` lines in commit messages.
- No references to internal process IDs (review tickets, finding IDs)
  in commit messages or code comments — describe the change on its own
  terms so future readers don't need access to the process artifact.

## Release process

The maintainer cuts releases manually. release-drafter keeps a draft
GitHub Release up to date with categorized PR titles. Tagging is done
locally with a signed annotated tag (`git tag -as vX.Y.Z`), pushed with
`git push origin vX.Y.Z`, and the draft is promoted to a published
release with `gh release edit … --draft=false`.

Module bytes are byte-identical between docs-only / CI-only releases
and their predecessor; the version bump exists to give downstream
pinning users a clean signal.
