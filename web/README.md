# web

React + Vite frontend (the minimal editor arrived in slice 0,
backlog 0.10). Editor: TipTap 3, in plain-text mode so the Markdown
the app compares stays literal.

Types are generated from Go with `tygo` (`make web-types`); the API
shapes live in `internal/server/api.go`.

The frontend only presents: anything with evidence value
(states, fingerprints, checks) is computed by Go. Every on-screen
string comes from `src/i18n/en.json` and `src/i18n/es.json`.

Build with `make web` (from the repo root): it builds into
`internal/server/static`, which the Go binary embeds. For
development, `npm run dev` in this folder proxies `/api` to
`localhost:8080`.
