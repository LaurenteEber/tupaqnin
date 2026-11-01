.PHONY: microtest-tools run test vet build microtest microtest-signoff web web-types demo-check microtest-checker microtest-judge-defense microtest-evidence microtest-digest microtest-judge-verify microtest-checker-exceeds

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
# The UI tests (pnpm test: node --test, P-92..P-94) run first.
# pnpm with the frozen lockfile and pinned versions only (B-19).
web:
	cd web && pnpm install --frozen-lockfile && pnpm test && pnpm run build

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

# The case 5 walkthrough end to end against real Meta (backlog-r1 1.8):
# steps 1 to 6 of the jury's walkthrough through the HTTP API, with
# the scripted acts of the jury. A failing run names the step and the
# model's last output; a passing run replaces the recorded answers in
# internal/demo/walkthrough/testdata/recorded/, which the default test
# replays with the fake. Same keychain key as microtest. Eber runs this.
demo-check:
	TQ_PROVIDER=meta META_API_KEY="$$(security find-generic-password -s tupaqnin-meta-api -w)" go test -tags=meta ./internal/demo/walkthrough/ -run TestDemoCheck -count=1 -v -timeout 30m

# Real checker outputs for the checker tests (backlog-r2 2.3, P-68),
# recorded into internal/provider/testdata/checker/ only when the
# judgment matches each scenario. Same keychain key as microtest. Eber runs this.
microtest-checker:
	META_API_KEY="$$(security find-generic-password -s tupaqnin-meta-api -w)" go test -tags=meta ./internal/provider/microtest/... -run TestMicroTestChecker -count=1 -v

# A real exceeds_reviewed (backlog-r3 3.5, B-68), recorded into
# internal/provider/testdata/checker/exceeds-reviewed.json only when the
# checker marks it from the inventory; a hand-made one stands in meanwhile.
# Same keychain key as microtest. Eber runs this.
microtest-checker-exceeds:
	META_API_KEY="$$(security find-generic-password -s tupaqnin-meta-api -w)" go test -tags=meta ./internal/provider/microtest/... -run TestMicroTestCheckerExceeds -count=1 -v

# Real judge outputs for the defense tests (backlog-r2 2.6).
microtest-judge-defense:
	META_API_KEY="$$(security find-generic-password -s tupaqnin-meta-api -w)" go test -tags=meta ./internal/provider/microtest/... -run TestMicroTestJudgeDefense -count=1 -v

# Evidence micro-test (backlog-r3 3.0): web_search and own tools with the
# reader strict output, plus OpenAlex and arXiv recordings. Eber runs this.
microtest-evidence:
	META_API_KEY="$$(security find-generic-password -s tupaqnin-meta-api -w)" go test -tags=meta ./internal/provider/microtest/... -run TestMicroTestEvidence -count=1 -v -timeout 30m

# Real digest outputs for the digest tests (backlog-r3 3.4), recorded
# into internal/provider/testdata/digest/ only when each scenario's
# digest is what its test needs. Same keychain key as microtest. Eber
# runs this.
microtest-digest:
	META_API_KEY="$$(security find-generic-password -s tupaqnin-meta-api -w)" go test -tags=meta ./internal/provider/microtest/... -run TestMicroTestDigest -count=1 -v

# Real judge outputs for the verification tests (backlog-r3 3.6), recorded
# into internal/provider/testdata/judge-verify/ only when each scenario's
# output is what its test needs. Same keychain key as microtest. Eber runs
# this.
microtest-judge-verify:
	META_API_KEY="$$(security find-generic-password -s tupaqnin-meta-api -w)" go test -tags=meta ./internal/provider/microtest/... -run TestMicroTestJudgeVerify -count=1 -v
