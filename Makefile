.PHONY: test vet build live

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

VERSION ?= 0.0.0-dev
LDFLAGS := -s -w -X github.com/SPDG/unicli/internal/cli.Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/unicli ./cmd/unicli
	go build -ldflags "$(LDFLAGS)" -o bin/unicli-mcp ./cmd/unicli-mcp

live:
	go test -tags live -count=1 ./internal/live/...
