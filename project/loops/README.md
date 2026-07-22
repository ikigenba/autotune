# autotune — Build Loop

This is the installed unattended build loop: three prompts, re-invoked with a
**fresh context** every turn, that build `project/plan/`'s pending phases one
at a time until the plan is empty.

## Running it

```
project/loops/run
```

which wraps exactly:

```
ralph project/loops/gather.md project/loops/build.md project/loops/verify.md
```

`ralph` runs from the service root (the repository root), so every path the
three prompts reference is root-relative (`project/…`). It reads only each
turn's **last** message and cycles the prompts `gather → build → verify →
gather → …` on that message's terminal status.

## The status contract

Every turn ends with exactly one terminal status; `ralph` advances on it:

| status | terminal? | means | who reports it |
|---|---|---|---|
| `CONTINUE` | no | progress message emitted mid-turn (some backends, e.g. gpt-5.5 under codex, tag every streamed message with a status) | any of the three, never as the last message |
| `NEXT` | yes | this turn's work is done; hand off to the next prompt | `gather` (when a phase is still pending), `build` (always), `verify` (always) |
| `DONE` | yes | the whole job is complete; the loop stops | `gather` only, and only when `STATUS.md` has no `⬜` phase left |

`build` and `verify` never report `DONE` — completing a phase's work, or
passing it, is still `NEXT`; only `gather` finding zero pending phases ends
the run.

## Per-step reads / writes / commits / deletions

| step | reads | writes | commits | deletes |
|---|---|---|---|---|
| `gather` | `project/plan/STATUS.md`; on a fresh brief also one `phase-NN.md`, `project/design/INDEX.md`, the realized `DNN.md`(s) | `project/loops/brief.md` (contract region only, on a fresh brief) | never | never |
| `build` | `project/loops/brief.md` only | source + test files under `internal/`, `cmd/` | yes, one increment per turn | never |
| `verify` | `project/loops/brief.md`; runs the suite | `project/loops/brief.md` (feedback region, full overwrite) or `project/plan/STATUS.md` (line removal) | yes, on a pass (retirement) or a stall reset (log line, no source) | `project/plan/phase-NN.md` + its `STATUS.md` line on a pass; `project/loops/brief.md` on a pass or a stall reset |

## The brief's lifecycle

`project/loops/brief.md` is the seam that keeps `build`'s context scoped to
one phase — the complete and only input `build` and `verify` consume, so
neither opens `project/design/` or `project/plan/`. It is **never
committed** (`project/loops/.gitignore` excludes it), **single-phase**, and
**phase-scoped, not per-cycle**:

- `gather` authors it once, when a phase first becomes the active `⬜`
  phase — reading the phase body, resolving its Decision(s), and copying in
  their full design prose (minus the Verification list) and the full text
  of only the ids that phase owns.
- It **persists across cycles** while that phase stays `⬜`. `gather`
  no-ops on an in-flight brief (same phase name in its header) rather than
  re-reading the big docs every cycle.
- `build` reads the whole brief (contract + feedback) every turn, prioritizes
  any open gaps in the feedback region, and does as much of the remaining
  work as cleanly fits one turn. It never writes the brief.
- `verify` re-derives coverage and suite state independently every turn.
  Pass → deletes the brief (along with retiring the phase). Gap → overwrites
  the feedback region with the currently-open gaps, or — after three
  consecutive no-progress attempts on the same gaps — discards the brief
  entirely (a stall reset) so the next `gather` rebuilds the contract fresh.

## Why it converges

`verify` can neither halt the loop nor advance a phase on a gap, so an
incomplete phase simply stays `⬜` and gets re-attacked next cycle — now with
`verify`'s grounded feedback in front of `build`. The persisted feedback
gives `verify` cross-cycle memory: it can tell slow convergence (the open-gap
set shrinking or changing) from a true stall (the same gap ids, unsatisfied
across three consecutive attempts, with no new build commit), and a stall
resets the trajectory rather than looping forever on a stuck contract. The
only exit is `gather` finding zero `⬜` phases, which requires every phase to
have passed `verify` — so the run ends only when the plan is fully built (or
a `ralph` budget rail trips).

## `project/loops/brief.md` schema

A gather-owned **contract region** (written once per phase, untouched by
`verify`) and a verify-owned **feedback region** (written only by `verify`,
overwritten in full each gap cycle, untouched by `gather` on an in-flight
brief):

```
# Brief — Phase NN

## Objective
## Realizes
## Design               (per realized Decision: Decision + Rejected prose, Verification list omitted)
## Ids to cover          (R-XXXX-XXXX — <full requirement text>, one per line, only this phase's ids)
## Files to touch
## Dependency interfaces (public signatures of packages this phase consumes but doesn't build)
## Done bar

## Verify feedback       (empty on a fresh brief; verify overwrites with attempt N, build commit,
                          stall streak, and only the currently-open gaps, each tied to an R-id
                          and the exact command/output that proves it open)
```

See `project/loops/gather.md`, `project/loops/build.md`, and
`project/loops/verify.md` for the exact procedures and boundaries each step
follows.
