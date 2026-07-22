# Phase 5 — Case runner with bounded parallelism

*Realizes design Decision 3. Depends on Phases 02, 03, 04.*

End state: package `internal/runner` per D03 — `Evaluate` running one bare
agentkit call per case (fresh conversation via the `NewConv` seam, system =
candidate prompt, user = `input.txt`), a worker pool bounded by `parallel`,
per-case scoring as calls complete, name-sorted deterministic aggregation
into `EvalResult` with mean composite and summed cost, and whole-evaluation
failure with cancellation on any terminal call or scorer error. Tests drive
a scripted fake provider through the seam.

**Done when:** R-QVB5-73FM, R-QWJ1-KV6B, R-QXQX-YMX0, R-QYYU-CENP are
covered by tagged tests in `internal/runner/*_test.go` and `go test ./...`
exits 0.
