# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[0.2.0]: https://github.com/ikigenba/autotune/releases/tag/v0.2.0
[0.1.0]: https://github.com/ikigenba/autotune/releases/tag/v0.1.0
