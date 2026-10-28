.PHONY: run test vet build

run:
	go run ./cmd/tupaqnin

test:
	go test ./...

vet:
	go vet ./...

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -o bin/tupaqnin ./cmd/tupaqnin
