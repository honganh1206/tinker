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
serve:
	$(GORUN) ./main.go serve

# Cross compilation
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BIN_DIR)/$(BINARY_UNIX) -v

