# autotune agent guide

You are building a tune folder for `autotune`, a CLI that automatically
improves a prompt: it scores a baseline, asks an improver model for revised
prompts, re-scores each revision, and keeps only revisions that beat the best
so far by more than the measured run-to-run noise. A holdout slice is scored
once at the end to check the winner generalizes.

## Required tools

Do not install anything; check these are present and tell the user what is
missing:

- `autotune` on PATH (`command -v autotune`).
- `embed` on PATH (`command -v embed`), only if your scorer compares text by
  embedding similarity (see below).
- Whatever interpreter your `score` program needs (e.g. `python3`).
- Provider credentials: by default both models use a subscription login at
  `~/.autotune/auth.json`; check the file exists.

## Tune folder contract

`autotune --init <folder>` scaffolds the layout; you replace every stub:

- `prompt.txt` — the prompt under test, verbatim. Start from the user's
  existing prompt. Never edit it after the run starts; winners land in
  `runs/<id>/best/prompt.txt` and the user adopts them by hand.
- `cases/dev/<name>/` — one directory per development case. Each holds
  `input.txt` (what the prompt runs against) plus any gold/reference files
  your scorer reads. 10 to 20 diverse cases is a good start.
- `cases/holdout/<name>/` — same shape, never shown to the improver, scored
  once at the end. Roughly a third the size of dev.
- `score` — an executable (any language, `chmod +x`). autotune runs it as
  `./score <absolute-case-dir>` with cwd = the tune folder and the model's
  output for that case on stdin. It must print one JSON object to stdout:
  `{"score": <0..1>, "feedback": "<optional text for the improver>"}`.
  It must be deterministic: same output, same score. Any failure (non-zero
  exit, bad JSON, score out of range) hard-aborts the whole run, so if the
  *model's* output is malformed, print `{"score": 0, "feedback": "why"}` and
  exit 0 instead of crashing. Rich feedback (what was missed, what was
  spurious) is what makes the improver effective.
- `improve.md` — the improver's system prompt. Describe the task, how
  scoring works, and instruct it to diagnose the worst cases' feedback, make
  one focused change, not repeat rejected theories from the history, and
  reply as `SUMMARY: <one line>` followed by the complete revised prompt in a
  fenced code block. See `examples/extract/improve.md` for a working model.
- `config.json` — `{"runner": {...}, "improver": {...}}`; each section takes
  `provider`, `model`, `auth`, and optional sampling keys like `effort` or
  `temperature`. The runner executes `prompt.txt` on each case; the improver
  proposes revisions. Any key is overridable at run time with
  `-c section.key=value`.

## Scoring with embeddings

For fuzzy text comparison (did the output say the same thing as the gold?),
exact string matching is too brittle. The
[`embed`](https://github.com/ikigenba/embed) CLI exists for this: pipe it
a JSON array of strings on stdin and it prints
`{"model", "provider", "dimensions", "embeddings"}` with one vector per input
string, in order. Batch every string from a case into one call, then compare
by cosine similarity with a threshold. `embed` owns its own credentials and
disk cache, so a warm cache scores offline and free. Honor an `EMBED_BIN`
environment override for the binary path. `examples/extract/score` is a full
worked example.

## Running

```
autotune <folder>                          # until no further gain
autotune --max-iterations 2 <folder>       # quick smoke run
autotune --max-spend 5 --max-time 1h <folder>
```

Verify the folder before a real run: run `./score` by hand on one case with a
hand-written good output (should score near 1) and a bad one (should score
low). Then suggest the user start with a small `--max-iterations` run and
read `runs/<id>/summary.md`.
