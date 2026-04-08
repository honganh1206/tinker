GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTOOL=$(GOCMD) tool
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=tinker
BINARY_UNIX=$(BINARY_NAME)_unix
BIN_DIR=./bin

all: test build
build: 
	$(GOBUILD) -o $(BIN_DIR)/$(BINARY_NAME) -v
test: 
	$(GOTEST) ./...
coverage:
	$(GOTEST) ./... -coverprofile=coverage.out
	$(GOTOOL) cover -html=coverage.out
benchmark:
	$(GOTEST) ./... -bench=. -benchmem
clean: 
	$(GOCLEAN)
	rm -f $(BIN_DIR)/$(BINARY_NAME)
	rm -f $(BIN_DIR)/$(BINARY_UNIX)
run:
	$(GORUN) ./main.go $(PROMPT)

# Cross compilation
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BIN_DIR)/$(BINARY_UNIX) -v

# Release helpers: make release-patch / release-minor / release-major
VERSION_FILE=VERSION
CURRENT_VERSION=$(shell cat $(VERSION_FILE))
MAJOR=$(shell echo $(CURRENT_VERSION) | cut -d. -f1)
MINOR=$(shell echo $(CURRENT_VERSION) | cut -d. -f2)
PATCH=$(shell echo $(CURRENT_VERSION) | cut -d. -f3)

release-patch:
	$(eval NEW_VERSION=$(MAJOR).$(MINOR).$(shell echo $$(($(PATCH)+1))))
	@echo "$(NEW_VERSION)" > $(VERSION_FILE)
	git add $(VERSION_FILE)
	git commit -m "release: v$(NEW_VERSION)"
	git tag "v$(NEW_VERSION)"
	git push && git push --tags

release-minor:
	$(eval NEW_VERSION=$(MAJOR).$(shell echo $$(($(MINOR)+1))).0)
	@echo "$(NEW_VERSION)" > $(VERSION_FILE)
	git add $(VERSION_FILE)
	git commit -m "release: v$(NEW_VERSION)"
	git tag "v$(NEW_VERSION)"
	git push && git push --tags

release-major:
	$(eval NEW_VERSION=$(shell echo $$(($(MAJOR)+1))).0.0)
	@echo "$(NEW_VERSION)" > $(VERSION_FILE)
	git add $(VERSION_FILE)
	git commit -m "release: v$(NEW_VERSION)"
	git tag "v$(NEW_VERSION)"
	git push && git push --tags

