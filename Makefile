.DEFAULT_GOAL := check

BINARY := bin/agtk
GO_FILES := $(shell find . -name '*.go' -not -path './vendor/*')
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || echo dev)
COMMIT ?= $(shell test -z "$$(git status --porcelain --untracked-files=normal)" && git rev-parse HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)

.PHONY: build catalog catalog-check check clean fmt fmt-check test test-race vet

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/agtk

fmt:
	gofmt -w $(GO_FILES)

fmt-check:
	@test -z "$$(gofmt -l $(GO_FILES))" || \
		(echo "Run 'make fmt' to format these files:"; gofmt -l $(GO_FILES); exit 1)

vet:
	go vet ./...

catalog:
	go generate ./...

catalog-check:
	go run ./cmd/cataloggen --check

test:
	go test ./...

test-race:
	go test -race ./...

check: fmt-check catalog-check vet test-race build

clean:
	rm -rf bin dist coverage.txt
