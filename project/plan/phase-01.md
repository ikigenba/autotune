# Phase 1 — Repo scaffold and CLI parsing

*Realizes design Decision 8 (CLI surface), slice: parsing and exit-code
taxonomy only (R-RR0J-54PO, R-RS8F-IWGD, R-RTGB-WO72); the health-log
rendering ids belong to Phase 09.*

End state: a buildable Go module `github.com/ikigenba/autotune` (Go 1.26)
with `go.mod` requiring `github.com/ikigenba/agentkit v0.7.0`, a `Makefile`
(`build`, `test`, `install` to `~/.local/bin`), a stub `cmd/autotune/main.go`
that parses and exits (full wiring arrives in Phase 09), and package
`internal/cli` implementing `Parse` (hand-rolled, both `--flag value` and
`--flag=value`, repeatable `-c`, documented defaults) and
`StopReason.ExitCode` per D08.

**Done when:** the three ids above are covered by tagged tests in
`internal/cli/*_test.go`, `go build ./...` succeeds, and `go test ./...`
exits 0.
