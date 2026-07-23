# Phase 13 — Warnings surfaced and spend accumulated from computed cost

*Realizes design Decision 5 (warnings surfacing, priced spend rail) —
slice R-M3TR-MDZP, R-M51O-05QE — with the reporting seams of Decisions 3
and 6. Depends on Phase 12.*

`internal/runner` gains the `WarnFunc` seam: `Evaluate` takes a `warn
WarnFunc` parameter and passes each drained stream's `Warnings()` to it;
`internal/improver.Propose` does the same via `runner.WarnFunc`.
`internal/loop.Run` wires both seams to a single per-run sink writing
`warning: <setting> — <detail>` lines to `Deps.Err`, deduplicated by
warning code. Spend accumulation is proven real: cost computed from
conversation `Pricing` × usage, not provider self-report. All existing
call sites updated for the new parameters.

**Done when:**

- R-M3TR-MDZP — usage-only fake provider + priced conversations ⇒
  cumulative spend equals the exact `Pricing.Cost(usage)` sum, and a lower
  `--max-spend` trips the rail with exit 3 — covered by tagged tests.
- R-M51O-05QE — one `warning:` line on `Err` per distinct code per run
  across runner and improver streams; distinct codes get distinct lines;
  nothing on `Out` — covered by tagged tests.
- The suite is green per design Conventions.
