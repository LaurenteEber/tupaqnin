---
titulo: Bloque 3 · Estructura del repo público y primer commit
estado: v1.1 · 28-oct · Go 1.27 (Eber usa go1.27.3) · v1 · 28-oct · especificación de B3-10 a B3-13 (adoptadas); va a implementación
relacion: bloque-3-decisiones.md (B3-10 a B3-13) · bloque-3-especificacion.md §6 y §7 · bloque-3.md
---

# Estructura del repo público y primer commit

**Respuesta:** un repo nuevo, `LaurenteEber/tupaqnin`, que empieza con un solo commit y sin historia anterior. Lleva el diseño en `docs/` y un esqueleto de la app que compila y se prueba, todavía sin lógica del ciclo. Lo hecho durante el hackathon se identifica en `HACKATHON.md`.

**Reglas del hackathon que aplica:** «Repositories must be public» · «you must clearly identify what was newly developed during the Hackathon» (periodo: 27-oct a 4-nov).

## 1. Árbol

```
tupaqnin/
├── README.md            qué es, estado, cómo correrlo, mapa del repo (inglés y español)
├── HACKATHON.md         qué existía antes del 27-oct y qué se hizo durante el evento
├── LICENSE              MIT
├── Makefile             run · test · vet · build
├── go.mod               github.com/LaurenteEber/tupaqnin · Go 1.27 · sin dependencias
├── .env.example         META_API_KEY · PORT · TQ_DATA_DIR
├── .gitignore
├── .github/workflows/ci.yml
├── cmd/tupaqnin/        main.go: un binario que sirve la API y la página
├── internal/
│   ├── store/           SQLite: esquema, escritura condicionada, historia      → especificación §1–§2
│   ├── graph/           consultas del grafo: marco, memoria, criticidad, sondas → fichas §9 · bloque 2
│   ├── cycle/           fin de paso, comprobaciones, estados, firma             → bloque 1
│   ├── memory/          arranque por tarea, tope, umbral                        → bloque 2
│   ├── provider/        cliente de la Responses API de Meta + uno falso         → especificación §7
│   ├── fingerprint/     huella tq-md/1                                          → especificación §4
│   ├── server/          HTTP, streaming, acceso; página embebida               → especificación §6
│   └── voice/           voz: transcripción con Meta, sin guardar el audio      → B3-14 (solo si sobra tiempo)
├── web/                 React + Vite (el editor mínimo llega en la rebanada 0, según el plan)
├── deploy/              Dockerfile · fly.toml
└── docs/                el diseño actual (investigación, decisiones, casos, bloques 1–3, pruebas)
```

**Nombres:** el código y sus comentarios, en inglés; `docs/`, en español (B3-12).

## 2. Qué lleva cada pieza en el primer commit

| Pieza | Contenido | Listo cuando… |
|---|---|---|
| `go.mod` | Módulo `github.com/LaurenteEber/tupaqnin`, `go 1.27`, sin dependencias ni línea `toolchain` | `go build ./...` pasa con go1.27.3 |
| `cmd/tupaqnin/main.go` | Lee `PORT` (por defecto 8080) y sirve el handler de `internal/server`, con `ReadHeaderTimeout` | `make run` levanta el servidor |
| `internal/server/` | Ruta `GET /healthz` → `{"status":"ok"}`; `GET /` → una página mínima embebida (`static/index.html`) que dice qué es tupaqnin y que está en construcción | Las dos rutas tienen prueba |
| Los otros 7 paquetes de `internal/` (incluido `voice/`) | Solo un `doc.go`: qué hace el paquete y qué parte del diseño implementa (la columna «→» del árbol) | `go vet ./...` pasa |
| `web/` | Solo `README.md`: React + Vite, TipTap, tipos con `tygo`; «solo presenta, lo que tiene valor de evidencia lo calcula Go». El editor mínimo entra después, en la rebanada 0 (`../_plan-hackathon.md`) | — |
| `deploy/` | `Dockerfile` en dos etapas (compila en `golang:1.27` con `CGO_ENABLED=0`; imagen *distroless*), y `fly.toml` con disco en `/data`, máquina siempre encendida y chequeo a `/healthz`. **No se despliega todavía** (B3-17 y H-2 del plan) | `docker build -f deploy/Dockerfile .` construye |
| `Makefile` | `run`, `test`, `vet`, `build` (a `bin/`) | `make test` pasa |
| `.github/workflows/ci.yml` | En cada push a `main` y en cada PR: `go vet ./...` y `go test ./...`, con la versión de Go que toma `actions/setup-go` de `go.mod` (`go-version-file: go.mod`) | El primer push queda en verde |
| `.env.example` | `META_API_KEY=` · `PORT=8080` · `TQ_DATA_DIR=./data`, con la nota «nunca subir `.env`» | — |

**Voz preparada (B3-14):** `internal/voice/` existe desde el primer commit, con su `doc.go` y una interfaz `Transcriber` (de audio a texto) que todavía no tiene implementación. En `web/`, la caja donde escribes al agente y la de la explicación de firma son un mismo componente de entrada, que hoy solo acepta texto. Sumar la voz es implementar esa interfaz y agregar el botón de micrófono a ese componente, sin tocar el ciclo: lo que llega al ciclo es siempre texto.

**Fuera del primer commit:** la lógica del ciclo, SQLite (`modernc.org/sqlite`), el cliente de Meta y `web/` con código. Entran en la rebanada 0 y siguientes (`../_plan-hackathon.md`).

## 3. `README.md` y `HACKATHON.md`

- **`README.md`**, en inglés con el lema también en español: el problema (deuda cognitiva en documentos) · qué hace tupaqnin en 4 líneas (el agente trabaja y el humano decide; nada bloquea y una sola puerta protege; la app calcula y los modelos juzgan; el agente no depende de su memoria) · estado y enlace al hackathon · cómo correrlo · tabla del árbol · tabla de documentos de diseño (bloques 1–3, plan, reglas, decisiones, investigación) · inspiración (gentle-ai y Engram, sin depender de ellos ni usar sus marcas) · licencia.
- **`HACKATHON.md`**, en inglés:
  - **Antes del evento (7 al 26-oct):** solo investigación y diseño, sin código: `docs/investigacion/`, `docs/decisiones/`, `docs/casos/` y los primeros borradores de `docs/diseno/`. Cada archivo de diseño trae su versión y fecha en el encabezado.
  - **Durante el evento (desde el 27-oct):** el diseño cerrado en el evento (bloque 1 validado, bloque 2 rehecho con sus pruebas en `docs/casos-prueba/relevo-b2-*`, bloque 3 alineado) y **todo el código**, desde el 28-oct. Desde el primer commit, cada commit es trabajo del evento.
  - **No menciona el repo ni la historia anteriores.**

## 4. Qué no se publica

Al `.gitignore` se suman (sin borrar nada del disco):

```
/.obsidian/
/.tmp-relevo/
/Event Details.md
/data/
*.db
*.db-wal
*.db-shm
/bin/
node_modules/
/web/dist/
```

Ya estaban: `/.pi/`, `.env`, `/docs/casos/caso-01-gee/`, `/_por_eliminar/`. **Revisar antes del commit:** `docs/investigacion/11-hackathon-meta.md` habla de tu elegibilidad y de Ceplan.

## 5. Cómo se publica (lo hace Eber)

El repo nuevo empieza sin historia: se crea una rama huérfana con el árbol actual y se sube como `main`. La historia local queda solo en tu Mac, en otra rama, y nunca se sube.

```sh
git branch -m main historia-previa
git checkout --orphan main
git rm -r --cached --quiet .obsidian .pi .tmp-relevo "Event Details.md"
git add -A
git status            # revisar: no deben aparecer caso-01-gee ni .env
git commit -m "tupaqnin: diseño y esqueleto de la app"
git push -u origin main
```

## 6. Criterios de aceptación

1. `make test` y `make vet` pasan en local; el CI del primer push queda en verde.
2. `make run` responde `/healthz` con `{"status":"ok"}` y `/` con la página.
3. `git log` del repo público tiene un solo commit.
4. No se publican `.obsidian/`, `.pi/`, `.tmp-relevo/`, `Event Details.md`, `docs/casos/caso-01-gee/` ni `.env`.
5. `HACKATHON.md` separa lo previo de lo hecho en el evento sin mencionar el repo anterior.
