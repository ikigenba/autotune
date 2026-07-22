# Phase 4 — Scorer subprocess contract

*Realizes design Decision 4. Depends on Phase 01.*

End state: package `internal/scorer` per D04 — exec-based `Scorer` running
`score <case-dir>` with cwd = folder root, model output on stdin, JSON
`{score, feedback}` parsed and range-checked from stdout, and every failure
mode (non-zero exit with stderr surfaced, unparseable stdout, missing or
out-of-range score) returned as a hard error. Tests use real executable
script fixtures — this is a real external contract, never mocked.

**Done when:** R-R06Q-Q6EE, R-R1EN-3Y53, R-R2MJ-HPVS, R-R3UF-VHMH are
covered by tagged tests in `internal/scorer/*_test.go` and `go test ./...`
exits 0.
