GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTOOL=$(GOCMD) tool
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOVET = $(GOCMD) vet
GOMOD = $(GOCMD) mod

BIN_DIR=./bin

CHANNELS = discord
BINARIES = tinker apiserver runner

all: test build

##@ General

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

build: $(addprefix build-,$(BINARIES)) $(addprefix build-channel-,$(CHANNELS)) ## Build all binaries

build-channel-%: ## Build a specific channel binary
	$(GOBUILD) -o $(BIN_DIR)/channel-$* ./cmd/channel/$*/

build-%: ## Build a specific binary (e.g., make build-tinker)
	$(GOBUILD) -o $(BIN_DIR)/$* ./cmd/$*/

test: 
	$(GOTEST) ./...

coverage:
	$(GOTEST) ./... -coverprofile=coverage.out
	$(GOTOOL) cover -html=coverage.out

benchmark:
	$(GOTEST) ./... -bench=. -benchmem

vet: ## Run go vet
	$(GOVET) ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Run gofmt
	gofmt -s -w .

tidy: ## Run go mod tidy
	$(GOMOD) tidy

##@ Web UI

web-install: ## Install frontend dependencies
	cd internal/web && npm ci

web-build: web-install ## Build the frontend for embedding
	cd internal/web && npm run build

web-dev: web-install ## Start the frontend dev server (hot-reload, proxy to :8080)
	cd internal/web && npm run dev

web-clean: ## Remove frontend build artifacts
	rm -rf internal/web/dist internal/web/node_modules

web-dev-serve: web-install
	go run ./cmd/apiserver/

##@ Local Development

API_ADDR ?= :8080
VITE_PORT ?= 11435

API_LOCAL_PORT ?= 8081
NATS_LOCAL_PORT ?= 4222

run-channels: ## Start all channels
	$(GOCMD)

local-nats: ## Start NATS JetStream server
	nats-server -js

run-channel: build-channel-discord ## Start the Discord channel
	@$(BIN_DIR)/channel-discord \
		--event-bus-url nats://localhost:$(NATS_LOCAL_PORT)

run-runner: build-runner ## Start the runner
	@$(BIN_DIR)/runner \
		--event-bus-url nats://localhost:$(NATS_LOCAL_PORT)

dev-all: ## Start everything locally: channel, runner, Vite, and local NATS 
	@echo ""
	@echo "============================================"
	@echo "  Tinker Local Development"
	@echo "============================================"
	@echo "  UI:        http://localhost:$(VITE_PORT)"
	@echo "============================================"
	@echo ""
	$(MAKE) -j4 local-nats web-dev-serve run-channel run-runner 
##@ Release

VERSION_FILE=VERSION
CURRENT_VERSION=$(shell cat $(VERSION_FILE))
MAJOR=$(shell echo $(CURRENT_VERSION) | cut -d. -f1)
MINOR=$(shell echo $(CURRENT_VERSION) | cut -d. -f2)
PATCH=$(shell echo $(CURRENT_VERSION) | cut -d. -f3)

release-patch: ## Release patch e.g., 0.1.1 -> 0.1.2
	$(eval NEW_VERSION=$(MAJOR).$(MINOR).$(shell echo $$(($(PATCH)+1))))
	@echo "$(NEW_VERSION)" > $(VERSION_FILE)
	git add $(VERSION_FILE)
	git commit -m "release: v$(NEW_VERSION)"
	git tag "v$(NEW_VERSION)"
	git push && git push --tags

release-minor: ## Release minor e.g., 0.1.10 -> 0.2.1
	$(eval NEW_VERSION=$(MAJOR).$(shell echo $$(($(MINOR)+1))).0)
	@echo "$(NEW_VERSION)" > $(VERSION_FILE)
	git add $(VERSION_FILE)
	git commit -m "release: v$(NEW_VERSION)"
	git tag "v$(NEW_VERSION)"
	git push && git push --tags

release-major: ## Release major e.g., 0.1.1 -> 1.1.1
	$(eval NEW_VERSION=$(shell echo $$(($(MAJOR)+1))).0.0)
	@echo "$(NEW_VERSION)" > $(VERSION_FILE)
	git add $(VERSION_FILE)
	git commit -m "release: v$(NEW_VERSION)"
	git tag "v$(NEW_VERSION)"
	git push && git push --tags

