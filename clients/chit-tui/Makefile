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

# Found on PATH or in GOPATH/bin, like the root Makefile. The root lint-tui
# passes its own GOLANGCI_LINT, so both modules use the same binary.
GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null || echo $(shell go env GOPATH)/bin/golangci-lint)
lint:
	GOWORK=off $(GOLANGCI_LINT) run ./...

clean:
	rm -rf $(BIN_DIR) coverage.out

# --- E2E Testing ---

E2E_POD     := chit-tui-e2e
E2E_IMAGE   := chit-tui-e2e:latest
# The trusted-service-containers PostgreSQL 17 on UBI; see the root Makefile.
PG_IMAGE    := ghcr.io/infrashift/trusted-service-containers/trusted/postgresql17:17.12-9410cd4@sha256:6375c284b5aaa6d83096d74ae6b1b7a3340bcd81739e4309465b18a3e781d6e3
CHIT_IMAGE  := chit-e2e:latest
# The attachment store, as deploy/chit.kube.yml runs it.
SEAWEED_IMAGE := ghcr.io/infrashift/trusted-service-containers/trusted/seaweedfs:4.51-382cdac@sha256:7c6682e56006764282eb70a6a96703aafa6d808cd0c13ff6b8b1514b680985b7
# A HALO-like machine actor that may post on behalf of a person, so the
# e2e tests can make a real on-behalf-of post. Test credentials only.
# The pod also widens the rate limit (50/s, burst 200): the suite makes many
# requests as alice, and TestHALO_RateLimit still empties diana's bucket.
E2E_MACHINE_ACTORS := [{"client_id":"tui-e2e-halod","username":"halod-tui-e2e","display_name":"halod (TUI e2e)","roles":"system_user","actor_type":"agent"}]
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
	podman run -d --pod $(E2E_POD) --name $(E2E_POD)-seaweedfs \
		-v $(CURDIR)/tests/e2e/seaweedfs:/etc/seaweedfs:ro,Z \
		$(SEAWEED_IMAGE) server -dir=/tmp -ip=127.0.0.1 -ip.bind=127.0.0.1 \
		-master.port=9333 -master.port.grpc=19333 -master.volumeSizeLimitMB=64 \
		-volume.port=8380 -volume.port.grpc=18380 -filer.port=8888 -filer.port.grpc=18888 \
		-s3 -s3.port=8333 -s3.port.grpc=18334 -s3.port.iceberg=0 -s3.port.lance=0 \
		-s3.config=/etc/seaweedfs/s3.json
	podman run -d --pod $(E2E_POD) --name $(E2E_POD)-chitd \
		-e CHIT_DATABASE_URL=postgres://chit:chit@localhost:5432/chit?sslmode=disable \
		-e CHIT_S3_ENDPOINT=http://127.0.0.1:8333 \
		-e CHIT_S3_ACCESS_KEY_ID=chit-dev-access-key \
		-e CHIT_S3_SECRET_ACCESS_KEY=chit-dev-secret-key \
		-e 'CHIT_MACHINE_ACTORS=$(E2E_MACHINE_ACTORS)' \
		-e CHIT_ON_BEHALF_OF_CLIENTS=tui-e2e-halod \
		-e CHIT_RATE_LIMIT_USER_PER_SECOND=50 \
		-e CHIT_RATE_LIMIT_USER_BURST=200 \
		$(CHIT_IMAGE) bash /src/scripts/uat-entrypoint.sh
	podman run --rm --pod $(E2E_POD) --name $(E2E_POD)-tests \
		-e CHIT_E2E_SERVER_URL=http://localhost:8065 \
		$(E2E_IMAGE) ; \
	EXIT_CODE=$$? ; \
	podman pod rm -f $(E2E_POD) ; \
	exit $$EXIT_CODE

test-e2e-clean:
	-podman pod rm -f $(E2E_POD) 2>/dev/null
