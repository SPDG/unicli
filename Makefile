.PHONY: test vet build live

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

build:
	go build -o bin/unicli ./cmd/unicli
	go build -o bin/unicli-mcp ./cmd/unicli-mcp

live:
	go test -tags live -count=1 ./internal/live/...
