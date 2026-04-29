VERSION ?= dev
LDFLAGS := -X github.com/blai/clean-slate/internal/version.Version=$(VERSION)
BIN := bin/cs

.PHONY: all build test install clean completions vet fmt lint check help

all: build

## Build the cs binary into ./bin/cs
build:
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

## Run all tests
test:
	go test ./...

## Install cs to $GOPATH/bin (or $GOBIN)
install:
	go install -ldflags "$(LDFLAGS)" .

## Remove build artifacts
clean:
	rm -rf bin completions

## Generate shell completion scripts into ./completions/
completions: build
	@mkdir -p completions
	$(BIN) completion bash > completions/cs.bash
	$(BIN) completion zsh  > completions/cs.zsh
	$(BIN) completion fish > completions/cs.fish
	@echo "Completions written to completions/"

## Run go vet
vet:
	go vet ./...

## Run gofmt -l (fail if any file needs formatting)
fmt:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "Needs gofmt:\n$$out"; exit 1; fi

## Run vet + fmt + test (CI-style check)
check: vet fmt test

## Show available targets
help:
	@awk 'BEGIN{FS=":"}/^## /{doc=substr($$0,4)} /^[a-zA-Z_-]+:/{if(doc){printf "  %-14s %s\n",$$1,doc; doc=""}}' $(MAKEFILE_LIST)
