# Phase 11 — Version stamping, release tooling, and install path

*Realizes design Decision 10 (versioning/release/install), with the D08/D09
version-output and `Deps.Version` rewrites those decisions now state.*

What exists at the end:

- `cmd/autotune/main.go` declares `var version = "dev"` and passes it as
  `Deps.Version`; `internal/app` prints that string bare for
  `-V`/`--version` and no longer holds a version constant of its own. The
  existing binary-envelope test (R-RYBX-FR5U) asserts the bare-version
  output per D09's current statement.
- The Makefile is D10's: `bin/autotune` built with the
  `git describe --tags --always --dirty` stamp, plus `fmt`, `test`,
  `install` (to `$(PREFIX)/bin`, `PREFIX ?= $(HOME)/.local`), and `clean`
  targets; `.gitignore` covers `bin/`.
- `.goreleaser.yaml` and `.github/workflows/release.yml` exist exactly as
  D10 specifies (embed's, names substituted): tag push `v*` → goreleaser v2
  → linux/darwin × amd64/arm64 tar.gz with versionless asset names,
  `checksums.txt`, GitHub changelog, `{{ .Tag }}` stamped.
- `install.sh` exists at the repo root per D10 (`REPO="ikigenba/autotune"`,
  `BINARY="autotune"`, pin var `AUTOTUNE_VERSION`).
- The top-level `README.md` has D10's `## Install` section (curl one-liner
  with override/pin notes, and `make install`), replacing the current bare
  build instructions.

Tagging `v0.1.0` and pushing it is the operator's act after this phase —
not part of the phase.

**Done when:**

- R-9A0E-BG7N and R-9B8A-P7YC are covered by tagged tests (unstamped build
  prints exactly `dev`; sentinel-stamped build prints exactly the
  sentinel) and `go test ./...` exits 0.
- `make clean build` exits 0 and produces `bin/autotune`, and
  `./bin/autotune -V` prints a non-empty string that is not `dev` when at
  least one tag exists, else the `git describe --always` fallback — i.e.
  the stamped binary's `-V` output equals
  `git describe --tags --always --dirty` exactly.
- `grep -c 'AUTOTUNE_VERSION' install.sh` ≥ 1;
  `grep -c 'name_template: "{{ .Binary }}_{{ .Os }}_{{ .Arch }}"' .goreleaser.yaml`
  = 1; `grep -c "goreleaser/goreleaser-action@v6" .github/workflows/release.yml`
  = 1; `grep -c '^bin/' .gitignore` ≥ 1;
  `grep -c 'install.sh | sh' README.md` ≥ 1.
- `grep -rn 'Version' internal/app/*.go` shows no hardcoded version string
  literal (the app package receives `Deps.Version`; it declares none).
