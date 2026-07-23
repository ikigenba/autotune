# autotune — Design Index

Each Decision maps to its `DNN.md`; every Verification id maps to its
Decision and file. Id lookup is a grep against this index. Regenerated
whenever a Decision is added or its Verification ids change.

## Decisions

- D1 → `D01.md` — Tune-folder contract and `--init` — R-QFGG-82SL,
  R-QGOC-LUJA, R-QHW8-ZM9Z, R-QJ45-DE0O, R-QKC1-R5RD, R-QLJY-4XI2
- D2 → `D02.md` — Config model: two agents, namespaced `-c`, folder pins,
  catalog pricing, spend pre-check — R-QMRU-IP8R, R-QNZQ-WGZG, R-QQFJ-O0GU,
  R-QRNG-1S7J, R-QSVC-FJY8, R-QU38-TBOX, R-M1DY-UUIB, R-M2LV-8M90
- D3 → `D03.md` — Runner: one bare call per case, bounded parallelism —
  R-QVB5-73FM, R-QWJ1-KV6B, R-QXQX-YMX0, R-QYYU-CENP
- D4 → `D04.md` — Scorer contract: external executable, hard failure —
  R-R06Q-Q6EE, R-R1EN-3Y53, R-R2MJ-HPVS, R-R3UF-VHMH
- D5 → `D05.md` — Tuning loop: baseline, epsilon acceptance, rails,
  warnings surfacing, finalize, graceful degradation — R-R52C-99D6,
  R-R7I5-0SUK, R-R8Q1-EKL9, R-R9XX-SCBY, R-RB5U-642N, R-RCDQ-JVTC,
  R-RDLM-XNK1, R-E50H-F4MC, R-M3TR-MDZP, R-M51O-05QE, R-SKPC-4XN1,
  R-SKPU-8QR2, R-FNST-6LM3
- D6 → `D06.md` — Improver: fresh-context bare call, evidence bundle —
  R-RETJ-BFAQ, R-RG1F-P71F, R-RH9C-2YS4, R-RIH8-GQIT
- D7 → `D07.md` — Run workspace: `runs/<id>/`, incumbent never touched —
  R-RJP4-UI9I, R-RM4X-M1QW, R-RNCT-ZTHL, R-ROKQ-DL8A, R-RPSM-RCYZ
- D8 → `D08.md` — CLI surface, exit codes, health-log output —
  R-RR0J-54PO, R-RS8F-IWGD, R-RTGB-WO72, R-RUO8-AFXR, R-RVW4-O7OG
- D9 → `D09.md` — Composition root, seams, end-to-end proof —
  R-RX41-1ZF5, R-RYBX-FR5U
- D10 → `D10.md` — Versioning, release, and install tooling (embed's
  scheme) — R-9A0E-BG7N, R-9B8A-P7YC

## Verification ids → Decision

- R-9A0E-BG7N → D10 (`D10.md`)
- R-9B8A-P7YC → D10 (`D10.md`)
- R-E50H-F4MC → D5 (`D05.md`)
- R-FNST-6LM3 → D5 (`D05.md`)
- R-M1DY-UUIB → D2 (`D02.md`)
- R-M2LV-8M90 → D2 (`D02.md`)
- R-M3TR-MDZP → D5 (`D05.md`)
- R-M51O-05QE → D5 (`D05.md`)
- R-QFGG-82SL → D1 (`D01.md`)
- R-QGOC-LUJA → D1 (`D01.md`)
- R-QHW8-ZM9Z → D1 (`D01.md`)
- R-QJ45-DE0O → D1 (`D01.md`)
- R-QKC1-R5RD → D1 (`D01.md`)
- R-QLJY-4XI2 → D1 (`D01.md`)
- R-QMRU-IP8R → D2 (`D02.md`)
- R-QNZQ-WGZG → D2 (`D02.md`)
- R-QQFJ-O0GU → D2 (`D02.md`)
- R-QRNG-1S7J → D2 (`D02.md`)
- R-QSVC-FJY8 → D2 (`D02.md`)
- R-QU38-TBOX → D2 (`D02.md`)
- R-QVB5-73FM → D3 (`D03.md`)
- R-QWJ1-KV6B → D3 (`D03.md`)
- R-QXQX-YMX0 → D3 (`D03.md`)
- R-QYYU-CENP → D3 (`D03.md`)
- R-R06Q-Q6EE → D4 (`D04.md`)
- R-R1EN-3Y53 → D4 (`D04.md`)
- R-R2MJ-HPVS → D4 (`D04.md`)
- R-R3UF-VHMH → D4 (`D04.md`)
- R-R52C-99D6 → D5 (`D05.md`)
- R-R7I5-0SUK → D5 (`D05.md`)
- R-R8Q1-EKL9 → D5 (`D05.md`)
- R-R9XX-SCBY → D5 (`D05.md`)
- R-RB5U-642N → D5 (`D05.md`)
- R-RCDQ-JVTC → D5 (`D05.md`)
- R-RDLM-XNK1 → D5 (`D05.md`)
- R-RETJ-BFAQ → D6 (`D06.md`)
- R-RG1F-P71F → D6 (`D06.md`)
- R-RH9C-2YS4 → D6 (`D06.md`)
- R-RIH8-GQIT → D6 (`D06.md`)
- R-RJP4-UI9I → D7 (`D07.md`)
- R-RM4X-M1QW → D7 (`D07.md`)
- R-RNCT-ZTHL → D7 (`D07.md`)
- R-ROKQ-DL8A → D7 (`D07.md`)
- R-RPSM-RCYZ → D7 (`D07.md`)
- R-RR0J-54PO → D8 (`D08.md`)
- R-RS8F-IWGD → D8 (`D08.md`)
- R-RTGB-WO72 → D8 (`D08.md`)
- R-RUO8-AFXR → D8 (`D08.md`)
- R-RVW4-O7OG → D8 (`D08.md`)
- R-RX41-1ZF5 → D9 (`D09.md`)
- R-RYBX-FR5U → D9 (`D09.md`)
- R-SKPC-4XN1 → D5 (`D05.md`)
- R-SKPU-8QR2 → D5 (`D05.md`)
