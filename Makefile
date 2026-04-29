VERSION ?= dev
LDFLAGS := -X github.com/blai/clean-slate/internal/version.Version=$(VERSION)
BIN := bin/cs

# Install location resolution order (first match wins, explicit intent beats heuristics):
#   1. $(PREFIX)/bin         — standard Unix override (e.g. PREFIX=/usr/local)
#   2. $(go env GOBIN)       — explicit Go-toolchain install dir
#   3. ~/.local/bin          — XDG user-local default (created via mkdir -p if missing)
#   4. $(go env GOPATH)/bin  — Go-toolchain fallback
#
# We resolve this explicitly (rather than using `go install`) so we can rename
# the binary at install time: the module is "clean-slate" but the binary is "cs".
GOBIN := $(shell go env GOBIN)
ifdef PREFIX
INSTALL_DIR := $(PREFIX)/bin
else ifneq ($(GOBIN),)
INSTALL_DIR := $(GOBIN)
else ifneq ($(wildcard $(HOME)/.local/bin),)
INSTALL_DIR := $(HOME)/.local/bin
else
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

## Install cs into $(INSTALL_DIR) (defaults to ~/.local/bin; override with PREFIX=...).
install: build
	@mkdir -p $(INSTALL_DIR)
	install -m 0755 $(BIN) $(INSTALL_DIR)/cs
	@echo "Installed cs to $(INSTALL_DIR)/cs"
	@case ":$$PATH:" in *":$(INSTALL_DIR):"*) ;; *) echo "Warning: $(INSTALL_DIR) is not on \$$PATH. Add it to your shell profile to run 'cs' directly." ;; esac

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
