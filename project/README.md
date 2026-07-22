# autotune — Project Workspace

Everything needed to design, plan, and build `autotune` (the codebase at
this repository's root) lives under `project/`. Every artifact has exactly
one writer:

| folder | what's in it | written by |
|---|---|---|
| `product/` | `README.md` — the *why*: problem, users, scope, promises, success criteria | `$seal-spec` (rewritten in place) |
| `research/` | `research.md` — collected external ground truth design references | `$seal-spec` (rewritten in place) |
| `design/` | `README.md` (spine) + `INDEX.md` (manifest) + `DNN.md` (one per Decision) | `$seal-spec` (rewritten in place) |
| `plan/` | `README.md` (rules) + `STATUS.md` (manifest) + `phase-NN.md` (one per pending phase) | `$seal-spec` (appends); the build loop deletes completed phases |
| `loops/` | the generated build-loop prompts + `README.md` | a prompt-generator workflow |

See `project/loops/README.md` (once a loop is installed) for how the build
loop runs.
