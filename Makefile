.PHONY: all build fmt lint test conformance e2e demo trace \
	tutorial tutorial-exec tutorial\:exec \
	docs-dev docs\:dev docs-build docs\:build \
	tidy vuln cover check clean

BIN_DIR ?= bin

all: build test conformance trace

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/alexandriac ./cmd/alexandriac
	go build -o $(BIN_DIR)/alexandria-server ./cmd/alexandria-server
	go build -o $(BIN_DIR)/alexandria-client ./cmd/alexandria-client
	go build -o $(BIN_DIR)/fake-idp ./deploy/fake-idp
	go build -o $(BIN_DIR)/fake-connector ./deploy/fake-connector

fmt:
	go tool gofumpt -w .

lint:
	go tool golangci-lint run

test:
	go test -race -coverprofile=coverage.out ./...

cover: test
	go tool cover -html=coverage.out

conformance:
	go test ./conformance/...

e2e:
	go test -v ./conformance -run '^TestE2E_F6$$'

demo: build
	go run ./conformance/demo -bin-dir $(BIN_DIR)

trace:
	go run ./conformance/trace

tutorial: build
	./tutorial/run

tutorial-exec tutorial\:exec: build
	python3 ./tutorial/run.py exec -v

docs-dev docs\:dev:
	cd docs && pnpm dev

docs-build docs\:build:
	cd docs && pnpm build

tidy:
	go mod tidy

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

check: fmt lint test conformance trace

clean:
	rm -rf $(BIN_DIR) coverage.out
