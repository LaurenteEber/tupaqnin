# Fixes P-34 and P-36, and the title instruction (before 1.7)

Objective: Eber's fixes after accepting 1.5 and 1.6. One local commit `fix: P-34, P-36` (no trailers, never push), after the 1.5b commit. Runner: `make test` · `make vet` · `make web`.

## Specs

- **S1 · P-34:** «clicking a paragraph did not open its history in the browser, though /history?block=1 answers correctly; after 1.5b, check it with a clean make web and a restarted server, and fix it if it still fails.»
- **S2 · P-36:** «the card shows «Used in: sec-4»; show section titles, never labels.»
- **S3 · B-18 instruction:** «in the principal instructions, ask it not to repeat the section title as a heading inside the text (B-18).»

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S3 | inline | done | `internal/cycle/prompts/principal.en.md` «Section titles» (draft for Eber) |
| T2 | S2 | inline, after the 1.5b writer returned | done | `TestP36_CardNamesItsSectionsByTitle`; `CardView.sections` = `{label, title}`; «Section sec-n» fallback replaced by `section.untitled` |
| T3 | S1 | inline, after 1.5b | done (not reproduced: no browser here) | clean `make web` (old bundles removed), fresh server on :8091 serves the new bundle, `history?block=1` answers; `EditorContent` forwards `onClick` (TipTap 3 `...rest`); the panel now scrolls into view on open |

## Log

- **L1 (2026-10-29, user, verbatim):** «1.5 and 1.6 accepted, including your choices. Eber’s browser test (fake provider) passed: understanding without reload, parsed stream, chain, card with assent rejected, draft → ready. Before 1.7: (1) P-34: clicking a paragraph did not open its history in the browser, though /history?block=1 answers correctly; after 1.5b, check it with a clean make web and a restarted server, and fix it if it still fails. (2) P-36: the card shows «Used in: sec-4»; show section titles, never labels. Also, in the principal instructions, ask it not to repeat the section title as a heading inside the text (B-18).»
- **L2:** The 1.5b writer is still running with `App.tsx`, the components and the generated types in its surface; T2 and T3 wait for it to avoid two writers on one file.
- **L3 (2026-10-29):** 1.5b committed `368b854` (assess: medium, review not due, under_budget). P-34 could not be clicked here; the likely cause is the panel opening below the editor out of view in the new scrolling column, now scrolled into view. The fake recording 01 still writes «# Alcance» as a heading of the section «Alcance» (tests pin it); the instruction for the real model is in `principal.en.md`.
