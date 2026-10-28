# AGENTS.md · tupaqnin

Instructions for coding agents working in this repository. Read this file in full before your first change.

## What tupaqnin is

A web app where a person and an AI agent write a document together, and the person ends up able to sign it and defend it. The agent works and the human decides; nothing blocks the agent, and a single gate protects the document; the app computes and the models judge; the agent does not depend on its own memory.

## Where the design is

The design is the source of truth, written in Spanish. Code names come from the glossary.

| You need | Read |
|---|---|
| What to build next, and what to read for it | `docs/construccion/backlog.md` |
| Data model, write gate, signing, deploy, stack | `docs/diseno/bloque-3/bloque-3-especificacion.md` |
| Technical defaults (ids, anchors, criticality, dates, tests…) | `docs/diseno/bloque-3/defaults-tecnicos.md` |
| The principal's fixed output | `docs/diseno/bloque-3/salida-principal.schema.json` |
| Spanish design term → English code name | `docs/diseno/glosario.md` |
| Repo layout and the first commit | `docs/diseno/bloque-3/repo-estructura.md` |
| The cycle, objects, states and model contracts | `docs/diseno/bloque-1/` (`bloque-1-fichas.md`, `bloque-1-estados.md`) |
| The principal's memory | `docs/diseno/bloque-2/bloque-2.md` |

Do not use `docs/investigacion/` as design: it is background research.

## Rules

1. **One backlog item at a time.** Do only what the item says. Respect its «No hacer» field. Do not start an item whose dependencies are not merged.
2. **Never invent design.** If something you need is not in the files the item lists, or in the glossary, write `[EL DISEÑO NO LO DICE]` in your report, say where, and either stop or use the smallest provisional choice and list it. Do not add tables, states, fields, events, routes or config options on your own.
3. **Names.** Code, comments, data, API, logs and prompts are in English, using the glossary names (`snake_case` in the database and JSON, `CamelCase` in Go). `docs/` stays in Spanish. Every on-screen string comes from `web/src/i18n/en.json` and `es.json`.
4. **Do not edit `docs/diseno/`.** Design changes are Eber's. If the design looks wrong, report it with the exact quote.
5. **Go computes, React presents.** Anything with evidence value (states, fingerprints, checks, criticality, labels) is computed in Go.
6. **History only grows.** No function edits or deletes events. Every write and its event go in the same transaction, conditioned on the version that was read.
7. **The model never sees internal ids.** Ids are translated to short labels (`dec-12`, `sec-3`) in one place only: `internal/graph`.
8. **Text the app compares is never translated:** anchors, passages, figures, and anything the human wrote.
9. **Secrets and content stay out of logs.** Never log document text, messages, the API key or raw provider error bodies. The stream never forwards raw provider errors.
10. **Tests.** Every rule the app enforces in Go has at least one test with its design code in the name (`TestA1_OpenChangeBlocksSignoff`). Use a temporary SQLite database per test and the fake provider by default. Tests against Meta run only with `-tags=meta` and `META_API_KEY`.
11. **Dependencies.** Only those named in the spec (§7) or in the backlog. Anything else: ask first.
12. **Do not commit or push.** Eber reviews, commits and pushes.

## Commands

`make run` · `make test` · `make vet` · `make build`. Configuration comes only from environment variables (see `.env.example`); never commit `.env`. `TQ_PROVIDER=fake` runs the app with the fake provider and no API key.

## Report at the end of each item

1. What it does, in 5 lines Eber can repeat without looking at the code.
2. Files created or changed.
3. Tests: name, rule tested, and the output of `make test` and `make vet`.
4. What you did not do from the item, and why.
5. Every `[EL DISEÑO NO LO DICE]`, where it was, and what provisional choice you made, if any.
6. At most 3 questions for Eber, each with its trade-off («A instead of B, because…; I would pick B if…»).
