VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown)
LDFLAGS := -X github.com/user/gophkeeper/internal/version.Version=$(VERSION) \
           -X github.com/user/gophkeeper/internal/version.Commit=$(COMMIT) \
           -X github.com/user/gophkeeper/internal/version.Date=$(DATE)
COVER_PKGS := $(shell go list ./... | grep -v '/pkg/gen$$' | grep -v '/cmd/')

.PHONY: build-server build-client build-all test test-race coverage lint proto clean

build-server:
	go build -ldflags "$(LDFLAGS)" -o bin/server ./cmd/server

build-client:
	go build -ldflags "$(LDFLAGS)" -o bin/gophkeeper ./cmd/client

build-all: build-server build-client

test:
	go test -count=1 ./...

test-race:
	CGO_ENABLED=1 go test -race -count=1 ./...

coverage:
	go test -coverprofile=coverage_app.out $(COVER_PKGS)
	go tool cover -func=coverage_app.out

lint:
	go run ./cmd/staticlint ./...

proto:
	buf generate

clean:
	rm -rf bin/ coverage.out coverage_app.out
