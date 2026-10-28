# tupaqnin

> A harness for writing documents with an AI agent, so that what you sign is something you understand and can defend.
>
> *Un harness para escribir documentos con un agente de IA: lo que firmas, lo entiendes y lo puedes defender.*

**tupaqnin** (Quechua: *its complement*). The AI complements human judgment; it does not replace it.

## The problem

AI agents now write reports, methodologies and research faster than anyone can follow. The person ends up signing a document they cannot explain: where a figure came from, why one option was chosen over another, what the conclusion rests on. That is **cognitive debt**. Code has tests, diffs and reviews to catch it. Documents have almost nothing.

## What tupaqnin does

- **The agent works and you decide.** The agent writes, proposes and records each decision with its trade-off («A instead of B, because…»). The choices that matter come to you as short cards that ask for a reason, not a yes.
- **Nothing blocks the work; one gate protects it.** The agent never waits for you. Only signing a section requires you to explain it, and the app checks your explanation against what the text actually says.
- **The app computes, the models judge.** States, fingerprints, links between figures and decisions, and what is critical are computed by rules in Go. Models are used only where judgment is needed.
- **The agent does not depend on its memory.** Its memory is rebuilt from the project graph and its history at every step, so a restart or a model change loses nothing.

It is designed for Meta's models: Muse Spark for the agent and Muse Wonder to contrast your explanation when you sign. The provider is not implemented yet; it arrives in slice 0, and the exact model names are confirmed by the model micro-test (piece 0.8).

## Status

🛠️ **In construction** for the Meta Global AI Developer Hackathon (27 Oct – 4 Nov 2026). The design is complete (`docs/diseno/`); the code is being built in slices, starting with slice 0 (`docs/construccion/backlog.md`). What existed before the event and what was built during it is listed in [`HACKATHON.md`](HACKATHON.md).

## Run it

Requires Go 1.27.

```sh
TQ_PROVIDER=fake make run    # http://localhost:8080 — GET /healthz and GET /
make test
make vet
make build  # bin/tupaqnin
```

Configuration comes only from environment variables (see `.env.example`);
the app never reads a `.env` file itself. `META_API_KEY` is required unless
`TQ_PROVIDER=fake`; defaults are `PORT=8080` and `TQ_DATA_DIR=./data`.
The Meta and fake providers arrive in piece 0.5.

## Repository map

| Path | What it holds |
|---|---|
| `cmd/tupaqnin/` | The single binary: API and the embedded web page (streaming arrives in piece 0.6) |
| `internal/` | One package per responsibility: `store`, `graph`, `cycle`, `memory`, `provider`, `fingerprint`, `server`, `voice` |
| `web/` | For now only a README; the React + Vite editor arrives in piece 0.10. It only presents; anything with evidence value is computed in Go |
| `deploy/` | Dockerfile and Fly.io config |
| `docs/` | The design in Spanish (`docs/diseno/`) and the build backlog (`docs/construccion/`) |
| `AGENTS.md` | Rules for coding agents working in this repo |

## Design documents (Spanish)

| Document | Contents |
|---|---|
| [`docs/diseno/bloque-1/`](docs/diseno/bloque-1/) | The co-creation cycle: stages, objects and states, checks, signing, model contracts |
| [`docs/diseno/bloque-2/`](docs/diseno/bloque-2/) | The agent's memory: what it keeps, how it restarts, how contradictions are caught |
| [`docs/diseno/bloque-3/`](docs/diseno/bloque-3/) | Platform: data model, write gate, technical defaults, output schema, repo layout |
| [`docs/diseno/glosario.md`](docs/diseno/glosario.md) | Spanish design terms → English code names |
| [`docs/diseno/_plan-hackathon.md`](docs/diseno/_plan-hackathon.md) | Slices and calendar |
| [`docs/construccion/backlog.md`](docs/construccion/backlog.md) | What to build next, with what to read for each piece |

## Inspiration

tupaqnin takes patterns from [gentle-ai](https://github.com/Gentleman-Programming/gentle-ai) and Engram, which solve the same problem well for code. It does not depend on them or use their names or marks.

## License

[MIT](LICENSE)
