# Phase 6 — Improver bundle and proposal

*Realizes design Decision 6. Depends on Phase 05 (runner types, `NewConv`
seam).*

End state: package `internal/improver` per D06 — `BuildBundle` rendering the
evidence bundle (incumbent, baseline/best/epsilon, full per-case score
table, worst-K detail with scorer feedback, attempt-history lines; holdout
never present by construction), `Parse` extracting the `SUMMARY:` line and
the single fenced replacement prompt, and `Propose` running sequential
fresh-context calls with malformed-reply retries up to the limit.

**Done when:** R-RETJ-BFAQ, R-RG1F-P71F, R-RH9C-2YS4, R-RIH8-GQIT are
covered by tagged tests in `internal/improver/*_test.go` and
`go test ./...` exits 0.
