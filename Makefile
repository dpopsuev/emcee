.PHONY: build test test-integration lint clean smoke

BIN := bin/emcee

build:
	go build -o $(BIN) .

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
