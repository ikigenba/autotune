# extract — example tune folder

A clone of the ikigenba wiki project's extract-prompt tuning workbench,
restated as an autotune folder. The prompt extracts subjects and claims
from source documents as JSON; scoring is the workbench's deterministic
math ported to a standalone python3 script.

`prompt.txt` is deliberately naive — the output contract and field rules
only, none of the workbench's refined guidance — so a demonstration run has
real headroom. Historical reference for the *fully refined* workbench prompt
on the same cases and scoring: baseline 0.783, tuned best 0.807, holdout
0.843, verdict "generalized"; expect this naive baseline to start well
below that.

## Contents

- `prompt.txt` — the extraction prompt under test (naive starting version).
- `improve.md` — improver system prompt (bare-call contract).
- `score` — python3 scorer; see requirements below.
- `config.json` — runner openai `gpt-5.6-luna` at low effort, improver
  openai `gpt-5.6-sol`, both `auth=sub` (one subscription login, no model
  API keys). Note: no temperature pin — the subscription backend rejects
  one; the epsilon noise floor absorbs the variance. This diverges from
  the workbench's historical eval config (anthropic `claude-sonnet-4-6`,
  temperature 0), so scores are not directly comparable to the reference
  numbers below.
- `cases/dev/` (15) and `cases/holdout/` (8) — each case is `input.txt`
  (the workbench's rendered document header + source text) plus
  `gold.json` (scorer-only gold subjects/claims; the header block inside
  it is retained for provenance and ignored by the scorer).

## Requirements

- `embed` CLI on PATH (`~/projects/embed`; override with `EMBED_BIN`) —
  the scorer gets claim embeddings through it (openai
  `text-embedding-3-small` @ 1536, the workbench pins). The embed CLI
  owns credentials (`OPENAI_API_KEY`, provided by this repo's `.envrc`)
  and its disk cache; a warm cache scores offline and free.
- OpenAI subscription login at `~/.autotune/auth.json` (from
  `oauth-login`) — used by both runner and improver.

## Run

```
autotune examples/extract
autotune --max-spend 5 -c improver.model=gpt-5.6-sol examples/extract
```

Scoring constants (threshold 0.80, margin 0.03, weights 0.35 subject /
0.50 claim / 0.15 field) live at the top of `score` — they are the
scorer's business, not autotune's.
