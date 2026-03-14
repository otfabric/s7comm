# Self-documented Makefile (https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html)
# Run 'make' or 'make help' to list targets.

.DEFAULT_GOAL := help

.PHONY: help all check test test-race coverage cover lint lint-ci fmt vet clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

all: ## Format, vet, and test
	@echo "Running all: fmt, vet, test"
	@$(MAKE) fmt vet test

check: fmt lint lint-ci vet test coverage ## Run all checks

test: ## Run unit tests with race detector
	@echo "Running tests"
	@go test -count=1 -race ./...

test-race: test ## Alias for race-enabled test run

coverage: ## Run tests with coverage (writes coverage.out)
	@echo "Running coverage"
	@go test -count=1 -race -coverprofile=coverage.out -covermode=atomic ./...

cover: coverage ## Open coverage report in browser
	@echo "Opening coverage report"
	@go tool cover -html=coverage.out

lint: ## Run staticcheck
	@echo "Running staticcheck"
	@staticcheck ./...

lint-ci: ## Run golangci-lint
	@echo "Running golangci-lint"
	@golangci-lint run ./...

fmt: ## Format Go code with gofmt
	@echo "Running gofmt"
	@gofmt -w .

vet: ## Run go vet
	@echo "Running go vet"
	@go vet ./...

clean: ## Remove generated coverage artifacts
	@rm -f coverage.out coverage.html
