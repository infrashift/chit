BINARY  := chit-tui
BIN_DIR := bin
CMD     := ./cmd/chit-tui

.PHONY: all build test cover lint clean e2e-build e2e-build-server test-e2e test-e2e-clean

all: lint test build

# GOWORK=off keeps this module resolving standalone from its own go.mod even
# though it sits inside the chit workspace (see go.work at the repo root).
build:
	GOWORK=off go build -o $(BIN_DIR)/$(BINARY) $(CMD)

test:
	GOWORK=off go test -race ./...

cover:
	GOWORK=off go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@rm -f coverage.out

lint:
	$(shell go env GOPATH)/bin/golangci-lint run ./...

clean:
	rm -rf $(BIN_DIR) coverage.out

# --- E2E Testing ---

E2E_POD     := chit-tui-e2e
E2E_IMAGE   := chit-tui-e2e:latest
PG_IMAGE    := docker.io/library/postgres:17-alpine
CHIT_IMAGE  := chit-e2e:latest
CHIT_SRC    ?= ../..

e2e-build:
	podman build -t $(E2E_IMAGE) -f Containerfiles/Containerfile.e2e .

e2e-build-server:
	podman build -t $(CHIT_IMAGE) -f $(CHIT_SRC)/Containerfiles/Containerfile.e2e $(CHIT_SRC)

test-e2e: e2e-build e2e-build-server
	-podman pod rm -f $(E2E_POD) 2>/dev/null
	podman pod create --name $(E2E_POD) --share net
	podman run -d --pod $(E2E_POD) --name $(E2E_POD)-pg \
		-e POSTGRES_USER=chit -e POSTGRES_PASSWORD=chit -e POSTGRES_DB=chit \
		$(PG_IMAGE)
	podman run -d --pod $(E2E_POD) --name $(E2E_POD)-chitd \
		-e CHIT_DATABASE_URL=postgres://chit:chit@localhost:5432/chit?sslmode=disable \
		$(CHIT_IMAGE) bash /src/scripts/uat-entrypoint.sh
	podman run --rm --pod $(E2E_POD) --name $(E2E_POD)-tests \
		-e CHIT_E2E_SERVER_URL=http://localhost:8065 \
		$(E2E_IMAGE) ; \
	EXIT_CODE=$$? ; \
	podman pod rm -f $(E2E_POD) ; \
	exit $$EXIT_CODE

test-e2e-clean:
	-podman pod rm -f $(E2E_POD) 2>/dev/null
