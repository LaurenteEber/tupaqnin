# Signoff balance and UI fixes from Eber's Chrome pass (P-128, C.21 to C.26)

Objective: the signoff reserve and its notice, then C.21 to C.25 in order, and C.26 if time remains. Branch `main`; one local commit each, explicit paths, no trailers, no push. Stop at 16:00 Lima time and report whatever is done.
Runner: `make test` · `make vet` · `make web` · govulncheck · headless smoke at 1440 px with a screenshot, in en and es. Delivery: not applicable (Eber pushes).

## Specs

Quoted verbatim from L1.

- **S1 · reserve:** «Raise the signoff reserve from about 2 contrasts to about 4 [initial] (models.go and wherever B3-20's reserve is computed). Tests that pin the reserve follow.»
- **S2 · notice:** «When the balance is spent, the signoff screen says: «Se agotó el saldo de la firma. Tu explicación y tu trabajo quedan guardados; las secciones sin decisiones críticas sí se pueden firmar.» (en and es).»
- **S3 · C.21:** «The four B-99 proposal origins have no text: proposal.origin.removes_half, removes_support, breaks_sentence, rewrites_sentence_end show as raw keys. Add them to en.json and es.json in plain words («El cambio quita más de la mitad de la sección», «Borra un párrafo que sostiene una decisión o una cita», «Deja una oración sin cerrar», «Cambia el final de una oración»). Extend the catalog test so every Go origin constant has a text in both catalogs.»
- **S4 · C.22:** «A proposal that removes paragraphs shows «Entra:» empty, once per paragraph. Show removed paragraphs once under «Sale», and «Entra: nada (se borra)» once; never an empty «Entra:».»
- **S5 · C.23:** «A critical card's options read «Mantener esta decisión / La otra opción». Show each option's statement in its label («Mantener: ‹statement›» / «Cambiar a: ‹the other statement›»), as the open questions already do.»
- **S6 · C.24:** «Opening a project shows the read-only notice for a few seconds and then removes it, with a single tab. Diagnose first (lease left by a tab that closed without its keepalive release? the first claim racing the first render?) and report before fixing. No read-only notice may flash on a single tab.»
- **S7 · C.25:** «Home: at the account's project limit, the left «Nuevo proyecto» and «Probar con el caso de demo» stay active while the center ones are off; disable them with the same reason. Show the project list and «Salir» once (keep the left column; the center keeps the notice and the two buttons).»
- **S8 · C.26 (if time remains, one commit):** «open-question options as one column (statement, then contrast, consequence and risk below); «Subir / Bajar» only on the selected section of the frame; money always with the locale's decimal separator; inputs inherit the body font (U-4); a custom file button with translated text; spellcheck off on the document editor; the header's «Te toca: n cosas» and the cards' counter count the same things or say what each counts.»
- **S9 · checks:** «For each: make test, make vet, make web, govulncheck, a headless smoke at 1440 px with a screenshot, in en and es. Stop at 16:00 Lima time and report whatever is done»
- **S10 · C.24:** «apply only points 1 and 3 of your proposal: 1. The save fired on pagehide saves but does not claim the edit (it must not call editors.claim for a tab that is leaving; keep the read-only check for every other save). 3. A network error on the claim keeps the previous state instead of setEditor(false). Do not add BroadcastChannel or the take_over retry: the crashed-tab case waits until after the hackathon. Test: write, close the tab, open another: no read-only notice. Then the same smoke you measured, in the same table.»
- **S11 · B3-20:** «B3-20 is updated (now synced): the 80 % warning is computed over what the agent can spend (the project cap minus the signoff balance), so it shows at about 1.76 USD before the stop. Change it and the test that pins it.»
- **S12 · plurals:** «Small: plurals in the frame («1 párrafo», «1 pendiente», and the same in English).»
- **S13 · checks:** «One commit each, explicit paths, no trailers, no push. make test, make vet, make web, govulncheck, a headless smoke at 1440 px in en and es. Stop at 13:00 Lima time and report.»

## Tasks

| ID | Specs | Route | Status | Commit / evidence |
|---|---|---|---|---|
| T1 | S1, S2 | inline | done | `5bd3eb6`; gate exit 0; smoke bal-en, bal-es |
| T2 | S3 | inline | done | RED then GREEN `TestB99_EveryProposalOriginHasATextInBothCatalogs`; gate exit 0; smoke c21-en, c21-es |
| T3 | S4 | inline | done | RED then GREEN `C.22` in `proposals.test.ts`; gate exit 0; smoke c22-en, c22-es |
| T4 | S5 | inline | done | RED then GREEN `C.23` in `cardDrafts.test.ts`; gate exit 0; smoke c23-en, c23-es |
| T5 | S6, S10 | inline | done | RED then GREEN `TestC24_TheSaveOfATabThatLeavesSavesAndDoesNotClaimTheEdit`; gate exit 0; smoke c24-en, c24-es: wrote and closed three times, no notice, last text saved; crashed holder still shows the notice, by request |
| T6 | S7 | inline | done | RED then GREEN `C.25` in `home.test.ts`; gate exit 0; smoke c25-en, c25-es (en `Sign out` once, the script looked for «Log out») |
| T8 | S11 | inline | pending | |
| T9 | S12 | inline | pending | |
| T7 | S8 | inline | done | RED then GREEN `C.26` in `noise.test.ts`, `spend.test.ts`; gate exit 0; smoke c26-oq, c26-doc, c26-file in en and es |

## Log

- **L1 (2026-11-04):** Eber: «Review of the three fixes (P-128 in docs/construccion/supervision.md): accepted.

Your question: keep it. With critical decisions and the signoff balance spent, signing stays impossible; signing unchecked would defeat the signoff. Two changes:
- Raise the signoff reserve from about 2 contrasts to about 4 [initial] (models.go and wherever B3-20's reserve is computed). Tests that pin the reserve follow.
- When the balance is spent, the signoff screen says: «Se agotó el saldo de la firma. Tu explicación y tu trabajo quedan guardados; las secciones sin decisiones críticas sí se pueden firmar.» (en and es).

Then UI fixes from my Chrome pass, one commit each, in this order, explicit paths, no trailers, no push:

C.21 · The four B-99 proposal origins have no text: proposal.origin.removes_half, removes_support, breaks_sentence, rewrites_sentence_end show as raw keys. Add them to en.json and es.json in plain words («El cambio quita más de la mitad de la sección», «Borra un párrafo que sostiene una decisión o una cita», «Deja una oración sin cerrar», «Cambia el final de una oración»). Extend the catalog test so every Go origin constant has a text in both catalogs.

C.22 · A proposal that removes paragraphs shows «Entra:» empty, once per paragraph. Show removed paragraphs once under «Sale», and «Entra: nada (se borra)» once; never an empty «Entra:».

C.23 · A critical card's options read «Mantener esta decisión / La otra opción». Show each option's statement in its label («Mantener: ‹statement›» / «Cambiar a: ‹the other statement›»), as the open questions already do.

C.24 · Opening a project shows the read-only notice for a few seconds and then removes it, with a single tab. Diagnose first (lease left by a tab that closed without its keepalive release? the first claim racing the first render?) and report before fixing. No read-only notice may flash on a single tab.

C.25 · Home: at the account's project limit, the left «Nuevo proyecto» and «Probar con el caso de demo» stay active while the center ones are off; disable them with the same reason. Show the project list and «Salir» once (keep the left column; the center keeps the notice and the two buttons).

If time remains, C.26 (polish, one commit): open-question options as one column (statement, then contrast, consequence and risk below); «Subir / Bajar» only on the selected section of the frame; money always with the locale's decimal separator; inputs inherit the body font (U-4); a custom file button with translated text; spellcheck off on the document editor; the header's «Te toca: n cosas» and the cards' counter count the same things or say what each counts.

For each: make test, make vet, make web, govulncheck, a headless smoke at 1440 px with a screenshot, in en and es. Stop at 16:00 Lima time and report whatever is done: I need the afternoon for the Meta pass, the deploy and the video. The delivery closes today at 21:00.»
- **L2 (2026-11-04):** C.24 diagnosis: no notice on first open, reload, or a clean close; the first-claim race is ruled out. It reproduces by typing and closing at once, then opening a new tab: the save at `pagehide` calls `editors.claim` for the tab that left (order inferred from the code), so the lease stays orphaned up to 45 s; also when the holder crashes. Any network error on a claim also turns the tab read-only until the next renewal. C.25 and C.26 go on meanwhile, out of the order asked, because they do not depend on it.
- **L3 (2026-11-04):** Eber: «Review of C.21 to C.26 (P-129 in docs/construccion/supervision.md): accepted. C.24: apply only points 1 and 3 of your proposal.» Its full text is S10 to S13.
