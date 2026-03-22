.PHONY: build install test test-integration lint clean smoke

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

lint:
	go vet ./...

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
