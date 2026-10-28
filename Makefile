.PHONY: run test vet build microtest

run:
	go run ./cmd/tupaqnin

test:
	go test ./...

vet:
	go vet ./...

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -o bin/tupaqnin ./cmd/tupaqnin

# Micro-test 0.8 against real Meta (backlog). The API key comes from
# the macOS keychain, never from a file or the repo. Eber runs this.
microtest:
	META_API_KEY="$$(security find-generic-password -s tupaqnin-meta-api -w)" go test -tags=meta ./internal/provider/microtest/... -run TestMicroTestPrincipalSchema -count=1 -v
