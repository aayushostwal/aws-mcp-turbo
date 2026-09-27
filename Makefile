GO ?= go
VERSION ?= dev

.PHONY: build test race vet bench check
build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o bin/aws-mcp-turbo ./cmd/aws-mcp-turbo
test:
	$(GO) test ./...
race:
	$(GO) test -race ./...
vet:
	$(GO) vet ./...
bench:
	$(GO) test ./internal/turbo -run '^$$' -bench . -benchmem
check: test vet
	test -z "$$(gofmt -l cmd internal)"
	node --test npm/test/*.test.mjs
