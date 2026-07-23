# autotune — Plan Status

One line per **pending** phase in build order; this is the only place a
phase's marker lives. Each phase line is a Markdown bullet beginning with
`- Phase` carrying `⬜` (pending). The build loop finds its next work with
`grep -nE '^- Phase .* ⬜' project/plan/STATUS.md | head -1` and reads only
that phase's body file. On completion the build loop deletes the phase's
line and body file — there is no done marker; done is gone. No bare status
glyph appears outside phase lines.

Next phase: 14

- Phase 13 ⬜ realizes R-M3TR-MDZP, R-M51O-05QE — warnings surfaced and spend accumulated from computed cost
