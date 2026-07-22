# autotune — Research

Collected external ground truth the design references. Non-contractual: the
build loop never reads this; it exists so design does not re-derive facts
about the sibling projects autotune builds on. Gathered 2026-07-22 by direct
exploration of the sibling checkouts.

## agentkit (github.com/ikigenba/agentkit, v0.7.0)

Provider-agnostic Go library for multi-turn tool-using LLM agents. Facts the
design relies on:

- Central type is `Conversation` — a plain struct, no constructor: fields
  `Provider`, `Model`, `System`, `Gen GenSettings`, `Retry RetryPolicy`,
  `Tools`, `History`, `Log io.Writer`, `MaxToolIterations`. Not safe for
  concurrent use; one conversation per goroutine.
- `conv.Send(ctx, userText) *Stream`; drain exactly once via
  `stream.Events()` (Go 1.23 range-over-func); after draining check
  `stream.Err()`, read `stream.Usage()`, `stream.Cost()`. Events are a sealed
  union: `ToolUse`, `ToolResult`, `MessageDone`.
- `GenSettings`: `Temperature *float64`, `TopP *float64`, `MaxTokens int`,
  `Reasoning ReasoningValue` (constructed via `agentkit.Level(string)`,
  `agentkit.Budget(int)`, `agentkit.DisableReasoning()`).
- `RetryPolicy`: `MaxAttempts`, `BaseDelay`, `MaxDelay`, `MaxElapsed`,
  `IgnoreRetryAfter`; zero value = sensible defaults; transient categories
  retried automatically honoring `Retry-After`.
- Rich error taxonomy: `ErrRateLimited`, `ErrOverloaded`, `ErrServerError`,
  `ErrTimeout`, `ErrNetwork`, `ErrAuthentication`, `ErrContextLength`,
  `ErrContentFilter`, plus `ErrToolLoopLimit` etc.
- Provider SPI: `Provider` interface = `RoundTrip(ctx, *Request) *RoundTrip` +
  `Name() string`. Provider subpackages (`anthropic`, `openai`, `google`,
  `openrouter`, `zai`) each expose `New(cred Credential, opts ...Option)`;
  `openai/subscription` provides OAuth-token auth via a `Store`
  (`Load(path)`, `Token(ctx)`); login itself is an external tool
  (`oauth-login`), the library does no terminal I/O.
- Advisory `catalog` package: `Lookup(model)`, `Resolve(provider, model)`,
  `Check(model, ReasoningValue)`; unknown models are never rejected, just
  unadvised.
- Cost is self-reported per stream (`stream.Cost()`); `conv.TotalUsage()` /
  `TotalCost()` accumulate across turns.
- Because the `Provider` interface is the seam, orchestration can be tested
  in-process against a scripted fake provider; the real HTTP contracts with
  provider APIs are exercised by agentkit's own test suite, not by consumers.

## agent-repl (github.com/ikigenba/agentrepl)

The `-c` convention autotune mirrors:

- Stdlib `flag`, single-dash flags. `-c key=value` is repeatable, bound to a
  `flag.Value` that appends raw strings; each is later split on the *first*
  `=` (`strings.Cut`); no `=` or an empty side is the error
  `expected key=value`.
- Values are applied in argv order, last-writer-wins, through a `config.Set`
  with a fixed key allowlist; unknown key is an error. Launch-time `-c` and
  runtime `/set` share the same code path.
- Key vocabulary (the allowlist): `auth, auth_file, base_delay, base_url,
  effort, ignore_retry_after, max_attempts, max_delay, max_elapsed,
  max_tokens, model, provider, system, temperature, thinking,
  thinking_budget, thinking_level, tool_loop_limit, top_p`.
- `auth` accepts `key` | `sub`; `sub` reads an OAuth file (default
  `~/.agentrepl/auth.json`, overridable via `auth_file`), produced by the
  external `oauth-login` helper. `key` reads the provider's API-key env var.
- No config file at all; env vars carry credentials (direnv + `~/.secrets`).
- `main.go` is a thin composition root: parse args, build a `Deps` struct
  (streams, getenv, clock, TTY detection), call `run()`, exit with its code.

## ralph

Loop-driver conventions autotune mirrors:

- Hand-rolled flag parsing supporting both `--flag value` and `--flag=value`.
- Budget rails, all `0 = unlimited`: `--max-iterations`, `--max-time`
  (duration syntax `30m`, `2h`), `--max-spend` (USD), `--max-tokens`; checked
  at the top of every cycle in a fixed order for determinism.
- Exit-code taxonomy: 0 done, 1 agent failed, 2 usage error, 3 budget rail
  crossed, 4 retries exhausted, 130 interrupted.
- A single `finish()` exit point emits the final report regardless of stop
  reason; a run ledger is appended best-effort under `~/.ralph/`.
- TTY-aware output: colorized/live only on a real character device, plain
  when piped, `NO_COLOR` respected.
- Reads harness stdout with `bufio.ReadBytes('\n')`, not `bufio.Scanner`
  (Scanner's 64KB line limit breaks on large JSONL lines).

## Prior art: the wip-autotune workbench (abandoned, in-tree at ikigenba/wiki)

A working single-project prompt tuner whose mechanics autotune generalizes.
Design decisions D64–D69 there, plus one real evidence run. What it proved:

- **Deterministic scoring is non-negotiable.** An earlier LLM-judge scorer
  was deleted outright ("non-deterministic scoring is the core defect the new
  design removes"). The rebuilt scorer was pure arithmetic.
- **Epsilon acceptance works.** Baseline measured with 3 repeats; the spread
  (max − min of the run composites) is the noise floor epsilon; a candidate
  is accepted only if `candidate > best + epsilon`. Real-run evidence: 8
  attempts, 2 accepts, and a candidate at 0.816 vs incumbent 0.807 correctly
  *rejected* for landing inside epsilon (0.0035).
- **Holdout once, at the end.** Running holdout every iteration "quietly
  turns holdout into a second dev set"; the improver must never see holdout.
  Final verdict: `holdout ≤ baseline ⇒ OVERFIT`, else generalized. Evidence
  run: dev best 0.807, holdout 0.843, verdict generalized.
- **Fresh context per improver attempt** beats an accumulating chat of failed
  theories — but attempt *memory* (a history of what was tried, its score,
  its verdict) still matters so theories are not repeated.
- **Manual adoption.** Nothing ever wrote the committed prompt; the driver
  printed a unified diff and the operator adopted deliberately. Kept `git
  status` clean and results trustworthy.
- **No reject-streak auto-stop.** A 5-consecutive-rejects stop was tried and
  removed; only budgets, a perfect dev score, or an interrupt stop the loop.
- Operational trap worth remembering (not adopted as a v1 requirement):
  agent harnesses can react to canonical `OPENAI_API_KEY`/`ANTHROPIC_API_KEY`
  env vars (unset them, switch billing); the workbench injected keys under
  `EVAL_`-prefixed aliases when driving scorers through third-party
  harnesses. autotune's scorer is a plain subprocess and inherits the
  environment untouched, so this does not bite v1, but it is the first
  suspect if a scorer ever sees mangled credentials.

## Options evaluated and not chosen (for the record)

- **Built-in scorer types** (exact-match, regex, embedding-similarity chosen
  by config): rejected — grows autotune forever and re-couples it to specific
  tasks; external-executable scoring keeps it generic. A reusable scorer-app
  library is a separate later project.
- **LLM-judge scoring**: rejected on the workbench's direct evidence.
- **Agentic (tool-using) runner and improver**: deferred — strictly more
  powerful, but drags in a workspace protocol and holdout-leak surface; a
  bare call physically cannot see what it is not handed.
- **Per-folder `auth.json`**: rejected in favor of shared
  `~/.autotune/auth.json` — one login serves all folders and no credential
  sits next to committed data.
