.PHONY: all build build-gui fmt lint test test-integration conformance e2e demo trace example-proxy example-envoy \
	tutorial tutorial-exec tutorial\:exec \
	docs-dev docs\:dev docs-build docs\:build \
	gui-dev gui\:dev gui-build gui\:build \
	docker-up docker-up-demo docker-down docker-ca docker-logs \
	k8s-up k8s-up-demo k8s-down k8s-delete k8s-ca k8s-status k8s-logs \
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

# Embeds the console (gui/) in alexandria-server and alexandria-client under /ui/; requires pnpm.
build-gui: build
	cd gui && pnpm install --frozen-lockfile && pnpm build:embed
	go build -tags gui -o $(BIN_DIR)/alexandria-server ./cmd/alexandria-server
	go build -tags gui -o $(BIN_DIR)/alexandria-client ./cmd/alexandria-client

fmt:
	go tool gofumpt -w .

lint:
	go tool golangci-lint run

test:
	go test -race -coverprofile=coverage.out ./...

test-integration:
	go test -tags="integration,jwx_es256k" -race -v ./internal/server/adapters/postgres ./internal/client/adapters/store_postgres

cover: test
	go tool cover -html=coverage.out

conformance:
	go test ./conformance/...

e2e:
	go test -v ./conformance -run '^TestE2E_F6$$'

demo: build
	go run ./conformance/demo -bin-dir $(BIN_DIR)

# Deployment examples: the client as L7 proxy, and Envoy as proxy with the client as validator (needs envoy or Docker).
example-proxy: build
	go run ./examples/proxy -bin-dir $(BIN_DIR)

example-envoy: build
	go run ./examples/envoy -bin-dir $(BIN_DIR)

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

gui-dev gui\:dev:
	cd gui && pnpm install && pnpm dev

gui-build gui\:build:
	cd gui && pnpm install && pnpm build

docker-up:
	docker compose -f deploy/docker/docker-compose.yaml up -d --build

docker-up-demo:
	docker compose -f deploy/docker/docker-compose.yaml --profile demo up -d --build

docker-down:
	docker compose -f deploy/docker/docker-compose.yaml --profile demo down

docker-ca:
	docker compose -f deploy/docker/docker-compose.yaml cp caddy:/certs/root.crt deploy/docker/caddy-root.crt

docker-logs:
	docker compose -f deploy/docker/docker-compose.yaml --profile demo logs -f

k8s-up:
	./deploy/k8s/scripts/up.sh

k8s-up-demo:
	./deploy/k8s/scripts/up.sh --demo

k8s-down:
	./deploy/k8s/scripts/down.sh

k8s-delete:
	./deploy/k8s/scripts/down.sh --cluster

k8s-ca:
	./deploy/k8s/scripts/gen-certs.sh --certs-only

k8s-status:
	kubectl --context kind-alexandria -n alexandria get pods,svc,ingress,pvc

k8s-logs:
	kubectl --context kind-alexandria -n alexandria logs -l app.kubernetes.io/part-of=alexandria --all-containers=true --prefix=true -f --max-log-requests=10

tidy:
	go mod tidy

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

check: fmt lint test conformance trace

clean:
	rm -rf $(BIN_DIR) coverage.out
