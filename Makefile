.PHONY: all build test conformance e2e demo trace clean

BIN_DIR ?= bin
GO_TEST_ENV := ALEXANDRIA_AUTH_CONFIG_ENABLED=false
GO_TAGS := -tags=jwx_es256k

all: build test conformance trace

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/vhub-server ./cmd/vhub-server
	go build -o $(BIN_DIR)/vhub-client ./cmd/vhub-client
	go build -o $(BIN_DIR)/fake-connector ./deploy/fake-connector

test:
	$(GO_TEST_ENV) go test $(GO_TAGS) ./internal/... ./cmd/...

conformance:
	go test ./conformance/...

e2e:
	go test -v ./conformance -run '^TestE2E_F6$$'

demo: build
	go run ./conformance/demo -bin-dir $(BIN_DIR)

trace:
	go run ./conformance/trace

clean:
	rm -rf $(BIN_DIR)
