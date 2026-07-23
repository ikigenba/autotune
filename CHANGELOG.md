# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.1] - 2026-07-23

### Fixed

- Improver robustness: a malformed improver reply no longer aborts the whole
  run. `SUMMARY:` parsing is lenient (tolerates leading whitespace and markdown
  emphasis such as `**SUMMARY:**`, and ignores a `SUMMARY:` line inside the
  prompt fence); a parse failure is retried with the prior error fed back to
  the model so it can correct itself; and if a revision still cannot be read,
  that one iteration is skipped and recorded while the run continues on the
  best-so-far and ends through its normal budgets. Only a persistently unusable
  improver (no readable revision for several consecutive iterations) stops the
  run with a failure.
- The final report now always ends with a `stop: <reason>` line, including on
  the accepted-winner path (previously it ended mid-diff with no stop line).
  Runs that skipped an iteration report a `skipped: <n>` count.

## [0.2.0] - 2026-07-23

### Fixed

- Cost reporting: conversations now carry catalog pricing, so running and
  final cost is real for every catalog-known model, on all providers and in
  both auth modes (previously $0 everywhere except openrouter).

### Added

- `--max-spend` pre-check: refuses to start when a configured model has no
  catalog pricing, instead of running uncapped.
- Provider warnings (degraded settings, unknown cost) are surfaced on
  stderr, deduplicated by warning code.

## [0.1.0] - 2026-07-22

### Added

- Initial release: tune-folder scaffolding (`--init`), the baseline /
  improve / evaluate / keep-if-better loop with epsilon acceptance, budget
  rails, external-executable scoring, one-shot holdout verdict, run
  workspace with full history, and prebuilt-release install via
  `curl | sh` or `make install`.

[0.2.1]: https://github.com/ikigenba/autotune/releases/tag/v0.2.1
[0.2.0]: https://github.com/ikigenba/autotune/releases/tag/v0.2.0
[0.1.0]: https://github.com/ikigenba/autotune/releases/tag/v0.1.0
