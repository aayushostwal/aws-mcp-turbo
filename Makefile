GO ?= go
VERSION ?= dev
# Keep the scanner's Go analyzer compatible when upgrading the Go toolchain.
GOVULNCHECK_VERSION := v1.8.0

.PHONY: build test race vet bench check vuln
build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o bin/aws-mcp-turbo ./cmd/aws-mcp-turbo
test:
	$(GO) test ./...
race:
	$(GO) test -race ./...
vet:
	$(GO) vet ./...
vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...
bench:
	$(GO) test ./internal/turbo -run '^$$' -bench . -benchmem
check: test vet
	test -z "$$(gofmt -l cmd internal)"
	node --test npm/test/*.test.mjs
