.PHONY: build build-mcp build-all run test lint validate-spec migrate-up migrate-down kube-up kube-down kube-migrate clean

BINARY=bin/chitd
MCP_BINARY=bin/chit-mcp
KUBE_FILE=deploy/chit.kube.yml
MIGRATE_URL?=postgres://chit:chit@localhost:5432/chit?sslmode=disable

build:
	go build -o $(BINARY) ./cmd/chitd

build-mcp:
	go build -o $(MCP_BINARY) ./cmd/chit-mcp

build-all: build build-mcp

validate-spec:
	go run github.com/getkin/kin-openapi/cmd/validate@latest api/openapi.yaml

run: build
	./$(BINARY)

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run

## --- Database Migrations ---

migrate-up:
	migrate -database "$(MIGRATE_URL)" -path migrations up

migrate-down:
	migrate -database "$(MIGRATE_URL)" -path migrations down

## --- Podman Kube ---

kube-up:
	podman kube play --network host $(KUBE_FILE)

kube-down:
	podman kube down $(KUBE_FILE)

# Run Kratos and Keto schema migrations against the running pod.
# Postgres init script creates the kratos/keto databases on first start.
kube-migrate:
	podman run --rm --network host docker.io/oryd/kratos:v1.3 \
		migrate sql "postgres://chit:chit@localhost:5432/kratos?sslmode=disable" --yes
	podman run --rm --network host \
		-v $(CURDIR)/deploy/keto/keto.yml:/home/ory/keto.yml:ro,Z \
		-e DSN=postgres://chit:chit@localhost:5432/keto?sslmode=disable \
		docker.io/oryd/keto:v0.12 \
		migrate up --yes
	@echo "Run 'make migrate-up' to apply chit schema migrations."

clean:
	rm -rf bin/
