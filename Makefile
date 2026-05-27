.DEFAULT_GOAL := run

.PHONY: build dev prod run test

APP := plums
BIN_DIR := bin
BIN := $(BIN_DIR)/$(APP)
GO := go

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
	$(GO) build -o $(BIN) .
