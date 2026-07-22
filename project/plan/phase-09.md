# Phase 9 — Composition root, health log, end-to-end

*Realizes design Decision 9 and Decision 8's remaining slice (health-log
rendering: R-RUO8-AFXR, R-RVW4-O7OG). Depends on Phases 01–08.*

End state: package `internal/app` with `Deps` and `Run` wiring cli → folder
→ config → workspace → loop per D09, the D08 health-log renderer (line
oriented, TTY-aware in-place ticker, `NO_COLOR` respected, unified diff on
a win), signal handling, and `cmd/autotune/main.go` reduced to building
real `Deps` and exiting with `Run`'s code. The end-to-end test scaffolds a
real temp folder via `--init`, fills it, and drives a full
accept-and-reject run in-process against a scripted fake provider and a
real scorer script; a binary-envelope test builds and executes the real
`autotune` binary.

**Done when:** R-RX41-1ZF5, R-RYBX-FR5U, R-RUO8-AFXR, R-RVW4-O7OG are
covered by tagged tests (in `internal/app/*_test.go` and
`cmd/autotune/*_test.go`) and `go test ./...` exits 0.
