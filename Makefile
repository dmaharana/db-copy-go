# Go parameters
BINARY_NAME=dbcopy
MAIN_PATH=./cmd/main.go
GO=go
GOFLAGS=-v

# Build directory
BUILD_DIR=build

# Git info
GIT_COMMIT=$(shell git rev-parse --short HEAD)
GIT_DIRTY=$(shell test -n "`git status --porcelain`" && echo "+CHANGES" || echo "")
BUILD_DATE=$(shell date '+%Y-%m-%d-%H:%M:%S')

# Linker flags
LDFLAGS=-ldflags "-X main.commit=${GIT_COMMIT}${GIT_DIRTY} -X main.date=${BUILD_DATE}"

# Make all directories
$(shell mkdir -p ${BUILD_DIR})

.PHONY: all build clean test help b c

all: clean build ## Build and clean

build b: ## Build the binary
	${GO} build ${GOFLAGS} ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME} ${MAIN_PATH}

clean c: ## Remove build directory
	rm -rf ${BUILD_DIR}

test: ## Run tests
	${GO} test -v ./...

fmt: ## Format the code
	${GO} fmt ./...

vet: ## Run go vet
	${GO} vet ./...

lint: ## Run linter
	if command -v golangci-lint >/dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint is not installed"; \
		exit 1; \
	fi

deps: ## Download dependencies
	${GO} mod download
	${GO} mod tidy

install: build ## Install binary to GOPATH/bin
	cp ${BUILD_DIR}/${BINARY_NAME} ${GOPATH}/bin/

uninstall: ## Remove binary from GOPATH/bin
	rm -f ${GOPATH}/bin/${BINARY_NAME}

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
