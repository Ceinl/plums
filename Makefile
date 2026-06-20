.DEFAULT_GOAL := run

.PHONY: build clean dev fmt fmt-check init-config install prod run test vet

APP := plums
BIN_DIR := bin
BIN := $(BIN_DIR)/$(APP)
GO := go
VERSION ?= 0.1.0-dev
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || printf unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

ENTRY := ./cmd/plums
PLUMS_CONFIG_DIR := $(HOME)/.config/plums/config

# run: quick stock launch — no user config or plugins.
run:
	$(GO) run $(ENTRY)

# init-config: seed the global config (~/.config/plums/config) explicitly. A
# normal first launch also creates config.go when it is missing.
init-config:
	$(GO) run $(ENTRY) -init-config

# dev: compile the GLOBAL user config (~/.config/plums/config/config.go) and its
# plugins against THIS checkout, then run the result — so local plums changes and
# your config plugins both take effect.
dev:
	@test -f "$(PLUMS_CONFIG_DIR)/config.go" || { echo "no $(PLUMS_CONFIG_DIR)/config.go — run 'make init-config' or launch plums once first"; exit 1; }
	@echo "Dev run — global config + plugins from $(PLUMS_CONFIG_DIR), compiled against $(CURDIR)"
	$(GO) run $(ENTRY) build -plums-dir "$(CURDIR)" -o "$(BIN)"
	$(BIN)

# prod: how an installed plums launches — plain run auto-builds the global config
# when the published plums module is resolvable (no local replace directive).
prod:
	$(GO) run $(ENTRY)

test:
	$(GO) test -race ./...

fmt:
	gofmt -w .

fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi

vet:
	$(GO) vet ./...

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) $(ENTRY)

install:
	$(GO) install -ldflags "$(LDFLAGS)" $(ENTRY)

clean:
	rm -rf $(BIN_DIR)
