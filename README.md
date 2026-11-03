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

It runs on Meta's models: Muse Spark for the agent and Muse Wonder to contrast your explanation when you sign.

## What the jury sees

Open the invitation link and press «Try with the demo case». It takes about 10 to 15 minutes.

1. **The request.** Paula's state of the art on microplastics, already filled in, with her criterion and her sources. Send it.
2. **What the agent understood.** How it will apply «field studies», with a case that counts and one that does not. Confirm or correct it; the agent does not wait for you.
3. **The agent at work.** Text arrives live, and the comparison rule (1.5 times, not 60) appears as a critical decision with its card.
4. **Your decision.** Answer the card with an option and a reason; «ok» does not count. Click a paragraph to see its history.
5. **Evidence.** A citation that does not exist in the sources is discarded in plain sight.
6. **The signoff.** Press «Sign section» and explain it without looking. Muse Wonder names what is missing without giving the answer and brings a new case. Resolve it and sign.

The screen works in English and Spanish.

## Run it locally

Requires Go 1.27. The web page is already built and embedded in the binary.

```sh
export TQ_PROVIDER=fake                 # no API key, recorded answers, no spend
go run ./cmd/tupaqnin                   # http://localhost:8080
go run ./cmd/tupaqnin invite create --label local   # prints /i/<token>, once
```

Run the second command from another terminal, with the same environment, and open `http://localhost:8080/i/<token>`. Creating a project needs a link.

With real models, set `TQ_PROVIDER=meta` and `META_API_KEY`. Configuration comes only from environment variables (see `.env.example`); the app never reads a `.env` file itself.

```sh
make test    # Go tests, with the fake provider
make vet
make web     # installs, tests and builds the page (Node 24 and pnpm)
make build   # bin/tupaqnin
```

## Repository map

| Path | What it holds |
|---|---|
| `cmd/tupaqnin/` | The single binary: API, embedded page and the `invite` command |
| `internal/` | One package per responsibility: `store`, `graph`, `cycle`, `checks`, `memory`, `evidence`, `provider`, `fingerprint`, `server` and others |
| `web/` | The React page. It only presents; anything with evidence value is computed in Go |
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

Built for the Meta Global AI Developer Hackathon. What existed before the event and what was built during it is in [`HACKATHON.md`](HACKATHON.md).

## Inspiration

tupaqnin takes patterns from [gentle-ai](https://github.com/Gentleman-Programming/gentle-ai) and Engram, which solve the same problem well for code. It does not depend on them or use their names or marks.

## License

[MIT](LICENSE)
