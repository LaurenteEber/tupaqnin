# Fixes after accepting 1.7: provider HTTP status, P-40, ask, P-41, signoff explanation in the frame

Objective: Eber's answers and fixes after accepting 1.7. One local `fix:` commit, no trailers, never push; then **stop** (Eber tests in the browser, then runs 1.8 with Meta). Runner: `make test` · `go test -tags devroutes ./internal/...` · `make vet` · `make web`.

## Specs (verbatim)

- **S1 · B-20 (c):** «yes, keep the HTTP status in its own field of the provider error, so B-20 (c) does not depend on Meta's body»
- **S2 · P-40:** «yes, also reject agent text on a section in `changed_since_signoff`, as in `signed` (states §1: steps arrive as proposals until the new signature, and proposals are slice 4) — P-40»
- **S3 · Ask:** «keep «Ask the agent» outside `EndStep`, but record the question as `human_message` and the answer in the history, not applied.»
- **S4 · P-41:** «the signing screen must not offer «See the history of that point» while signing is open, because the history shows the decision, its contrast and the reason (spec §9: no frame during signing); show it only after signing, pausing or leaving.»
- **S5 · Frame:** «And show the current signing explanation in the frame (spec §9).»
- **S6 · Commit:** «One `fix:` commit, then stop»

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S1 | inline | done | `TestB20c_HTTPStatusIsKeptWhenTheBodyNamesItsOwnCode` (RED: build), `TestB20c_UnavailableIsTimeout5xxOr429Only` on `HTTPStatus` |
| T2 | S2 | inline | done | `TestP40_StepOnAChangedSinceSignoffSectionRejectsOnlyThatText` (RED seen) |
| T3 | S3 | inline | done | `TestB14_AskTheAgentRecordsYourQuestionAndItsAnswerNotApplied` (RED seen) |
| T4 | S4 | inline | done | `SignoffPanel` gates link and panel on `phase != in_progress`; no web test runner |
| T5 | S5 | inline | done | `TestM1_FrameShowsTheCurrentSignoffExplanation` (RED seen) |

## Log

- **L1 (2026-10-29, user, verbatim):** «1.7 and the four fixes accepted, including your choices except two. Answers: (1) yes, keep the HTTP status in its own field of the provider error, so B-20 (c) does not depend on Meta's body; (2) yes, also reject agent text on a section in `changed_since_signoff`, as in `signed` (states §1: steps arrive as proposals until the new signature, and proposals are slice 4) — P-40; (3) keep «Ask the agent» outside `EndStep`, but record the question as `human_message` and the answer in the history, not applied. Also P-41: the signing screen must not offer «See the history of that point» while signing is open, because the history shows the decision, its contrast and the reason (spec §9: no frame during signing); show it only after signing, pausing or leaving. And show the current signing explanation in the frame (spec §9). One `fix:` commit, then stop: Eber will test in the browser and then run 1.8 with Meta.»
- **L2:** «except two» = the two choices reversed by answers (2) and (3)'s history part; the `elecciones-agente.md` rows for them are updated.
- **L3 (2026-10-29):** All five done; reversed choices struck in `elecciones-agente.md`. Not checked in a browser. Stop after the commit: Eber tests in the browser, then runs 1.8 with Meta.
