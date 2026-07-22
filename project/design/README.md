# autotune — Design

**Authority: shape and its proof.** This design owns *how* autotune is built
and *how each behavior is proven*. Product owns the why and the promises;
design states the exact, checkable form of those promises and never
re-declares the why. Design uses the product's contractual constants by value
but does not own them. This is the single current statement of the
architecture, rewritten in place as it changes; construction history lives in
git.

## Requirement ids

Each Decision ends with a **Verification** list: the concrete behaviors that
decision requires. Every item carries a minted `R-XXXX-XXXX` id — a stable,
unique handle for that one behavior. The ids live inline in these lists and
nowhere else; there is no separate requirements document. Design's
responsibility for ids ends at minting them — how coverage is measured and
when the work is "done" are downstream's concern and not specified here.

## Conventions

- Language: Go 1.26. Module path: `github.com/ikigenba/autotune`. Binary:
  `autotune`, built from `cmd/autotune`.
- Dependency: `github.com/ikigenba/agentkit v0.7.0` (library and provider
  subpackages; see research.md for its footprint).
- Build/typecheck command: `go build ./...`. Test command: `go test ./...`.
  **"The suite is green" means `go test ./...` exits 0.** Tests must not
  require network access or credentials.
- Requirement-id tags live in `*_test.go` files: each realized id appears
  verbatim (in a test name's comment or the test body) in exactly one test.
- Formatting: `gofmt`. Errors: fail loudly; no silent fallbacks. External
  input (flags, config.json, scorer output, improver responses) is validated
  at the boundary; internal invariants are asserted, not re-validated.
- Exit-code taxonomy (shared across Decisions): `0` finished (including "no
  improvement found"), `1` internal/agent/scorer failure, `2` usage error,
  `3` budget rail crossed, `130` interrupted (SIGINT/SIGTERM).
- Injected seams for testability, following agent-repl/ralph: `cmd/autotune`
  is a thin composition root building a `Deps` struct (streams, getenv,
  clock, TTY detection, provider factory); all logic lives under
  `internal/` and runs against injected deps. The agentkit `Provider`
  interface is the model-call seam: tests drive orchestration with a
  scripted fake provider; the real provider HTTP contracts are owned and
  tested by agentkit itself.

## Layout

`INDEX.md` is the manifest; `DNN.md` is one self-contained file per Decision
(zero-padded; referenced in prose and the plan as `D<N>`); this README holds
only the spine. Design is rewritten in place: a changed Decision is rewritten
in its `DNN.md` and `INDEX.md` is regenerated; a new Decision adds a `DNN.md`
and an INDEX entry.
