GO_CMD := go
BIN_DIR ?= bin
DIST_DIR ?= dist
GOPROXY ?= https://proxy.golang.org,direct
export GOPROXY

VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed "s/^v//")
LDFLAGS := $(if $(VERSION),-ldflags "-X github.com/besrabasant/ssh-tunnel-manager/config.AppVersion=$(VERSION)")
PROTOC ?= protoc
PROTOC_VERSION := 36.1
PROTO_FILE := rpc/daemon.proto
PROTO_GEN_FILES := rpc/daemon.pb.go rpc/daemon_grpc.pb.go
PROTOC_GEN_GO_VERSION := v1.36.12
PROTOC_GEN_GO_GRPC_VERSION := v1.6.2
GOBIN := $(shell $(GO_CMD) env GOPATH)/bin
GO_FILES := $(shell find . -name '*.go' -not -path './vendor/*')

.PHONY: all fmt fmt-check test test-race coverage vet check build build-daemon build-client build-linux build-macos build-all download-deps proto-tools proto proto-check clean help gen_proto download_deps build_daemon build_client build_linux build_macos build_all

all: build

fmt:
	gofmt -w $(GO_FILES)

fmt-check:
	@files="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$files" ]; then printf '%s\n' "$$files"; gofmt -d $(GO_FILES); exit 1; fi

test:
	$(GO_CMD) test ./...

test-race:
	$(GO_CMD) test -race ./...

coverage:
	$(GO_CMD) test -coverprofile=coverage.out ./...
	$(GO_CMD) tool cover -func=coverage.out

vet:
	$(GO_CMD) vet ./...

check: fmt-check vet test test-race build proto-check

build: build-daemon build-client

build-daemon: | $(BIN_DIR)
	$(GO_CMD) build $(LDFLAGS) -o $(BIN_DIR)/sshtmd ./daemon

build-client: | $(BIN_DIR)
	$(GO_CMD) build $(LDFLAGS) -o $(BIN_DIR)/sshtm ./client

build-linux: | $(DIST_DIR)
	GOOS=linux GOARCH=amd64 $(GO_CMD) build $(LDFLAGS) -o $(DIST_DIR)/sshtmd-linux-amd64 ./daemon
	GOOS=linux GOARCH=amd64 $(GO_CMD) build $(LDFLAGS) -o $(DIST_DIR)/sshtm-linux-amd64 ./client
	GOOS=linux GOARCH=arm64 $(GO_CMD) build $(LDFLAGS) -o $(DIST_DIR)/sshtmd-linux-arm64 ./daemon
	GOOS=linux GOARCH=arm64 $(GO_CMD) build $(LDFLAGS) -o $(DIST_DIR)/sshtm-linux-arm64 ./client

build-macos: | $(DIST_DIR)
	GOOS=darwin GOARCH=amd64 $(GO_CMD) build $(LDFLAGS) -o $(DIST_DIR)/sshtmd-darwin-amd64 ./daemon
	GOOS=darwin GOARCH=amd64 $(GO_CMD) build $(LDFLAGS) -o $(DIST_DIR)/sshtm-darwin-amd64 ./client
	GOOS=darwin GOARCH=arm64 $(GO_CMD) build $(LDFLAGS) -o $(DIST_DIR)/sshtmd-darwin-arm64 ./daemon
	GOOS=darwin GOARCH=arm64 $(GO_CMD) build $(LDFLAGS) -o $(DIST_DIR)/sshtm-darwin-arm64 ./client

build-all: build-linux build-macos

$(BIN_DIR) $(DIST_DIR):
	mkdir -p $@

download-deps:
	$(GO_CMD) mod download

proto-tools:
	GOBIN=$(GOBIN) $(GO_CMD) install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	GOBIN=$(GOBIN) $(GO_CMD) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

proto:
	@command -v $(PROTOC) >/dev/null || (echo "protoc is required; install protobuf compiler $(PROTOC_VERSION)" >&2; exit 1)
	@actual="$$($(PROTOC) --version | awk '{print $$2}')"; test "$$actual" = "$(PROTOC_VERSION)" || (echo "protoc $(PROTOC_VERSION) is required (found $$actual)" >&2; exit 1)
	@PATH="$(GOBIN):$$PATH"; export PATH; command -v protoc-gen-go >/dev/null || (echo "protoc-gen-go is required; run 'make proto-tools'" >&2; exit 1)
	@PATH="$(GOBIN):$$PATH"; export PATH; command -v protoc-gen-go-grpc >/dev/null || (echo "protoc-gen-go-grpc is required; run 'make proto-tools'" >&2; exit 1)
	PATH="$(GOBIN):$$PATH" $(PROTOC) --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative $(PROTO_FILE)

proto-check:
	@command -v $(PROTOC) >/dev/null || (echo "protoc is required; install protobuf compiler $(PROTOC_VERSION)" >&2; exit 1)
	@actual="$$($(PROTOC) --version | awk '{print $$2}')"; test "$$actual" = "$(PROTOC_VERSION)" || (echo "protoc $(PROTOC_VERSION) is required (found $$actual)" >&2; exit 1)
	@PATH="$(GOBIN):$$PATH"; export PATH; command -v protoc-gen-go >/dev/null && command -v protoc-gen-go-grpc >/dev/null || (echo "protobuf generators are required; run 'make proto-tools'" >&2; exit 1)
	@tmp="$$(mktemp -d)"; trap 'rm -rf "$$tmp"' EXIT; \
		PATH="$(GOBIN):$$PATH" $(PROTOC) --go_out="$$tmp" --go_opt=paths=source_relative --go-grpc_out="$$tmp" --go-grpc_opt=paths=source_relative $(PROTO_FILE); \
		for file in $(PROTO_GEN_FILES); do \
			sed -E '/^\/\/.*protoc[[:space:]].*v?[0-9]+\./d' "$$file" > "$$tmp/expected"; \
			sed -E '/^\/\/.*protoc[[:space:]].*v?[0-9]+\./d' "$$tmp/$$file" > "$$tmp/actual"; \
			diff -u "$$tmp/expected" "$$tmp/actual" || exit 1; \
		done

# Backward-compatible aliases used by packaging and older documentation.
gen_proto: proto
download_deps: download-deps
build_daemon: build-daemon
build_client: build-client
build_linux: build-linux
build_macos: build-macos
build_all: build-all

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR) coverage.out

help:
	@echo "Usage:"
	@echo "  make fmt            Format Go files"
	@echo "  make fmt-check      Verify Go formatting"
	@echo "  make test           Run tests"
	@echo "  make test-race      Run tests with the race detector"
	@echo "  make coverage       Write coverage.out and report coverage"
	@echo "  make vet            Run go vet"
	@echo "  make check          Run formatting, vet, tests, builds, and protobuf freshness checks"
	@echo "  make proto-tools    Install pinned protobuf Go generators"
	@echo "  make proto          Regenerate committed protobuf Go files"
	@echo "  make proto-check    Verify committed protobuf Go files are current"
	@echo "  make build          Build daemon and CLI under ./$(BIN_DIR)"
	@echo "  make VERSION=1.2.3 build-all  Build cross-platform binaries under ./$(DIST_DIR)"
	@echo "  BIN_DIR=path make build       Override the local build output directory"
	@echo "  DIST_DIR=path make build-all  Override the cross-platform output directory"
