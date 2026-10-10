GO             ?= go
GOFMT          ?= gofmt
GOLANGCI_LINT  ?= golangci-lint
GOOS           ?= $(shell $(GO) env GOOS)
GOARCH         ?= $(shell $(GO) env GOARCH)
GO_FILES       := $(sort $(wildcard *.go))
LINT_CONFIG    := $(abspath .golangci.yml)

.PHONY: help tidy format format-check vet lint compile test check

.DEFAULT_GOAL  := help
help:
	@printf '%s\n' \
		'Usage: make <command> [VARIABLE=value]' \
		'' \
		'Commands:' \
		'  help            Show help (default)' \
		'  tidy            Tidy go.mod and go.sum' \
		'  format          Format Go source files' \
		'  format-check    Check Go formatting without changing files' \
		'  vet             Run go vet' \
		'  lint            Run golangci-lint v2' \
		'  compile         Compile all packages without running code' \
		'  test            Run tests (may access the configured API)' \
		'  check           Run format-check, vet, lint and compile' \
		'' \
		'Variables: GO, GOFMT, GOLANGCI_LINT, GOOS, GOARCH' \
		'Example: make compile GOOS=windows GOARCH=amd64'

tidy:
	@$(GO) mod tidy

format:
	@$(GOFMT) -w $(GO_FILES)

format-check:
	@set -e; files=$$($(GOFMT) -l $(GO_FILES)); \
	if [ -n "$$files" ]; then \
		printf '%s\n' 'Go files need formatting:' "$$files" >&2; \
		exit 1; \
	fi

vet:
	@$(GO) vet -mod=readonly ./...

lint:
	@$(GOLANGCI_LINT) run --config "$(LINT_CONFIG)" ./...

compile:
	@GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build -mod=readonly ./...

test:
	@if command -v gotestsum >/dev/null 2>&1; then \
		gotestsum --format testname --format-icons text -- -mod=readonly -race -count=1 -failfast -v ./...; \
	else \
		$(GO) test -mod=readonly -race -count=1 -failfast -v ./...; \
	fi

check:
	@$(MAKE) --no-print-directory format-check
	@$(MAKE) --no-print-directory vet
	@$(MAKE) --no-print-directory lint
	@$(MAKE) --no-print-directory compile
