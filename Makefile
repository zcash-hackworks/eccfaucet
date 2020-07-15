SHELL := /bin/bash

VERSION := `git describe --always`
GITCOMMIT := `git rev-parse HEAD`
BRANCH := `git rev-parse --abbrev-ref HEAD`
BUILDDATE := `date +%Y-%m-%d`
BUILDUSER := `whoami`

LDFLAGSSTRING :=-X main.Version=$(VERSION)
LDFLAGSSTRING +=-X main.GitCommit=$(GITCOMMIT)
LDFLAGSSTRING +=-X main.Branch=$(BRANCH)
LDFLAGSSTRING +=-X main.BuildDate=$(BUILDDATE)
LDFLAGSSTRING +=-X main.BuildUser=$(BUILDUSER)

LDFLAGS :=-ldflags "$(LDFLAGSSTRING)"

.PHONY: all build

all: build

# Build binary
build:
	CGO_ENABLED=0 go build $(LDFLAGS) 

test:
	go test -v ./...