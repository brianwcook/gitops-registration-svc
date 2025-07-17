#!/bin/bash

# Test script for disabled registration functionality
# This tests the ALLOW_NEW_NAMESPACES configuration

set -euo pipefail

# Source common functions from existing test files
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/run-tests.sh"

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

# Configuration
SERVICE_URL="http://localhost:8080"
GITEA_CLUSTER_URL="http://git-servers.git-servers.svc.cluster.local/git"

echo_info() {
    echo "🔵 $1"
}

echo_success() {
    echo "✅ $1"
}

echo_error() {
    echo "❌ $1"
}

echo_warning() {
    echo "⚠️  $1"
}

# Test: Registration enabled (default state)
test_registration_enabled() {
    echo_info "Testing registration when ALLOW_NEW_NAMESPACES=true (enabled)..."
    
    # Deploy service with registration enabled
    echo_info "Step 1: Deploying service with registration enabled..."
    kubectl --context kind-gitops-registration-test patch deployment gitops-registration-service -n default --type='merge' -p='{"spec":{"template":{"spec":{"containers":[{"name":"gitops-registration-service","env":[{"name":"ALLOW_NEW_NAMESPACES","value":"true"}]}]}}}}'
    
    # Wait for rollout
    kubectl --context kind-gitops-registration-test rollout status deployment/gitops-registration-service -n default --timeout=60s
    sleep 10
    
    # Test registration request
    local test_data='{
        "repository": {
            "url": "'$GITEA_CLUSTER_URL'/team-alpha-config.git",
            "branch": "main"
        },
        "namespace": "test-enabled-registration"
    }'
    
    echo_info "Step 2: Attempting registration when enabled..."
    local response_code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$SERVICE_URL/api/v1/registrations" \
        -H "Content-Type: application/json" \
        -d "$test_data" 2>/dev/null || echo "000")
    
    if [ "$response_code" = "201" ]; then
        echo_success "✓ Registration succeeded when enabled (HTTP 201)"
        ((TESTS_PASSED++))
    else
        echo_error "✗ Registration failed when enabled (HTTP $response_code)"
        ((TESTS_FAILED++))
    fi
    
    # Clean up test namespace
    kubectl --context kind-gitops-registration-test delete namespace "test-enabled-registration" --ignore-not-found=true
}

# Test: Registration disabled
test_registration_disabled() {
    echo_info "Testing registration when ALLOW_NEW_NAMESPACES=false (disabled)..."
    
    # Deploy service with registration disabled
    echo_info "Step 1: Deploying service with registration disabled..."
    kubectl --context kind-gitops-registration-test patch deployment gitops-registration-service -n default --type='merge' -p='{"spec":{"template":{"spec":{"containers":[{"name":"gitops-registration-service","env":[{"name":"ALLOW_NEW_NAMESPACES","value":"false"}]}]}}}}'
    
    # Wait for rollout
    kubectl --context kind-gitops-registration-test rollout status deployment/gitops-registration-service -n default --timeout=60s
    sleep 10
    
    # Test registration request
    local test_data='{
        "repository": {
            "url": "'$GITEA_CLUSTER_URL'/team-alpha-config.git", 
            "branch": "main"
        },
        "namespace": "test-disabled-registration"
    }'
    
    echo_info "Step 2: Attempting registration when disabled..."
    local response_code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$SERVICE_URL/api/v1/registrations" \
        -H "Content-Type: application/json" \
        -d "$test_data" 2>/dev/null || echo "000")
    
    if [ "$response_code" = "503" ]; then
        echo_success "✓ Registration correctly rejected when disabled (HTTP 503)"
        ((TESTS_PASSED++))
    else
        echo_error "✗ Registration should have been rejected with HTTP 503, got HTTP $response_code"
        ((TESTS_FAILED++))
    fi
    
    # Test the error response body
    echo_info "Step 3: Verifying error response contains REGISTRATION_DISABLED..."
    local response_body=$(curl -s -X POST "$SERVICE_URL/api/v1/registrations" \
        -H "Content-Type: application/json" \
        -d "$test_data" 2>/dev/null || echo "{}")
    
    if echo "$response_body" | grep -q "REGISTRATION_DISABLED"; then
        echo_success "✓ Error response contains REGISTRATION_DISABLED"
        ((TESTS_PASSED++))
    else
        echo_error "✗ Error response missing REGISTRATION_DISABLED: $response_body"
        ((TESTS_FAILED++))
    fi
    
    if echo "$response_body" | grep -q "currently disabled"; then
        echo_success "✓ Error message contains 'currently disabled'"
        ((TESTS_PASSED++))
    else
        echo_error "✗ Error message missing 'currently disabled' text"
        ((TESTS_FAILED++))
    fi
}

# Test: Registration status endpoint
test_registration_status_endpoint() {
    echo_info "Testing registration status endpoint..."
    
    # Test status when disabled
    echo_info "Step 1: Checking status when registration is disabled..."
    local status_response=$(curl -s "$SERVICE_URL/api/v1/registrations/status" 2>/dev/null || echo "{}")
    
    if echo "$status_response" | grep -q '"allowNewNamespaces":false'; then
        echo_success "✓ Status endpoint shows allowNewNamespaces: false"
        ((TESTS_PASSED++))
    else
        echo_error "✗ Status endpoint should show allowNewNamespaces: false"
        ((TESTS_FAILED++))
    fi
    
    # Re-enable registration and test status
    echo_info "Step 2: Re-enabling registration and checking status..."
    kubectl --context kind-gitops-registration-test patch deployment gitops-registration-service -n default --type='merge' -p='{"spec":{"template":{"spec":{"containers":[{"name":"gitops-registration-service","env":[{"name":"ALLOW_NEW_NAMESPACES","value":"true"}]}]}}}}'
    
    # Wait for rollout
    kubectl --context kind-gitops-registration-test rollout status deployment/gitops-registration-service -n default --timeout=60s
    sleep 10
    
    local status_response_enabled=$(curl -s "$SERVICE_URL/api/v1/registrations/status" 2>/dev/null || echo "{}")
    
    if echo "$status_response_enabled" | grep -q '"allowNewNamespaces":true'; then
        echo_success "✓ Status endpoint shows allowNewNamespaces: true when re-enabled"
        ((TESTS_PASSED++))
    else
        echo_error "✗ Status endpoint should show allowNewNamespaces: true when re-enabled"
        ((TESTS_FAILED++))
    fi
}

# Wait for service to be ready
wait_for_service() {
    local max_attempts=30
    local attempt=0
    
    echo_info "Waiting for GitOps Registration Service to be ready..."
    
    while [ $attempt -lt $max_attempts ]; do
        if curl -s "$SERVICE_URL/health/ready" > /dev/null 2>&1; then
            echo_success "Service is ready"
            return 0
        fi
        
        attempt=$((attempt + 1))
        echo_info "Attempt $attempt/$max_attempts: Service not ready yet, waiting..."
        sleep 5
    done
    
    echo_error "Service failed to become ready after $max_attempts attempts"
    return 1
}

# Main test execution
main() {
    echo_info "=============================================================="
    echo_info "GitOps Registration Service - Disabled Registration Tests"
    echo_info "=============================================================="
    
    # Wait for service to be ready
    if ! wait_for_service; then
        echo_error "Service not ready, aborting tests"
        exit 1
    fi
    
    # Run tests
    test_registration_enabled
    test_registration_disabled
    test_registration_status_endpoint
    
    # Print summary
    echo_info ""
    echo_info "=============================================================="
    echo_info "Disabled Registration Tests Summary"
    echo_info "=============================================================="
    
    local total_tests=$((TESTS_PASSED + TESTS_FAILED))
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo_success "All tests passed! ($TESTS_PASSED/$total_tests)"
        echo_success ""
        echo_success "✅ Registration enabled functionality working"
        echo_success "✅ Registration disabled functionality working"
        echo_success "✅ Registration status endpoint working"
        echo_success "✅ Error responses formatted correctly"
        echo_success ""
        echo_success "Disabled registration feature is working correctly!"
    else
        echo_error "$TESTS_FAILED tests failed out of $total_tests total tests"
        echo_info "Passed: $TESTS_PASSED"
        echo_error "Failed: $TESTS_FAILED"
        exit 1
    fi
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi 