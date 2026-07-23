# Phase 12 — Catalog pricing on conversations and the spend pre-check

*Realizes design Decision 2 (config: catalog pricing, spend pre-check) —
slice R-M1DY-UUIB, R-M2LV-8M90.*

`internal/config` gains real cost plumbing: `Section.Conversation()` sets
the conversation's `Pricing` from `catalog.Lookup` for catalog-known models
(nil for off-catalog), and `Config.PricingPrecheck(maxSpend)` refuses a
positive `--max-spend` when either section's model is off-catalog, naming
the section and model. The composition root (`internal/app`) runs the
pre-check after config resolution and before any provider call, mapping a
violation to the usage exit code.

**Done when:**

- R-M1DY-UUIB — catalog-known model ⇒ non-nil `Pricing` equal to the
  catalog entry's, identical under `auth=key` and `auth=sub`; off-catalog
  model ⇒ nil `Pricing` — covered by tagged tests.
- R-M2LV-8M90 — pre-check errors name section and model; `maxSpend == 0`
  or all-known passes; via the CLI the violation exits 2 with zero provider
  calls — covered by tagged tests.
- The suite is green per design Conventions.
