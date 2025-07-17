.PHONY: help build test test-unit test-integration test-integration-local clean setup-kind teardown-kind

# Default target
help: ## Show this help message
	@echo "GitOps Registration Service - Make Targets"
	@echo "=========================================="
	@echo "Usage: make <target> [IMAGE=<image-spec>]"
	@echo ""
	@echo "IMAGE examples:"
	@echo "  make test-integration-full IMAGE=quay.io/bcook/gitops-registration:latest  (default)"
	@echo "  make test-integration-full IMAGE=localhost/my-custom:latest              (local build)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}'

# Build targets
build: ## Build the GitOps registration service binary
	@echo "Building GitOps registration service..."
	go build -o bin/gitops-registration-service cmd/server/main.go

build-image: ## Build Docker image locally
	@echo "Building Docker image..."
	podman build -t gitops-registration:latest .

# Test targets
test: test-unit ## Run all tests

test-unit: ## Run unit tests
	@echo "Running unit tests..."
	go test -v ./internal/...

test-integration: ## Run integration tests (requires existing KIND cluster)
	@echo "Running integration tests on existing cluster..."
	cd test/integration && ./run-enhanced-tests.sh

# Complete integration test pipeline
test-integration-full: ## Complete integration test from scratch (single-node KIND)
	@echo "=========================================="
	@echo "GitOps Registration Service - Full Integration Test"
	@echo "=========================================="
	@$(MAKE) teardown-kind || true
	@$(MAKE) setup-integration-env
	@$(MAKE) deploy-service IMAGE=$(IMAGE)
	@$(MAKE) setup-test-data
	@$(MAKE) run-integration-tests
	@echo "=========================================="
	@echo "✅ Integration tests completed successfully!"
	@echo "=========================================="

# Individual setup targets
setup-integration-env: ## Set up complete integration environment
	@echo "Setting up integration test environment..."
	cd test/integration && ./setup-test-env.sh

deploy-service: ## Deploy the service to KIND (set IMAGE to override default)
	@echo "Deploying GitOps registration service..."
	cd test/integration && ./deploy-service.sh $(IMAGE)

setup-test-data: ## Set up git repositories and test data
	@echo "Setting up test repositories and data..."
	cd test/integration && ./setup-git-repos.sh
	cd test/integration && ./populate-git-repos.sh

run-integration-tests: ## Run the enhanced integration tests
	@echo "Running enhanced integration tests..."
	cd test/integration && ./run-enhanced-tests.sh

# KIND cluster management
setup-kind: ## Set up single-node KIND cluster
	@echo "Setting up KIND cluster..."
	cd test/integration && ./setup-kind-cluster.sh

teardown-kind: ## Tear down KIND cluster and clean up
	@echo "Tearing down KIND cluster..."
	kind delete cluster --name gitops-registration-test || true
	docker system prune -f || true

# Development targets
dev-setup: setup-integration-env setup-test-data ## Set up development environment
	@$(MAKE) deploy-service IMAGE=$(IMAGE)
	@echo "Development environment ready!"
	@echo "Service URL: http://gitops-registration.konflux-gitops.svc.cluster.local"
	@echo "ArgoCD UI: http://localhost:30080 (admin/admin123)"
	@echo "Gitea UI: http://localhost:30300 (gitops-admin/gitops123)"

dev-test: ## Run tests in development mode (assumes env is set up)
	@$(MAKE) deploy-service IMAGE=$(IMAGE)
	@$(MAKE) run-integration-tests

# Cleanup targets
clean: ## Clean up build artifacts
	@echo "Cleaning up build artifacts..."
	rm -rf bin/
	rm -rf coverage.out

clean-all: teardown-kind clean ## Clean up everything (cluster + artifacts)
	@echo "Complete cleanup finished!"

# Quick targets for common workflows
quick-test: ## Quick test (assumes environment exists)
	@$(MAKE) deploy-service IMAGE=$(IMAGE)
	@$(MAKE) run-integration-tests

full-reset: clean-all test-integration-full ## Complete reset and full test

# Status and debugging
status: ## Show status of integration environment
	@echo "Checking integration environment status..."
	cd test/integration && ./show-environment-status.sh

logs: ## Show service logs
	@echo "Showing GitOps registration service logs..."
	kubectl --context kind-gitops-registration-test logs -n gitops-system -l serving.knative.dev/service=gitops-registration --tail=50

# CI/CD targets
ci-test: ## Run tests suitable for CI/CD
	@$(MAKE) test-unit
	@$(MAKE) test-integration-full 