VERSION ?= dev
LDFLAGS := -X github.com/blai/clean-slate/internal/version.Version=$(VERSION)
BIN := bin/cs

# Install location: $GOBIN if set, else $GOPATH/bin, else $HOME/go/bin.
# `go install` picks this automatically, but we need it explicitly to rename
# the binary (module is "clean-slate", binary should be "cs").
INSTALL_DIR := $(shell go env GOBIN)
ifeq ($(INSTALL_DIR),)
INSTALL_DIR := $(shell go env GOPATH)/bin
endif

.PHONY: all build test install clean completions vet fmt check help

all: build

## Build the cs binary into ./bin/cs
build:
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

## Run all tests
test:
	go test ./...

## Install cs as 'cs' into $GOBIN (or $GOPATH/bin).
## go install would name it after the module (clean-slate), so we build + install explicitly.
install: build
	@mkdir -p $(INSTALL_DIR)
	install -m 0755 $(BIN) $(INSTALL_DIR)/cs
	@echo "Installed cs to $(INSTALL_DIR)/cs"

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
