# tupaqnin

*El agente trabaja y el humano decide.* — The agent works, the human decides.

## The problem

Long documents built with AI agents accumulate cognitive debt: decisions are
buried in text, nobody knows what changed or why, and signing off on a section
means trusting memory instead of evidence.

## What tupaqnin does

- The agent works and the human decides; nothing blocks, and a single write
  gate protects the document.
- The app computes (states, fingerprints, checks) and the models judge.
- The agent does not depend on its own memory: its working state is rebuilt
  from the project's history at every step.
- Every change records who made it, on which version, and why.

## Status

Base skeleton (slice 0 in progress). The server compiles, serves `/healthz`
and a minimal embedded page, and has tests. The document cycle, store,
write gate, fingerprint and signing arrive in the following slices.
Built for a hackathon running 27 Oct – 4 Nov; see [HACKATHON.md](HACKATHON.md).

## Run it

```sh
cp .env.example .env   # fill in META_API_KEY if needed
make run               # http://localhost:8080
make test
make vet
make build             # bin/tupaqnin
```

## Repository map

| Path | What it is |
|---|---|
| `cmd/tupaqnin/` | Single binary: API + embedded page |
| `internal/store/` | SQLite: schema, version-conditioned writes, history (spec §1–§2) |
| `internal/graph/` | Graph queries: framework view, memory, criticality, probes |
| `internal/cycle/` | End of step, write gate, checks, states, signing (block 1) |
| `internal/memory/` | Working state: per-task bootstrap, caps, threshold (block 2) |
| `internal/provider/` | Meta Responses API client + fake provider (spec §7) |
| `internal/fingerprint/` | `tq-md/1` fingerprint (spec §4) |
| `internal/server/` | HTTP, streaming, access; embedded page (spec §6) |
| `web/` | React + Vite frontend (slice 1) |
| `deploy/` | Dockerfile and `fly.toml` (not deployed yet) |
| `docs/` | Design: blocks 1–3, decisions, cases (in Spanish) |

## Design documents

| Document | Contents |
|---|---|
| `docs/bloque-1/` | The cycle: states, cards, flows, decisions |
| `docs/bloque-2/` | The agent's working state and handover |
| `docs/bloque-3/` | Platform: data model, write gate, fingerprint, signing, deployment |

Code and comments are in English; `docs/` is in Spanish.

## Inspiration

Inspired by gentle-ai and Engram, without depending on them or using their marks.

## License

MIT — see [LICENSE](LICENSE).
