# Piece 1.9b · Sections by hand

Objective: backlog-r1 v0.17, piece 1.9b (K-17 precised 30-oct, P-48, P-49). One local commit `feat: slice-1.9b` on `main`, no trailers, never push; then piece 1.8. Runner: `make test` · `make vet` · `make web`.

## Specs

- **S1 · Create:** «`POST /sections` `{title, role, focus}` reemplaza la ruta de la 0.10 (que fija `other` y `proposed`, sin título): título obligatorio (400 `title_required`); con la marca «aquí se concluye», el rol nace `conclusion` `confirmed`; sin ella, no se fija y lo propone el principal; `focus` hasta 60 palabras `[inicial]` (400 `focus_too_long`); nace en `draft`, vacía y al final». Listo cuando: «Prueba: sin título responde 400 · prueba: con la marca queda `conclusion` `confirmed`; sin ella, el siguiente paso puede proponer el rol · prueba: una sección con `focus` y sin texto queda `draft` (B-21)».
- **S2 · Details and order:** «`PUT /sections/{label}/details` `{title?, focus?}` y `PUT /sections/order` `{labels[]}`, con `section_details_changed` (al reordenar, uno por sección que cambia de posición); no cambian el estado y quedan fuera de la huella y de lo firmado». Listo cuando: «prueba: cambiar el título, el `focus` o el orden de una sección firmada la deja `signed` y registra `section_details_changed`».
- **S3 · Position:** «`position` ordena el esqueleto, la columna derecha y el arranque; una `new-n` del principal va al final». Listo cuando: «prueba: reordenar cambia el orden de `/sections` y del marco».
- **S4 · Focus in the task:** «`focus` va en la tarea en turno cuando la tarea es esa sección (bloque-2 §3)». Listo cuando: «prueba: un arranque cuya tarea es esa sección trae su `focus` en literal».
- **S5 · Web:** ««+ Sección» pide título, «aquí se concluye» y «qué debe abordar»; el título y «qué debe abordar» se editan con un clic en la columna derecha; «subir» y «bajar» en el esqueleto».
- **S6 · P-48:** «el selector de proyectos del marco dice «Tus proyectos ▾» y el título del proyecto queda solo en la cabecera».
- **No hacer:** «Quitar secciones (fuera del hackathon, K-17). Arrastrar para reordenar. Que el principal reordene».

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S1 S2 S3 (Go: routes, ordering, signoff out of details) | inline | done | cycle `TestK17_*` and `TestB21_SectionWithFocusAndNoTextStaysDraft` (RED: compile), `TestK17_RenamingDuringTheSignoffStillSigns` (RED on the old `Sign`); server `TestK17_CreateSectionWithoutTitleAnswers400`, `TestK17_ReorderChangesTheOrderOfSectionsAndTheFrame` (written after the code); old 0.10 API test now sends a title |
| T2 | S4 (memory task) | inline | done | `TestK17_TaskOnASectionCarriesItsFocusLiteral` (RED seen) |
| T3 | S5 S6 (web) | inline | done | `make web-types`, `tsc --noEmit` and `make web` green; not run in a browser (Eber) |

## Log

- **L1 (30-oct, Eber):** «Finish slice 1: piece 1.9b, then 1.8. […] 1.9b · Sections by hand (docs/construccion/backlog-r1.md §1.9b; K-17 in docs/diseno/bloque-1/bloque-1-decisiones.md, precised 30-oct; P-48). «+ Section» asks for a title (required), the «aquí se concluye» mark (optional; unmarked = role not set, the principal proposes it) and «qué debe abordar» (optional, ≤ 60 words). It replaces the 0.10 button. The field is `focus` (not `brief`, which already means «encargo»). It goes in the principal's task when the task is that section (bloque-2 §3), never into the section text: a section with focus and no text stays draft (B-21). Edit title and focus of any section with one click; reorder with up/down in the frame skeleton. One `section_details_changed` event per section that changes. None of this changes the state; title and focus stay out of the fingerprint and the signoff. P-48: the frame's project selector reads «Tus proyectos ▾»; the project title shows only in the header. No new dependencies. make test, make vet, make web. Commit «feat: slice-1.9b».»
- **L2:** Map: section title/focus/position live in the version body, so a details change bumps the version. `Gate.Sign` refuses on a version change (`ErrSignoffOutdated`); it must compare the signed text instead, so a rename during signoff does not void it. B-11 (agent step on a changed section → one retry) stays as is. Sections were ordered by label (`sec-10` before `sec-2`).
- **L3:** T1–T3 done. `make test`, `make vet`, `make web` green. Seven choices appended to `elecciones-agente.md`. B-11 left as is: a rename or reorder while the agent works costs that step one retry.
