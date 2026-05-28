BINARY = mdns2hosts.exe
GOARCH = amd64
GOOS   = windows

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  = $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS = -s -w \
	-X 'main.Version=$(VERSION)' \
	-X 'main.Commit=$(COMMIT)' \
	-X 'main.BuildDate=$(DATE)'

.PHONY: all build test clean lint coverage run

all: build

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test -short ./hosts/... ./mdns/... ./service/...

test-full:
	go test ./hosts/... ./mdns/... ./service/...

coverage:
	go test ./hosts/... ./mdns/... ./service/... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run --timeout=3m

clean:
	rm -f $(BINARY) coverage.out coverage.html
