# autotune — Product

**Authority: intent.** This document owns the *why*: the problem, the users,
the scope, and what we promise. It does not state mechanism, formats, exit
codes, or test assertions — those belong to design. Where behavior appears
here it is a *promise* in outcome terms; design states the exact, checkable
proof of each promise.

## Problem

Prompts for LLM tasks are improved by hand: run, eyeball, tweak, repeat. The
loop is slow, unmeasured, and biased — "looks better" is not a score, and a
tweak that helps one example silently hurts three others. A previous attempt
embedded an auto-tuning harness inside the target project itself, which worked
but coupled the tuning machinery to one codebase and made it unusable anywhere
else.

## Purpose

`autotune` is a standalone command-line tool that automatically improves a
prompt. Pointed at a self-contained folder holding a starting prompt, a
dataset of cases, a scoring program, and an improver prompt, it measures a
baseline, repeatedly proposes revised prompts, re-scores them against the
dataset, and keeps only revisions that are genuinely better — unattended,
until a budget runs out or no further gain is found.

## Users

The prompt author: a developer who owns a prompt used somewhere in production
(extraction, analysis, classification, generation) and wants it measurably
better without spending hours in a manual tweak loop. They can write a small
scoring program in any language and curate example cases; they run `autotune`
from a terminal and read its output.

## Scope

autotune does exactly one job: optimize a single prompt against a scored
dataset inside one structured folder. It scaffolds new tune folders, runs the
optimization loop, records everything it tried, and reports the winner. It is
provider-agnostic (any model the underlying agent library supports) and
scoring-agnostic (scoring is the user's program, not autotune's opinion).

Nothing else. In particular, v1 deliberately excludes: a library of reusable
scoring programs (a separate later project), tool-using/agentic execution of
the prompt under test, automatic adoption of the winner into the folder's
starting prompt, machine-readable output streams, and resuming an interrupted
run.

## Contractual constants

- Default runner model (executes the prompt under test): `gpt-5.6-luna`.
- Default improver model (proposes revisions): `gpt-5.6-sol`.
- Both default to subscription authentication, sharing one login file at
  `~/.autotune/auth.json`.

## What we promise (user-facing behavior)

- `autotune --init path/to/folder` lays out a complete, working tune-folder
  skeleton — starting prompt, improver prompt, scoring program stub, config,
  example case — that the user fills in. It refuses to touch a folder that
  already has contents.
- `autotune [options] path/to/folder` runs the whole cycle unattended:
  baseline first, then improve → evaluate → keep-if-better, over and over,
  within the budgets the user set (iterations, wall-clock time, dollars).
- Scoring is the user's own program, run for every case; autotune trusts its
  numbers and nothing else. Improvements are kept only when they beat the
  incumbent by more than the measured run-to-run noise, so a kept prompt is a
  real improvement, not luck.
- A held-out portion of the dataset, never shown to the improvement process,
  is used once at the end to tell the user whether the winner generalizes or
  merely overfit the development cases.
- The user's starting prompt file is never modified. The winner, every
  candidate, every score, and a human-readable history land in a run folder
  inside the tune folder; at the end the user sees exactly what changed (a
  diff) and adopts it deliberately.
- While running, autotune continuously logs its progress — every baseline
  measurement, every candidate's score and keep/reject verdict, running cost —
  so a glance at the terminal shows it is alive and converging.
- Configuration is passed generically as `-c key=value`, consistent with the
  user's other agent tools; the runner and the improver are configured
  independently, and each folder pins its own reproducible defaults.
- If anything the scores depend on breaks — the scoring program fails, a model
  call fails for good — autotune stops loudly rather than reporting numbers it
  cannot stand behind.

## Success criteria (outcomes)

- A user can scaffold a new tune folder, fill in prompt/cases/scorer, and get
  a complete tuning run end to end with no other setup.
- On a real dataset, a run produces either a winner that scores measurably
  better than baseline on held-out cases, or an honest report that no genuine
  improvement was found — never a false win.
- The user can reconstruct any run afterwards from the run folder alone: what
  was tried, what each attempt scored, why it was kept or rejected, what it
  cost.
- The starting prompt file is byte-identical before and after every run.
- A run left overnight respects the budgets it was given and ends on its own.
- Watching the terminal during a run answers "is it healthy and making
  progress?" without consulting any other file.
- The same folder, replayed with the same pinned config, produces scores
  comparable within the reported noise floor.
