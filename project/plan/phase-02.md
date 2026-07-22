# Phase 2 — Config resolution

*Realizes design Decision 2. Depends on Phase 01.*

End state: package `internal/config` per D02 — `ParsePair`, `Resolve` over
the three layers (built-in defaults, `config.json` sections, namespaced `-c`
pairs), the full key vocabulary with typed landing spots in `Section`,
`auth key|sub` semantics with the `<home>/.autotune/auth.json` default, and
`Section.Conversation` building an `agentkit.Conversation` through an
injectable provider factory so tests never touch the network.

**Done when:** R-QMRU-IP8R, R-QNZQ-WGZG, R-QQFJ-O0GU, R-QRNG-1S7J,
R-QSVC-FJY8, R-QU38-TBOX are covered by tagged tests in
`internal/config/*_test.go` and `go test ./...` exits 0.
