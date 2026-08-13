.PHONY: test vet build live

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

build:
	go build -o bin/unicli ./cmd/unicli

live:
	go test -tags live -count=1 ./internal/live/...
