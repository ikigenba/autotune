# autotune — Plan

**Authority: construction order.** This document and `project/plan/` own the
build order of **pending** work only. Completion is deletion: the build loop
removes a finished phase's `STATUS.md` line and its `phase-NN.md` in the
completion commit; construction history lives in git, never here. To extend
the project: update product and design in place first, then append a new
`phase-NN.md` plus a `STATUS.md` line, numbered from the counter — never
renumber, never reuse a number. Coverage invariant: every *current* design
Verification id is either already realized by a tagged test in the codebase
or assigned to exactly one pending phase.

**One phase = one package = one build-turn context.** Each phase is a single
coherent unit of work — almost always one package — scoped to that unit's
design Decisions and the *interfaces* (not internals) of the packages it
depends on, and sized so the build loop can carry it in one fresh build-turn
context, ideally finishing in a turn or two. If a Decision is too large for
one context it is split across phases, each naming its slice of the
Decision's Verification ids.

**Done bar.** A phase is done when every Verification id it realizes (or its
explicit slice) is covered by a clearly-named test and the suite is green —
see design's Conventions for the exact commands (`go test ./...` exits 0).
Every phase's acceptance bar is deterministic exit conditions, never a
subjective judgment, never a self-referential or unsatisfiable check.

## Layout

`STATUS.md` is the manifest: the `Next phase` counter plus the **only** home
of the pending markers. `phase-NN.md` is one body file per pending phase
(zero-padded). This README is the static rules. The build loop's only
mutations are removing a finished phase's `STATUS.md` line together with its
`phase-NN.md`; the counter is never decremented and never touched by the
loop.
