.PHONY: dev prod test run build

LOCAL_CONFIG ?= ./config.json
PROD_CONFIG ?= $(HOME)/.config/plums/config.json

dev:
	@echo "Dev run"
	@echo "Config: $(LOCAL_CONFIG)"
	go run . --config $(LOCAL_CONFIG)

prod:
	@echo "Prod run"
	@echo "Config: $(PROD_CONFIG)"
	go run . --config $(PROD_CONFIG)

run:
	go run .

test:
	go test ./...

build:
	go build -o bin/plums .
