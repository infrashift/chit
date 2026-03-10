.PHONY: build build-mcp build-reconcile build-all run test test-container test-e2e test-e2e-clean lint validate-spec cue-validate reconcile migrate-up migrate-down kube-up kube-down kube-migrate uat-up uat-down docs-dev clean

BINARY=bin/chitd
MCP_BINARY=bin/chit-mcp
RECONCILE_BINARY=bin/chit-reconcile
KUBE_FILE=deploy/chit.kube.yml
MIGRATE_URL?=postgres://chit:chit@localhost:5432/chit?sslmode=disable

E2E_POD=chit-e2e
E2E_IMAGE=chit-e2e:latest
PG_IMAGE=docker.io/library/postgres:17-alpine

UAT_POD=chit-uat
UAT_PORT?=8065

build:
	go build -o $(BINARY) ./cmd/chitd

build-mcp:
	go build -o $(MCP_BINARY) ./cmd/chit-mcp

build-reconcile:
	go build -o $(RECONCILE_BINARY) ./cmd/chit-reconcile

build-all: build build-mcp build-reconcile

validate-spec:
	go run github.com/getkin/kin-openapi/cmd/validate@latest api/openapi.yaml

run: build
	./$(BINARY)

test:
	go test -race -count=1 ./...

test-container:
	podman build -t chit-test:latest -f Containerfiles/Containerfile.test .
	podman run --rm chit-test:latest

test-e2e:
	podman build -t $(E2E_IMAGE) -f Containerfiles/Containerfile.e2e .
	-podman pod rm -f $(E2E_POD) 2>/dev/null
	podman pod create --name $(E2E_POD) --share net
	podman run -d --pod $(E2E_POD) --name $(E2E_POD)-pg \
		-e POSTGRES_USER=chit -e POSTGRES_PASSWORD=chit -e POSTGRES_DB=chit \
		$(PG_IMAGE)
	podman run --rm --pod $(E2E_POD) --name $(E2E_POD)-tests \
		-e CHIT_DATABASE_URL=postgres://chit:chit@localhost:5432/chit?sslmode=disable \
		$(E2E_IMAGE) ; \
	EXIT_CODE=$$? ; \
	podman pod rm -f $(E2E_POD) ; \
	exit $$EXIT_CODE

test-e2e-clean:
	-podman pod rm -f $(E2E_POD) 2>/dev/null

lint:
	golangci-lint run

## --- CUE / Reconciler ---

cue-validate:
	go run cuelang.org/go/cmd/cue@latest eval ./auth/...

reconcile: build-reconcile
	./$(RECONCILE_BINARY)

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

## --- UAT (Manual Acceptance Testing) ---

uat-up:
	podman build -t $(E2E_IMAGE) -f Containerfiles/Containerfile.e2e .
	-podman pod rm -f $(UAT_POD) 2>/dev/null
	podman pod create --name $(UAT_POD) --share net -p $(UAT_PORT):8065
	podman run -d --pod $(UAT_POD) --name $(UAT_POD)-pg \
		-e POSTGRES_USER=chit -e POSTGRES_PASSWORD=chit -e POSTGRES_DB=chit \
		$(PG_IMAGE)
	podman run -d --pod $(UAT_POD) --name $(UAT_POD)-chitd \
		-e CHIT_DATABASE_URL=postgres://chit:chit@localhost:5432/chit?sslmode=disable \
		$(E2E_IMAGE) bash /src/scripts/uat-entrypoint.sh
	@echo ""
	@echo "UAT environment starting..."
	@echo "  Server: http://localhost:$(UAT_PORT)/api/v1/system/ping"
	@echo "  Logs:   podman logs -f $(UAT_POD)-chitd"
	@echo ""

uat-down:
	-podman pod rm -f $(UAT_POD) 2>/dev/null

## --- Docs ---

docs-dev:
	cd docs && bun --bun run dev

clean:
	rm -rf bin/
