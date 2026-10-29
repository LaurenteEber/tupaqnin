# Fixes B-21, P-37, P-38, P-39 (after 1.7, before the review stop)

Objective: Eber's fixes after his browser test of 1.5b. One local commit `fix: B-21, P-37, P-38, P-39`, after `feat: slice-1.7`; no trailers, never push. Runner: `make test` · `make vet` · `make web`.

## Specs

- **S1 · B-21 (adopted, verbatim from `docs/construccion/backlog-r1.md`):** «**Una sección sin texto queda en «borrador»; no aparece como «lista» ni para firmar.** Pasa a «lista» solo cuando tiene texto y ningún pendiente. […] ≈ 15 min, en `SectionStatus` con su prueba». User: «a section with no text stays draft, never ready, and is not offered for signing; change it in SectionStatus, with a test.»
- **S2 · P-39:** «the empty start mentions the demo case and a blank project but has no buttons for them; put both buttons in the center of the empty start.»
- **S3 · P-37:** «after sending, the request becomes a read-only message of yours (as in the mockup), no longer an editable form.»
- **S4 · P-38:** «in the history, show local time (T-7) and words, not «dec-1» or «v1».» T-7: «Fechas en UTC […] en la base; se muestran en la zona horaria de tu navegador.»
- **S5 · Commit:** «One fix: commit for these four.»

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S1 | inline | done | `TestB21_SectionWithoutTextStaysDraft` (RED seen), `TestB21_AskingToSignAnEmptySectionIsRefused`; frame «Te toca» uses the same `SectionStatus` |
| T2 | S2 | inline | done | demo and blank buttons in `.empty` of the conversation column; `make web` |
| T3 | S3 | inline | done | `TestP37_SubmittedRequestIsMarkedSubmitted`; `RequestPanel` renders `.msg.me` read-only |
| T4 | S4 | inline | done | `HistoryPanel` local time + catalog phrases; no web test runner |

## Log

- **L1 (2026-10-29, user, verbatim):** «Eber’s browser test of 1.5b passed: frame, card from «Yours to do», history on paragraph click. P-34 and P-36 closed. After 1.7, before stopping for review, also do: (1) B-21 (adopted, see docs/construccion/backlog-r1.md): a section with no text stays draft, never ready, and is not offered for signing; change it in SectionStatus, with a test. (2) P-39: the empty start mentions the demo case and a blank project but has no buttons for them; put both buttons in the center of the empty start. (3) P-37: after sending, the request becomes a read-only message of yours (as in the mockup), no longer an editable form. (4) P-38: in the history, show local time (T-7) and words, not «dec-1» or «v1». One fix: commit for these four.»
- **L2:** Waiting for the 1.7 writer: all four touch its surfaces.
- **L3 (2026-10-29):** 1.7 committed `100d0ab` (assess medium, under_budget). Four fixes done inline; also `gofmt` on `internal/server/cards.go` (left unformatted in 1.4). Not checked in a browser.
