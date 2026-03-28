.PHONY: build install test test-integration lint lint-new fmt vet preflight install-hooks clean smoke

BIN := bin/emcee
GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

build:
	go build -o $(BIN) .

install: build
	cp $(BIN) $(GOBIN)/emcee
	@echo "installed to $(GOBIN)/emcee"

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

lint-new:
	golangci-lint run --new-from-rev=HEAD ./...

preflight: fmt vet lint test

install-hooks:
	@echo '#!/bin/sh' > .git/hooks/pre-commit
	@echo 'make lint-new' >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "pre-commit hook installed (runs make lint-new)"

clean:
	rm -rf bin/

smoke: build
	@echo "=== smoke: help ==="
	$(BIN) --help
	@echo ""
	@echo "=== smoke: list --help ==="
	$(BIN) list --help
	@echo ""
	@echo "=== smoke: get --help ==="
	$(BIN) get --help
	@echo ""
	@echo "=== smoke: create --help ==="
	$(BIN) create --help
	@echo ""
	@echo "=== smoke: serve --help ==="
	$(BIN) serve --help
	@echo ""
	@echo "smoke test passed"
