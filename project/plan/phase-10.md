# Phase 10 — Surface failure causes on stderr

*Realizes design Decision 5 (Failure reporting), slice: R-E50H-F4MC.*

End state: `internal/loop`'s `Deps` carries an `Err io.Writer` (defaulting
to `io.Discard` when nil), every failure stop path — baseline evaluation,
improver proposal, candidate evaluation, workspace writes, holdout
evaluation — writes the underlying error text to it before the final
report, and `internal/app` wires the process stderr into it. Observable
end state: a run that dies on its first model call prints the actual
error, not just `stop: internal failure`.

**Done when:** R-E50H-F4MC is covered by a tagged test (a failing
evaluation's error message asserted present on the `Err` writer, and
absent-only-classification asserted gone) and `go test ./...` exits 0.
