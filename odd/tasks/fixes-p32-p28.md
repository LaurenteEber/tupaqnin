# Fixes P-32 and P-28 before 1.5, and a fake card recording

Objective: the fixes Eber asked after accepting 1.3 and 1.4. Branch `main`; one local commit `fix: P-32, P-28` when green, no trailers, never push. Then piece 1.5. Runner: `make test` · `make vet` · `make web`.

## Specs

- **S1 · P-32:** «if a section’s role is confirmed, an agent proposal must not change it back to proposed; keep the proposal in the step end as not applied (like B-17).»
- **S2 · P-28:** «App.tsx renders the raw delta stream in a <pre>; show the parsed text, triage line and note, and keep the raw stream only behind a dev flag.»
- **S3 · Recording:** «yes, add a fake step recording with a critical decision so Eber can test the card in the browser now.»
- **S4 · Assents:** «keep assent words from both languages» (no change).

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S1 | inline | done | `TestP32_*` (step and understanding); mutation fails both |
| T2 | S2 | inline | done | `TestP28_StreamedPiecesCarryTheirFieldAndNoRawInTheDemoBuild`, `TestP28_DevBuildAlsoStreamsTheRawText` (devroutes); App renders triage, text, note apart; raw in `<details>` |
| T3 | S3 | inline | done | `TestT5_DemoRecordingOpensACard`; curl :8091 case-5 → `/todo` has card-1 |

## Log

- **L1 (2026-10-29, user, verbatim):** «1.3 and 1.4 accepted, including all your choices except one, and both doubts: keep assent words from both languages; yes, add a fake step recording with a critical decision so Eber can test the card in the browser now. Fix before 1.5: (1) P-32: if a section’s role is confirmed, an agent proposal must not change it back to proposed; keep the proposal in the step end as not applied (like B-17). (2) P-28 is still open: App.tsx renders the raw delta stream in a <pre>; show the parsed text, triage line and note, and keep the raw stream only behind a dev flag. Note: Eber’s docs copy had blanked docs/construccion/elecciones-agente.md; he restored it from your commit. Then continue with 1.5.»
- **L2:** The rejected choice is the 1.3 row «El agente puede volver a proponer el rol… queda proposed» (P-32 reverses it). Verified P-28: since 1.3 the Go stream already carries only the decoded triage line, text and note (`fieldFilter`, `TestP28_*`); what stays open is the page: one `<pre>` with everything, no separate triage and note, no raw view for development. Dev flag = the existing `devroutes` build tag (B-4), no new config option. Eber's server on :8080 was not running, so his binary could not be checked.
