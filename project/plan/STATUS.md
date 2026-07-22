# autotune — Plan Status

One line per **pending** phase in build order; this is the only place a
phase's marker lives. Each phase line is a Markdown bullet beginning with
`- Phase` carrying `⬜` (pending). The build loop finds its next work with
`grep -nE '^- Phase .* ⬜' project/plan/STATUS.md | head -1` and reads only
that phase's body file. On completion the build loop deletes the phase's
line and body file — there is no done marker; done is gone. No bare status
glyph appears outside phase lines.

Next phase: 11

- Phase 10 ⬜ realizes R-E50H-F4MC — surface failure causes on stderr (`internal/loop`, `internal/app`)
