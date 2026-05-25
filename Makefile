.DEFAULT_GOAL := run

.PHONY: build dev prod run test

APP := plums
BIN_DIR := bin
BIN := $(BIN_DIR)/$(APP)
GO := go

LOCAL_CONFIG ?= ./config.json
PROD_CONFIG ?= $(HOME)/.config/plums/config.json

run:
	$(GO) run .

dev:
	@echo "Dev run"
	@echo "Config: $(LOCAL_CONFIG)"
	$(GO) run . --config $(LOCAL_CONFIG)

prod:
	@echo "Prod run"
	@echo "Config: $(PROD_CONFIG)"
	$(GO) run . --config $(PROD_CONFIG)

test:
	$(GO) test ./...

build:
	$(GO) build -o $(BIN) .
