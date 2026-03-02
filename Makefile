BINARY := k6-ec2
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test lint clean install

build:
        go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/k6-ec2

test:
        go test ./... -v -race

lint:
        golangci-lint run ./...

clean:
        rm -rf bin/ dist/

install:
        go install $(LDFLAGS) ./cmd/k6-ec2

.DEFAULT_GOAL := build
