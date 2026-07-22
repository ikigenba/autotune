# autotune — Plan Status

One line per **pending** phase in build order; this is the only place a
phase's marker lives. Each phase line is a Markdown bullet beginning with
`- Phase` carrying `⬜` (pending). The build loop finds its next work with
`grep -nE '^- Phase .* ⬜' project/plan/STATUS.md | head -1` and reads only
that phase's body file. On completion the build loop deletes the phase's
line and body file — there is no done marker; done is gone. No bare status
glyph appears outside phase lines.

Next phase: 10

- Phase 05 ⬜ realizes R-QVB5-73FM, R-QWJ1-KV6B, R-QXQX-YMX0, R-QYYU-CENP — case runner with bounded parallelism (`internal/runner`)
- Phase 06 ⬜ realizes R-RETJ-BFAQ, R-RG1F-P71F, R-RH9C-2YS4, R-RIH8-GQIT — improver bundle and proposal (`internal/improver`)
- Phase 07 ⬜ realizes R-RJP4-UI9I, R-RM4X-M1QW, R-RNCT-ZTHL, R-ROKQ-DL8A, R-RPSM-RCYZ — run workspace (`internal/workspace`)
- Phase 08 ⬜ realizes R-R52C-99D6, R-R7I5-0SUK, R-R8Q1-EKL9, R-R9XX-SCBY, R-RB5U-642N, R-RCDQ-JVTC, R-RDLM-XNK1 — tuning loop engine (`internal/loop`)
- Phase 09 ⬜ realizes R-RX41-1ZF5, R-RYBX-FR5U, R-RUO8-AFXR, R-RVW4-O7OG — composition root, health log, end-to-end (`internal/app`, `cmd/autotune`)
