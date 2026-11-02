# Slice 4 · Hard cases

Objective: backlog-r4 v0.7, pieces in the order 4.1 → 4.5 → 4.4 → 4.2 → 4.3 → 4.6, after Entrega (`odd/tasks/slice-entrega.md`). Branch: `main`, one local commit per piece `feat: slice-4.n`, explicit paths, no signature or trailer lines, never push. Runner: `go test -count=1 ./...` · `make vet` · `make web`; each commit gated on their exit codes. A piece that is not green is not committed: that is the switch. Never call Meta, never run fly.

## Specs

Each piece's spec is its row in `docs/construccion/backlog-r4.md` v0.7, read verbatim there. Decisions B-73 to B-86 in the same file, all adopted. 4.6 uses the «Tabla de frenos de la 4.6» in the same file; its 4 gaps are decided.

- **S1 · 4.1** Propuestas (B-73 to B-77).
- **S5 · 4.5** Umbral y limpieza del contexto (B-86).
- **S4 · 4.4** «Continúa hasta terminar» (B-83, B-84, B-85).
- **S2 · 4.2** Deshacer y edición sospechosa (B-78, B-79, B-80).
- **S3 · 4.3** Objetar y revocar (B-81, B-82).
- **S6 · 4.6** Los casos 7 y 9, y los tramos cortados.

## Tasks

| ID | Spec | Route | Status | Commit |
|---|---|---|---|---|
| T1 | S1 · 4.1 | delegated (context backstop); verifier: items 1, 3, 4 | done | feat: slice-4.1 |
| T5 | S5 · 4.5 | delegated (context backstop) | done | feat: slice-4.5 |
| T4 | S4 · 4.4 | delegated (context backstop); verifier: items 3, 4 | done | feat: slice-4.4 |
| T2 | S2 · 4.2 | delegated (context backstop) | pending | |
| T3 | S3 · 4.3 | delegated (context backstop) | pending | |
| T6 | S6 · 4.6 | delegated (context backstop) | pending | |

## Log

- **L1 (1-nov, Eber):** the request is quoted in full in `odd/tasks/slice-entrega.md` L1; this document is its item 2: "Slice 4: docs/construccion/backlog-r4.md v0.7, pieces in the order 4.1 → 4.5 → 4.4 → 4.2 → 4.3 → 4.6 (4.6 uses the «Tabla de frenos de la 4.6» in the same file; its 4 gaps are decided)."
- **L2 (T1, 4.1):** writer delivered the piece; parent applied its reviewed patch to `internal/demo/walkthrough`, outside its surface, where the tests pinned the rejections B-73 replaces. Verifier on the real binary: 12 of 13 «Listo cuando» passed and 25 races ended consistently; 3 blockers fixed in one correction batch, RED then GREEN: the new-step notice was lost when accepting during a running step (now the proposal's own line in «lo que cambió», via `delivery`); the web dropped `turn_id`, so the new step ran unseen; the displaced contrast had no decisions (B-74). Open for Eber: rule 2 and the checker skip accepted text, as the row declares, so the same text gets `text_without_decision` as a step but not when accepted. Advisories: discarding a displaced proposal leaves the decision's `used_in` and its `open_card`, so the card blocks signing your text; an accepted figure comes back linked to a superseded decision; after an accepted `replace_all` a second displacement has empty triage and note; `ProposalLine` and the signoff list show only the first text; closed section proposals are never shown. For 4.2: after an undo, B-79 says the displaced proposal can be accepted and the code answers `section_changed`.
- **L3 (T5, 4.5):** writer delivered the piece. No verifier: Risk none as parent read it. `ensureInstance` and `runStep` are internal, and the differential of two turns below the threshold (8 fake requests and published steps) is byte-identical to 13dd60a; the recognition of the 400 is `[inicial]` and tested, and only Eber's Meta run can check a real body. A behaviour nobody asked for, pinned by a test: an instance with zero calls now gets a `new_session` startup. Open for Eber: over the threshold with a signoff in progress, your message launches no step and stays unanswered with no notice on the page until a later turn's startup carries it; outside the mode nothing reopens the chain when the signoff closes. Not done: the retry covers steps only, not the understanding, digest or stop report; `SignoffClosed` is a no-op that waits for 4.4; the over-cap notice of bloque-2 §3 does not exist, already at base (`Brief.OverCap` is unread).
- **L4 (T4, 4.4):** writer delivered the piece. Verifier: 4 blockers, fixed in one correction batch, RED then GREEN except the web ones: activating during a reply-only step ended the mode at once (stale read in `chainTurn`); a queued message a failing step had taken was lost, also across a restart (now `undeliveredMessages`, from the history); a signoff outcome nobody read answered a later request; the page did not follow chains the app opens (now `applySummary` and `turnToFollow`). Held: at most 1 step at a time over 12 rounds with `-race`, no cap passed in what was probed, normal mode identical to the baseline except `mode` in `/summary`. Open for Eber: `mode_finished` lives in `stop_report`, `mode_set` and the stream, not in the history's `step_ended`: B-37's reason holds and `project_cap` takes that route, but the row, B-83 and the 4.6 table say `step_ended`. Advisories: after a restart the bar says «Hasta terminar» with nothing running until your next message; a step that only proposes a question ran 60 steps with a repeating fixture, no dedupe, so a real model could burn budget that way; `message.queued` stays up until the turn's end; rounds, audit and «Preguntar» still answer 409 during the mode. The page has not run in a browser.
