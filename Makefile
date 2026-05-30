.DEFAULT_GOAL := run

.PHONY: build clean dev install prod run test

APP := plums
BIN_DIR := bin
BIN := $(BIN_DIR)/$(APP)
GO := go
VERSION ?= 0.1.0-dev
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || printf unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

ENTRY := ./cmd/plums

run:
	$(GO) run $(ENTRY)

dev:
	@echo "Dev run"
	@echo "Config: ./.agents/plums/config/layout.json"
	$(GO) run $(ENTRY) --config-local

prod:
	@echo "Prod run"
	@echo "Config: $(HOME)/.config/plums/config/layout.json"
	$(GO) run $(ENTRY) --config-global

test:
	$(GO) test ./...

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) $(ENTRY)

install:
	$(GO) install -ldflags "$(LDFLAGS)" $(ENTRY)

clean:
	rm -rf $(BIN_DIR)
