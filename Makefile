.PHONY: build test dev fmt

build:
	go build -o bin/btcdwatchd ./cmd/btcdwatchd

test:
	go test ./...

dev:
	./scripts/dev.sh

fmt:
	gofmt -w ./cmd ./internal
