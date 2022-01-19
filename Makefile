GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
VERSION=1.0
GITCOMMITCOUNT:=$$(git rev-list HEAD | wc -l | tr -d ' ')
GITHASH:=$$(git rev-parse --short HEAD)
DATETIME:=$$(date "+%Y%m%d.%H%M%S")
VERSIONS:=$(VERSION).$(GITCOMMITCOUNT)-$(GITHASH)-$(DATETIME)

.PHONY: rm
rm: ## remove server binary
	rm -f server

.PHONY: clean
clean: ## go clean
	$(GOCLEAN)

.PHONY: lint
lint: ## golangci-lint
	golangci-lint run ./...

.PHONY: generate
generate: ## go generate
	$(GOCMD) generate ./...

.PHONY: test
test: ## go test
	$(GOCMD) test -race ./...

.PHONY: download
download: ## go mod download
	$(GOCMD) mod download

##@ Building
.PHONY: build
build: clean download ## go build
	$(GOBUILD) -o server -gcflags='-N -l' -ldflags "-X main.ServiceVersion=$(VERSIONS)" ./cmd

.PHONY: cov
cov: ## run code coverage
	$(GOCMD) test -race -coverprofile=coverage.txt -covermode=atomic ./...

##@ Helpers
.PHONEY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
