# Piece 1.8 · Case 5 end to end

Objective: backlog-r1 v0.17, piece 1.8. One local commit `feat: slice-1.8` on `main`, no trailers, never push; then stop for Eber's browser check and `make demo-check`. Runner: `make test` · `make vet`.

## Specs

- **S1 · Fake walkthrough:** «Una prueba de recorrido con el proveedor falso y las respuestas grabadas del recorrido real». Listo cuando: «Con el proveedor falso: el recorrido completo pasa en una prueba».
- **S2 · demo-check:** «`make demo-check`, que corre el recorrido con `TQ_PROVIDER=meta` (con la clave del llavero, como `make microtest`) y lo corre Eber». Listo cuando: «con Meta, en el navegador: el recorrido pasa sin intervención manual, y la historia tiene en orden los eventos de cada paso».
- **Si algo no está:** «Si el recorrido real no pasa, el retorno dice en qué paso y con qué salida, sin corregir el diseño».
- **No hacer:** «La parte del caso que es de las rebanadas 2 y 3».

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S1 | inline | done | `TestH1_Case5WalkthroughPassesEndToEnd` passes steps 1–6 and the 11 ordered events; recordings hand-made until demo-check |
| T2 | S2 | inline | done | `TestDemoCheck` (`-tags=meta`, skips without `TQ_PROVIDER=meta`), `make demo-check`; vetted with `-tags=meta`, never run (Eber) |

## Log

- **L1 (30-oct, Eber):** «1.8 · Case 5 end to end (backlog-r1 §1.8; docs/demo/caso-5/). The walkthrough test with the fake provider and recorded answers, and `make demo-check` for me to run with TQ_PROVIDER=meta (key from the keychain, like make microtest). You never call Meta. Commit «feat: slice-1.8».»
- **L2:** This document was written after the code, not before the first write (the ODD rule was missed for this piece). No real recordings exist; three choices in `elecciones-agente.md`.
