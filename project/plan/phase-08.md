# Phase 8 — Tuning loop engine

*Realizes design Decision 5. Depends on Phases 05, 06, 07.*

End state: package `internal/loop` per D05 — baseline over `--repeat`
evaluations yielding mean baseline and max−min epsilon, the iterate cycle
(rails in fixed order, improver proposal, single dev evaluation, strict
`candidate > best + epsilon` acceptance), stop conditions (rails → 3,
perfect score → 0, cancellation → 130), and a single finalize path that
always runs: at-most-once holdout evaluation of an accepted winner with the
`OVERFIT`/`generalized` verdict, honest no-improvement reporting, and
summary/workspace writes. Tests drive scripted fake providers and an
injected clock.

**Done when:** R-R52C-99D6, R-R7I5-0SUK, R-R8Q1-EKL9, R-R9XX-SCBY,
R-RB5U-642N, R-RCDQ-JVTC, R-RDLM-XNK1 are covered by tagged tests in
`internal/loop/*_test.go` and `go test ./...` exits 0.
