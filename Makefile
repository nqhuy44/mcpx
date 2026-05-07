SHELL := /bin/bash

# ── arguments ────────────────────────────────────────────────────────────────
# SERVER: which component to build/test (proxy | git | cicd | ...)
#         leave empty to target all components
# OS / ARCH: cross-compile target (e.g. make build OS=linux ARCH=amd64)
# CONFIG: path to gateway config for `make run` (default: proxy/gateway.yaml)
SERVER ?=
OS     ?= $(shell go env GOOS)
ARCH   ?= $(shell go env GOARCH)
CONFIG ?= proxy/gateway.yaml

# ── derived ───────────────────────────────────────────────────────────────────
BIN_DIR  := $(CURDIR)/bin
# All Go modules: proxy + any servers/* directory that contains a go.mod
ALL_MODULES := proxy $(patsubst %/go.mod,%,$(wildcard servers/*/go.mod))

# Resolve target module list
ifeq ($(SERVER),)
  TARGETS := $(ALL_MODULES)
else
  TARGETS := $(filter $(SERVER) servers/$(SERVER) proxy,$(ALL_MODULES))
  ifeq ($(TARGETS),)
    $(error Unknown SERVER "$(SERVER)". Available: $(ALL_MODULES))
  endif
endif

# Binary name for a module path (proxy → mcpx-proxy, servers/git → mcpx-git)
bin_name = $(BIN_DIR)/mcpx-$(notdir $(1))

# ── phony targets ─────────────────────────────────────────────────────────────
.PHONY: all build test clean run lint fmt help

all: build

## build [SERVER=<name>] [OS=<os>] [ARCH=<arch>]  — compile binaries
build:
	@mkdir -p $(BIN_DIR)
	@for mod in $(TARGETS); do \
	    name=mcpx-$$(basename $$mod); \
	    out=$(BIN_DIR)/$$name; \
	    [ "$(OS)/$(ARCH)" != "$$(go env GOOS)/$$(go env GOARCH)" ] && out=$$out-$(OS)-$(ARCH); \
	    echo "  build  $$mod  →  $$out"; \
	    (cd $$mod && GOOS=$(OS) GOARCH=$(ARCH) CGO_ENABLED=0 go build -trimpath -o $$out ./cmd) || exit 1; \
	    [ -f $$mod/gateway.yaml ] && cp $$mod/gateway.yaml $(BIN_DIR)/gateway.yaml || true; \
	done

## test [SERVER=<name>]  — run tests
test:
	@for mod in $(TARGETS); do \
	    echo "  test   $$mod"; \
	    (cd $$mod && go test ./...) || exit 1; \
	done

## lint [SERVER=<name>]  — vet + staticcheck (staticcheck must be installed)
lint:
	@for mod in $(TARGETS); do \
	    echo "  lint   $$mod"; \
	    (cd $$mod && go vet ./...) || exit 1; \
	    command -v staticcheck >/dev/null && (cd $$mod && staticcheck ./...) || true; \
	done

## fmt [SERVER=<name>]  — gofmt in-place
fmt:
	@for mod in $(TARGETS); do \
	    echo "  fmt    $$mod"; \
	    (cd $$mod && gofmt -w .); \
	done

## run [CONFIG=<path>]  — run the proxy (builds first)
run:
	./bin/mcpx-proxy

## clean  — remove bin/
clean:
	rm -rf $(BIN_DIR)

## help  — print this message
help:
	@grep -E '^## ' Makefile | sed 's/^## /  /'
