# Phase 3 — Tune-folder load and `--init` scaffold

*Realizes design Decision 1. Depends on Phase 02 (the scaffolded
`config.json` must parse under `internal/config`).*

End state: package `internal/folder` per D01 — `Load` validating the full
folder contract (required files, executable `score`, ≥1 dev case, sorted
cases, optional holdout) with errors naming the first missing piece, and
`Init` scaffolding the exact skeleton (template `config.json` with the
contractual model defaults, executable `score` stub, example case,
`.gitignore` covering `runs/`) while refusing non-empty directories.

**Done when:** R-QFGG-82SL, R-QGOC-LUJA, R-QHW8-ZM9Z, R-QJ45-DE0O,
R-QKC1-R5RD, R-QLJY-4XI2 are covered by tagged tests in
`internal/folder/*_test.go` and `go test ./...` exits 0.
