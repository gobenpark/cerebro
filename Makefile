GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
VERSION=1.0
GITCOMMITCOUNT:=$$(git rev-list HEAD | wc -l | tr -d ' ')
GITHASH:=$$(git rev-parse --short HEAD)
DATETIME:=$$(date "+%Y%m%d.%H%M%S")
VERSIONS:=$(VERSION).$(GITCOMMITCOUNT)-$(GITHASH)-$(DATETIME)
.PHONY: rm clean lint generate test download build

rm:
	rm -f server

clean:
	$(GOCLEAN)

lint:
	golangci-lint run ./...

generate:
	$(GOCMD) generate ./...

test:
	$(GOCMD) test -race ./...

download:
	$(GOCMD) mod download

build: clean download
	$(GOBUILD) -o server -gcflags='-N -l' -ldflags "-X main.ServiceVersion=$(VERSIONS)" ./cmd

cov:
	$(GOCMD) test -race -coverprofile=coverage.txt -covermode=atomic ./...