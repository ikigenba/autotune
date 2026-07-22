# Phase 7 — Run workspace

*Realizes design Decision 7. Depends on Phases 02, 03, 05 (config, folder,
runner types).*

End state: package `internal/workspace` per D07 — `Create` making the
UTC-timestamped `runs/<id>/` tree from the injected clock, writers for the
config stamp, baseline, per-candidate prompt+scorecard (zero-padded,
byte-deterministic JSON), `PromoteBest` on accept only, append-only
`history.md`, and `WriteSummary` from the workspace-owned `Summary` struct.
The folder's `prompt.txt` is never written by any code path in this
package.

**Done when:** R-RJP4-UI9I, R-RM4X-M1QW, R-RNCT-ZTHL, R-ROKQ-DL8A,
R-RPSM-RCYZ are covered by tagged tests in `internal/workspace/*_test.go`
and `go test ./...` exits 0.
