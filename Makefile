.DEFAULT_GOAL := run

.PHONY: build dev prod run test

APP := plums
BIN_DIR := bin
BIN := $(BIN_DIR)/$(APP)
GO := go
VERSION ?= 0.1.0-dev
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || printf unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

run:
	$(GO) run .

dev:
	@echo "Dev run"
	@echo "Config: ./.agents/plums/config/layout.json"
	$(GO) run . --config-local

prod:
	@echo "Prod run"
	@echo "Config: $(HOME)/.config/plums/config/layout.json"
	$(GO) run . --config-global

test:
	$(GO) test ./...

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) .
