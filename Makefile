.PHONY: microtest-tools run test vet build microtest microtest-signoff web web-types

run:
	go run ./cmd/tupaqnin

test:
	go test ./...

vet:
	go vet ./...

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -o bin/tupaqnin ./cmd/tupaqnin

# The web app (backlog 0.10): installs dependencies and builds the
# React+Vite+TipTap frontend into internal/server/static, which the
# Go binary embeds. Run once before `make run` for the browser UI.
# pnpm with the frozen lockfile and pinned versions only (B-19).
web:
	cd web && pnpm install --frozen-lockfile && pnpm run build

# Regenerates web/src/api/types.ts from the Go API shapes with tygo
# (pinned as a go.mod tool directive, B-19).
web-types:
	cd web && go tool tygo generate

# Micro-test 0.8 against real Meta (backlog). The API key comes from
# the macOS keychain, never from a file or the repo. Eber runs this.
microtest:
	META_API_KEY="$$(security find-generic-password -s tupaqnin-meta-api -w)" go test -tags=meta ./internal/provider/microtest/... -run TestMicroTestPrincipalSchema -count=1 -v

# Micro-test of tool calls (backlog-r1 1.9, R-3): function tools in
# strict mode with the principal's strict output, against real Meta.
# Writes docs/casos-prueba/micro-prueba-herramientas/informe.md. Same
# keychain key as microtest. Eber runs this.
microtest-tools:
	META_API_KEY="$$(security find-generic-password -s tupaqnin-meta-api -w)" go test -tags=meta ./internal/provider/microtest/... -run TestMicroTestTools -count=1 -v -timeout 30m

# Real Wonder outputs for the signoff tests (backlog-r1 1.7), recorded
# into internal/provider/testdata/signoff/ only when their marks match
# each scenario. Same keychain key as microtest. Eber runs this.
microtest-signoff:
	META_API_KEY="$$(security find-generic-password -s tupaqnin-meta-api -w)" go test -tags=meta ./internal/provider/microtest/... -run TestMicroTestWonderSignoff -count=1 -v
