# autotune

`autotune` automatically improves a prompt. Pointed at a self-contained tune
folder holding a starting prompt, a dataset of cases, a scoring program, and
an improver prompt, it measures a baseline, repeatedly asks an improver model
to propose revisions, re-scores each one against the dataset, and keeps only
revisions that beat the incumbent by more than the measured run-to-run noise.
A held-out slice of cases is scored once at the end to report whether the
winner generalizes or merely overfit. Your starting prompt is never modified;
every candidate, score, and a final diff land in a run folder inside the tune
folder, and you adopt the winner deliberately.

## Build and install

Requires Go.

```
make build      # compile
make test       # run tests
make install    # build and install to ~/.local/bin/autotune
```

## Run the example

`examples/extract` tunes a subject-and-claim extraction prompt against 15 dev
and 8 holdout cases with a deterministic embedding-based scorer. It needs the
`embed` CLI on PATH and an OpenAI subscription login at `~/.autotune/auth.json`
(see `examples/extract/README.md` for details).

```
autotune --max-iterations 2 examples/extract
```

Watch the baseline, epsilon, and per-candidate ACCEPT/reject lines; when it
stops, read `examples/extract/runs/<timestamp>/summary.md` for the scores, the
holdout verdict, and the full prompt diff.

## Tuning your own prompt

`autotune --init path/to/folder` scaffolds an empty tune folder: `prompt.txt`
(the prompt under test), `cases/dev/` and `cases/holdout/` (one directory per
case), `score` (your executable scorer), `improve.md` (the improver's system
prompt), and `config.json`. You fill it in and run `autotune path/to/folder`.

In practice the fastest way to fill one in is to have a coding agent build it
for you. [`AGENT.md`](AGENT.md) is a prompt you can hand to any agent that
teaches it the tune-folder contract. For example:

> Read AGENT.md from github.com/ikigenba/autotune. Build a tune folder at
> `tune/summarize` for the prompt in `src/prompts/summarize.txt`, which
> summarizes support tickets into one paragraph. Draw 20 dev and 8 holdout
> cases from `data/tickets/`, write gold summaries for me to review, and
> write a scorer that rewards factual coverage of the gold summary and
> penalizes length over 100 words.

Then run it with whatever budget you like:

```
autotune --max-spend 5 --max-time 1h tune/summarize
```
