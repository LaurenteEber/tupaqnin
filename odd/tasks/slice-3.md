# Slice 3 · Evidence

Objective: backlog-r3 v0.7, pieces 3.0 → 3.7, one at a time as Eber releases them (now: 3.0 only). One local commit per piece `feat: slice-3.N` on `main`, explicit paths, no trailers, never push. Runner: `make test` · `make vet` · `make web` (UI tests: `cd web && pnpm test`).

## Specs

Each piece's spec is its row in `docs/construccion/backlog-r3.md` v0.7, read verbatim there: «Qué», «Implementa», «Leer», «Interfaz», «Listo cuando», «No hacer», «Si algo no está». Decisions B-55 to B-72 in the same file.

- **S0 · 3.0** Micro-prueba de evidencia. «Write TestMicroTestEvidence in internal/provider/microtest/evidence_test.go (tag meta), using the reader's strict output from docs/diseno/bloque-3/salida-lector.schema.json (its extref has no URL: the app never downloads an address the model wrote, B-58). Part (3), without a model, saves the OpenAlex and arXiv responses as recordings for 3.1, without any key. The User-Agent contact comes from TQ_CONTACT_EMAIL (set on my side).» Report in `docs/casos-prueba/micro-prueba-evidencia/informe.md`, written from Eber's run of `make microtest-evidence`, in the format of `docs/casos-prueba/micro-prueba-herramientas/informe.md`.

## Tasks

| ID | Spec | Route | Status | Commit |
|---|---|---|---|---|
| T0 | S0 (test) | inline | done | feat: slice-3.0 |
| T0r | S0 (report) | inline | waiting Eber's run | — |

## Log

- **L1 (31-oct, Eber):** "Then start slice 3 with piece 3.0 only (docs/construccion/backlog-r3.md: read 3.0, B-55, B-60, B-65; follow docs/casos-prueba/micro-prueba-herramientas/informe.md as the format). Write TestMicroTestEvidence in internal/provider/microtest/evidence_test.go (tag meta), using the reader's strict output from docs/diseno/bloque-3/salida-lector.schema.json (its extref has no URL: the app never downloads an address the model wrote, B-58). Part (3), without a model, saves the OpenAlex and arXiv responses as recordings for 3.1, without any key. The User-Agent contact comes from TQ_CONTACT_EMAIL (set on my side). I approve golang.org/x/net/html (B-60) as a direct dependency when 3.1 needs it. One commit «feat: slice-3.0», explicit paths only, no signature lines, no push. make test, make vet. Then stop: I run make microtest-evidence and you write the report from its output."
- **L2 (T0):** `evidence_test.go`: (1) 5 reader calls with `{"type":"web_search"}` + strict `salida-lector` (Spark 1.3 low, B-65); (2) 5 runs with strict `search_literature` / `read_hit` over live OpenAlex and arXiv, up to 8 tool calls (B-65), hits numbered `hit-n`; `read_hit` returns the API's abstract (full text is 3.1); (3) 3 OpenAlex + 2 arXiv queries (case 5 minimum particle size, B-71; «pobreza y desarrollo cerebral») saved to `internal/evidence/testdata/recordings/` without key or contact. Own HTTP client per B-60 (5 s, 5 MB after decompression, ≤3 redirects, arXiv every 3 s, contact in the User-Agent). Everything is logged with -v for the report; nothing from Meta, OpenAlex or arXiv fails the test. The test stops if the repo's `salida-lector.schema.json` still has `extref.url` (the repo copy predates Eber's design copy). `tools_test.go`: the streaming call is now `postResponses`, shared, and keeps the raw `usage`. Pre-existing failure, not from this piece: `TestSchemaCompilesWithECMARegex` (-tags=meta) misses `stretch` in its conforming example.
