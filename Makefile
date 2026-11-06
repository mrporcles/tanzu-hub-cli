NAME ?= th
OUTPUT = ./dist/$(NAME)
GO_SOURCES = $(shell find . -type f -name '*.go')
VERSION ?= $(or $(shell git describe --tags --abbrev=0 2>/dev/null | sed "s/^v//"),dev)
LDFLAGS_VERSION = -X github.com/laidbackware/tanzu-hub-cli/cmd.version=$(VERSION)

.PHONY: all
all: build lint test ## Runs build, lint and test

.PHONY: clean
clean: ## Clean testcache and delete build output
	go clean -testcache
	@rm -rf bin/
	@rm -rf dist/

$(OUTPUT): $(GO_SOURCES)

.PHONY: build
build: $(OUTPUT) ## Build the main binary
	@echo "Building $(VERSION)"
	go build -ldflags "$(LDFLAGS_VERSION)" -o ./bin/$(NAME) .

.PHONY: test
test: ## Run the unit tests
	go test -short ./...

.PHONY: release
release: $(GO_SOURCES) ## Cross-compile binary for various operating systems
	@rm -rf dist
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin   GOARCH=amd64 go build -trimpath -ldflags "-w $(LDFLAGS_VERSION)" -o $(OUTPUT)-darwin-amd64-v$(VERSION)
	CGO_ENABLED=0 GOOS=darwin   GOARCH=arm64 go build -trimpath -ldflags "-w $(LDFLAGS_VERSION)" -o $(OUTPUT)-darwin-arm64-v$(VERSION)
	CGO_ENABLED=0 GOOS=linux    GOARCH=amd64 go build -trimpath -ldflags "-w $(LDFLAGS_VERSION)" -o $(OUTPUT)-linux-amd64-v$(VERSION)
	CGO_ENABLED=0 GOOS=windows  GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS_VERSION)"    -o $(OUTPUT)-windows-amd64-v$(VERSION).exe

.PHONY: lint
lint: ## Validate style and syntax
	@if ! command -v golangci-lint > /dev/null 2>&1 && [ ! -f ./bin/golangci-lint ]; then \
		echo "Installing latest golangci-lint"; \
		curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s latest; \
		echo "[OK] golangci-lint installed"; \
	fi
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run; \
	else \
		./bin/golangci-lint run; \
	fi

.PHONY: tidy
tidy: ## Remove unused dependencies
	go mod tidy

.PHONY: list
list: ## Print the current module's dependencies.
	go list -m all

# Absolutely awesome: http://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
help: ## Print help for each make target
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'