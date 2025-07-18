.PHONY: help build test test-unit test-integration test-integration-local clean setup-kind teardown-kind test-commit test-commit-fast

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

test-coverage: ## Run tests with coverage report
	@echo "Running unit tests with coverage..."
	go test -v -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-coverage-check: ## Run tests and check 70% coverage threshold
	@echo "Running unit tests with coverage threshold check..."
	go test -v -coverprofile=coverage.out -covermode=atomic ./...
	@COVERAGE=$$(go tool cover -func=coverage.out | grep "total:" | awk '{print $$3}' | sed 's/%//'); \
	echo "Current coverage: $${COVERAGE}%"; \
	echo "Required threshold: 70%"; \
	if [ $$(echo "$${COVERAGE} < 70" | bc -l) -eq 1 ]; then \
		echo "❌ Coverage $${COVERAGE}% is below required threshold of 70%"; \
		echo "Please add tests to increase coverage above 70%"; \
		exit 1; \
	else \
		echo "✅ Coverage $${COVERAGE}% meets the required threshold of 70%"; \
	fi

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
	@echo "Git repositories already configured via smart-git-servers..."
	@echo "Verifying git server accessibility..."
	kubectl --context kind-gitops-registration-test wait --for=condition=Ready pod -l app=git-servers -n git-servers --timeout=60s
	@echo "Setting up impersonation configuration for testing..."
	kubectl --context kind-gitops-registration-test apply -f test/integration/test-impersonation-clusterrole.yaml
	kubectl --context kind-gitops-registration-test patch configmap gitops-registration-config -n konflux-gitops --type merge -p='{"data":{"config.yaml":"server:\n  port: 8080\nargocd:\n  server: argocd-server.argocd.svc.cluster.local\n  namespace: argocd\nsecurity:\n  impersonation:\n    enabled: true\n    clusterRole: test-gitops-impersonator\n    serviceAccountBaseName: gitops-sa\n    validatePermissions: true\n    autoCleanup: true\nregistration:\n  allowNewNamespaces: true"}}'
	kubectl --context kind-gitops-registration-test patch clusterrole gitops-registration-controller --type='merge' -p='{"rules":[{"apiGroups":[""],"resources":["namespaces"],"verbs":["create","get","list","watch","update","patch","delete"]},{"apiGroups":[""],"resources":["serviceaccounts"],"verbs":["create","get","list","watch","update","patch","delete"]},{"apiGroups":["rbac.authorization.k8s.io"],"resources":["roles","rolebindings","clusterroles","clusterrolebindings"],"verbs":["create","get","list","watch","update","patch","delete"]},{"apiGroups":["rbac.authorization.k8s.io"],"resourceNames":["gitops-tenant-role"],"resources":["clusterroles"],"verbs":["bind"]},{"apiGroups":["rbac.authorization.k8s.io"],"resourceNames":["gitops-role"],"resources":["clusterroles"],"verbs":["bind"]},{"apiGroups":["rbac.authorization.k8s.io"],"resourceNames":["test-gitops-impersonator"],"resources":["clusterroles"],"verbs":["bind"]},{"apiGroups":["argoproj.io"],"resources":["appprojects","applications"],"verbs":["create","get","list","watch","update","patch","delete"]},{"apiGroups":[""],"resources":["resourcequotas","limitranges"],"verbs":["create","get","list","watch","update","patch","delete"]},{"apiGroups":[""],"resources":["configmaps","secrets","services"],"verbs":["get","list","watch","create","update","patch","delete"]},{"apiGroups":["apps"],"resources":["deployments","replicasets"],"verbs":["create","get","list","watch","update","patch","delete"]},{"apiGroups":["networking.k8s.io"],"resources":["ingresses"],"verbs":["create","get","list","watch","update","patch","delete"]},{"apiGroups":["authorization.k8s.io"],"resources":["subjectaccessreviews"],"verbs":["create"]},{"apiGroups":["authentication.k8s.io"],"resources":["tokenreviews"],"verbs":["create"]},{"apiGroups":[""],"resources":["events"],"verbs":["create","get","list","watch"]},{"apiGroups":[""],"resources":["nodes","persistentvolumes"],"verbs":["get","list","watch"]}]}'
	kubectl --context kind-gitops-registration-test delete pod -n konflux-gitops -l serving.knative.dev/service=gitops-registration || true
	@echo "Waiting for service to restart with impersonation configuration..."
	@sleep 10
	kubectl --context kind-gitops-registration-test wait --for=condition=ready pod -n konflux-gitops -l serving.knative.dev/service=gitops-registration --timeout=60s

run-integration-tests: ## Run the enhanced integration tests
	@echo "Running enhanced integration tests with Go..."
	@echo "Setting up kubectl port-forward for external access..."
	@DEPLOYMENT=$$(kubectl --context kind-gitops-registration-test get deployment -n konflux-gitops -l serving.knative.dev/service=gitops-registration --no-headers | grep -E '1/1|[1-9]+/[1-9]+' | head -1 | cut -d' ' -f1) && \
	echo "Using deployment: $$DEPLOYMENT" && \
	kubectl --context kind-gitops-registration-test port-forward -n konflux-gitops deployment/$$DEPLOYMENT 8080:8080 & \
	PORT_FORWARD_PID=$$! && \
	echo "Port-forward PID: $$PORT_FORWARD_PID"
	@echo "Waiting for port-forward to be ready..."
	@sleep 5
	@echo "Running Go integration tests..."
	@set -e; \
	export SERVICE_URL=http://localhost:8080 && \
	cd test/integration && \
	go mod tidy && \
	go test -v -timeout=30m -count=1 -tags=integration ./... && \
	echo "✅ Integration tests passed!" && \
	echo "Cleaning up port-forward..." && \
	(ps aux | grep "kubectl.*port-forward.*gitops-registration" | grep -v grep | awk '{print $$2}' | head -1 | xargs -r kill 2>/dev/null || true)

# KIND cluster management
setup-kind: ## Set up single-node KIND cluster
	@echo "Setting up KIND cluster..."
	cd test/integration && ./setup-kind-cluster.sh

teardown-kind: ## Tear down KIND cluster and clean up
	@echo "Tearing down KIND cluster..."
	@echo "Cleaning up any existing port-forwards..."
	@pkill -f "kubectl.*port-forward.*gitops-registration" || true
	@pkill -f "port-forward.*8080" || true
	@sleep 2
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

# Commit readiness targets
test-commit: ## Run all tests that GitHub Actions runs (full CI validation)
	@echo "=========================================="
	@echo "Running full commit validation (matches GitHub Actions)"
	@echo "=========================================="
	@echo "1. Checking Go formatting..."
	@if [ "$$(gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "❌ The following files are not formatted:"; \
		gofmt -s -l .; \
		echo ""; \
		echo "💡 Run 'gofmt -s -w .' to fix formatting issues"; \
		exit 1; \
	fi
	@echo "✅ All Go files are properly formatted"
	@echo "2. Checking go mod tidy..."
	@cp go.mod go.mod.bak
	@cp go.sum go.sum.bak
	@go mod tidy
	@if ! diff -q go.mod go.mod.bak || ! diff -q go.sum go.sum.bak; then \
		echo "❌ go.mod or go.sum is not tidy"; \
		echo ""; \
		echo "💡 Run 'go mod tidy' and commit the changes"; \
		git diff go.mod go.sum; \
		rm -f go.mod.bak go.sum.bak; \
		exit 1; \
	fi
	@rm -f go.mod.bak go.sum.bak
	@echo "✅ go.mod and go.sum are tidy"
	@echo "3. Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=2m; \
		echo "✅ Linting passed"; \
	else \
		echo "⚠️  golangci-lint not found, skipping lint check"; \
		echo "   Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi
	@echo "4. Building application..."
	@go build -v ./cmd/server
	@echo "✅ Build successful"
	@echo "5. Running unit tests with coverage..."
	@$(MAKE) test-coverage-check
	@echo "6. Running integration tests..."
	@$(MAKE) test-integration-full
	@echo "=========================================="
	@echo "✅ All commit validation tests passed!"
	@echo "Ready to commit and push 🚀"
	@echo "=========================================="

test-commit-fast: ## Run commit validation excluding integration tests (fast)
	@echo "=========================================="
	@echo "Running fast commit validation (no integration tests)"
	@echo "=========================================="
	@echo "1. Checking Go formatting..."
	@if [ "$$(gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "❌ The following files are not formatted:"; \
		gofmt -s -l .; \
		echo ""; \
		echo "💡 Run 'gofmt -s -w .' to fix formatting issues"; \
		exit 1; \
	fi
	@echo "✅ All Go files are properly formatted"
	@echo "2. Checking go mod tidy..."
	@cp go.mod go.mod.bak
	@cp go.sum go.sum.bak
	@go mod tidy
	@if ! diff -q go.mod go.mod.bak || ! diff -q go.sum go.sum.bak; then \
		echo "❌ go.mod or go.sum is not tidy"; \
		echo ""; \
		echo "💡 Run 'go mod tidy' and commit the changes"; \
		git diff go.mod go.sum; \
		rm -f go.mod.bak go.sum.bak; \
		exit 1; \
	fi
	@rm -f go.mod.bak go.sum.bak
	@echo "✅ go.mod and go.sum are tidy"
	@echo "3. Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=2m; \
		echo "✅ Linting passed"; \
	else \
		echo "⚠️  golangci-lint not found, skipping lint check"; \
		echo "   Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi
	@echo "4. Building application..."
	@go build -v ./cmd/server
	@echo "✅ Build successful"
	@echo "5. Running unit tests with coverage..."
	@$(MAKE) test-coverage-check
	@echo "=========================================="
	@echo "✅ Fast commit validation passed!"
	@echo "Ready for PR (integration tests will run in CI)"
	@echo "==========================================" 