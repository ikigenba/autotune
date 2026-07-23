# autotune

A standalone Go CLI that automatically improves a prompt against a scored
dataset: measure a baseline, ask an improver model for revisions, re-score
each, and keep only the ones that beat the best so far by more than the
measured run-to-run noise; a holdout slice is scored once at the end to check
the winner generalizes. Module `github.com/ikigenba/autotune`, binary
`autotune` from `cmd/autotune`.

`AGENT.md` (singular) is a different file — the prompt an end user hands to an
agent to *build a tune folder*, referenced from the README. This file is for
developing autotune itself.

## How changes are made

This repo is spec-driven. The source of truth is `project/`, not the code: you
change behavior by changing the spec and letting the build loop realize it, not
by editing `internal/` directly. Reach for a direct code edit only when the
operator explicitly asks for one — a bug fix, a bug report with a clear cause,
or an instruction naming the file. "Route this through the spec" means *seal the
spec first*, not "implement it however."

The loop, end to end:

1. Update `project/product` (the why) and `project/design` (the how + its
   proof) in place. New or changed behavior mints/adjusts `R-XXXX-XXXX`
   Verification ids; regenerate `design/INDEX.md`.
2. Append a `project/plan/phase-NN.md` and its `STATUS.md` line for the ids
   that aren't yet covered by a tagged test.
3. Let the build loop (`$ralph`) build the phase and delete it on completion.

The authorities and rules already live under `project/` — read
`project/README.md`, `project/plan/README.md`, and `project/design/README.md`,
and the `$ikispec`, `$open-spec`, `$seal-spec`, and `$ralph` skills. Don't
restate them here; this section just says *which* path to take.

## Layout

- `cmd/autotune/` — thin composition root; builds a `Deps` struct (streams,
  getenv, clock, TTY, provider factory) and calls `internal/`.
- `internal/` — all logic, run against injected deps: `cli`, `config`,
  `folder`, `scorer`, `runner`, `improver`, `loop`, `workspace`, `app`. The
  agentkit `Provider` interface is the model-call seam; tests use a scripted
  fake provider.
- `project/` — the spec (see above). `examples/` — a worked tune folder.

## Working in the tree

- `go build ./...` typechecks; `go test ./...` is the suite and must exit 0
  with no network or credentials. `gofmt` clean.
- Each realized `R-XXXX-XXXX` id appears verbatim in exactly one `*_test.go`.
- Fail loudly: validate external input (flags, `config.json`, scorer output,
  improver replies) at the boundary; assert internal invariants. The shared
  exit-code taxonomy and versioning scheme are owned by `project/design`
  (D8/D10) — check there before changing them.
