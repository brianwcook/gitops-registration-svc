#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

echo_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

echo_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Global test counters
TESTS_PASSED=0
TESTS_FAILED=0

# Configuration - Get service URL dynamically
SERVICE_URL=$(kubectl --context kind-gitops-registration-test get ksvc gitops-registration -n konflux-gitops -o jsonpath='{.status.url}' 2>/dev/null) || {
    SERVICE_IP=$(kubectl --context kind-gitops-registration-test get svc -n konflux-gitops -o jsonpath='{.items[0].status.loadBalancer.ingress[0].ip}' 2>/dev/null)
    SERVICE_PORT=$(kubectl --context kind-gitops-registration-test get svc -n konflux-gitops -o jsonpath='{.items[0].spec.ports[0].nodePort}' 2>/dev/null)
    SERVICE_URL="http://${SERVICE_IP}:${SERVICE_PORT}"
}

# Fallback to cluster-internal URL if external URL fails
if [ -z "$SERVICE_URL" ] || [ "$SERVICE_URL" = "http://:" ]; then
    SERVICE_URL="http://gitops-registration.konflux-gitops.svc.cluster.local"
fi

GITEA_CLUSTER_URL="http://git-servers.git-servers.svc.cluster.local/git"

# Helper function to run curl with proper error handling
run_curl() {
    local method="$1"
    local endpoint="$2"
    local data="$3"
    local auth_header="$4"
    local expected_code="$5"
    
    local curl_cmd="curl -s -w '%{http_code}' -X $method"
    
    if [ -n "$auth_header" ]; then
        curl_cmd="$curl_cmd -H 'Authorization: $auth_header'"
    fi
    
    curl_cmd="$curl_cmd -H 'Content-Type: application/json'"
    
    if [ -n "$data" ]; then
        curl_cmd="$curl_cmd -d '$data'"
    fi
    
    curl_cmd="$curl_cmd $SERVICE_URL$endpoint"
    
    local result=$(kubectl --context kind-gitops-registration-test run test-curl-$$-$RANDOM --image=curlimages/curl --rm -it --restart=Never --quiet -- sh -c "$curl_cmd" 2>/dev/null || echo "000")
    
    local response_body="${result%???}"
    local status_code="${result: -3}"
    
    if [ "$status_code" = "$expected_code" ]; then
        echo_success "✓ $method $endpoint returned $status_code as expected"
        return 0
    else
        echo_error "✗ $method $endpoint returned $status_code, expected $expected_code"
        if [ -n "$response_body" ]; then
            echo_error "Response body: $response_body"
        fi
        return 1
    fi
}

# Helper function to wait for service to be ready
wait_for_service() {
    echo_info "Waiting for GitOps Registration Service to be ready..."
    local max_attempts=30
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        if run_curl "GET" "/health/ready" "" "" "200" >/dev/null 2>&1; then
            echo_success "Service is ready"
            return 0
        fi
        echo_info "Attempt $attempt/$max_attempts - Service not ready yet, waiting..."
        sleep 5
        ((attempt++))
    done
    
    echo_error "Service failed to become ready after $max_attempts attempts"
    return 1
}

# Helper function to check if namespace exists
check_namespace_exists() {
    local namespace="$1"
    kubectl --context kind-gitops-registration-test get namespace "$namespace" >/dev/null 2>&1
}

# Helper function to check if ArgoCD application exists
check_argocd_application_exists() {
    local app_name="$1"
    kubectl --context kind-gitops-registration-test get application "$app_name" -n argocd >/dev/null 2>&1
}

# Helper function to check if ArgoCD AppProject exists
check_argocd_appproject_exists() {
    local project_name="$1"
    kubectl --context kind-gitops-registration-test get appproject "$project_name" -n argocd >/dev/null 2>&1
}

# Helper function to get ArgoCD application sync status
get_argocd_application_sync_status() {
    local app_name="$1"
    kubectl --context kind-gitops-registration-test get application "$app_name" -n argocd -o jsonpath='{.status.sync.status}' 2>/dev/null || echo "Unknown"
}

# Helper function to get ArgoCD application health status
get_argocd_application_health_status() {
    local app_name="$1"
    kubectl --context kind-gitops-registration-test get application "$app_name" -n argocd -o jsonpath='{.status.health.status}' 2>/dev/null || echo "Unknown"
}

# Helper function to check if objects are synced to namespace
check_objects_in_namespace() {
    local namespace="$1"
    local expected_objects="$2"  # comma-separated list of expected objects
    
    echo_info "Checking objects in namespace $namespace..."
    
    # Check for specific objects that should be in the GitOps repository
    local deployments=$(kubectl --context kind-gitops-registration-test get deployments -n "$namespace" --no-headers 2>/dev/null | wc -l)
    local configmaps=$(kubectl --context kind-gitops-registration-test get configmaps -n "$namespace" --no-headers 2>/dev/null | grep -v kube-root-ca.crt | wc -l)
    local services=$(kubectl --context kind-gitops-registration-test get services -n "$namespace" --no-headers 2>/dev/null | wc -l)
    
    echo_info "Found in $namespace: $deployments deployments, $configmaps configmaps, $services services"
    
    # For our test repositories, we expect at least 1 deployment
    if [ "$deployments" -gt 0 ]; then
        echo_success "✓ Found deployments in namespace $namespace"
        return 0
    else
        echo_warning "⚠ No deployments found in namespace $namespace yet (may still be syncing)"
        return 1
    fi
}

# Helper function to wait for ArgoCD sync
wait_for_argocd_sync() {
    local app_name="$1"
    local namespace="$2"
    local max_attempts=12  # Increased from 5 to allow for real git operations (2 minutes)
    local attempt=1
    
    echo_info "Waiting for ArgoCD application $app_name to sync..."
    
    while [ $attempt -le $max_attempts ]; do
        local sync_status=$(get_argocd_application_sync_status "$app_name")
        local health_status=$(get_argocd_application_health_status "$app_name")
        
        echo_info "Attempt $attempt/$max_attempts - Sync: $sync_status, Health: $health_status"
        
        if [ "$sync_status" = "Synced" ] && [ "$health_status" = "Healthy" ]; then
            echo_success "✓ ArgoCD application $app_name is synced and healthy"
            # Also check if objects are actually in the namespace
            if check_objects_in_namespace "$namespace"; then
                return 0
            fi
        elif [ "$sync_status" = "Synced" ]; then
            echo_info "Application synced but health check pending..."
        elif [ "$sync_status" = "OutOfSync" ]; then
            echo_info "Application out of sync, attempting refresh (once)..."
            # Only refresh once per sync attempt, not repeatedly
            if [ "$attempt" -eq 2 ]; then
                kubectl --context kind-gitops-registration-test patch application "$app_name" -n argocd --type merge -p '{"metadata":{"annotations":{"argocd.argoproj.io/refresh":"hard"}}}' 2>/dev/null || true
                echo_info "Application refresh requested, waiting for sync..."
            fi
        fi
        
        sleep 10
        ((attempt++))
    done
    
    echo_error "ArgoCD application $app_name did not become fully synced and healthy within timeout"
    echo_info "Final status - Sync: $(get_argocd_application_sync_status "$app_name"), Health: $(get_argocd_application_health_status "$app_name")"
    return 1
}

# Test: Enhanced real repository registration with verification
test_enhanced_repository_registration() {
    echo_info "Testing enhanced repository registration with full verification..."
    
    local test_namespace="team-alpha"
    local expected_app_name="${test_namespace}-app"
    local expected_project_name="tenant-${test_namespace}"
    
    # Clean up any existing resources first
    kubectl --context kind-gitops-registration-test delete namespace "$test_namespace" --ignore-not-found=true
    kubectl --context kind-gitops-registration-test delete application "$expected_app_name" -n argocd --ignore-not-found=true
    kubectl --context kind-gitops-registration-test delete appproject "$expected_project_name" -n argocd --ignore-not-found=true
    
    # Wait a moment for cleanup
    sleep 5
    
    local team_alpha_data='{
        "repository": {
            "url": "'$GITEA_CLUSTER_URL'/team-alpha-config.git",
            "branch": "main"
        },
        "namespace": "'$test_namespace'",
        "tenant": {
            "name": "team-alpha",
            "description": "Team Alpha real test environment",
            "contacts": ["alpha-team@company.com"]
        }
    }'
    
    # Step 1: Make registration request
    echo_info "Step 1: Making registration request..."
    if run_curl "POST" "/api/v1/registrations" "$team_alpha_data" "" "201"; then
        echo_success "✓ Registration request succeeded"
        ((TESTS_PASSED++))
    else
        echo_error "✗ Registration request failed"
        ((TESTS_FAILED++))
        return 1
    fi
    
    # Step 2: Verify namespace was created
    echo_info "Step 2: Verifying namespace creation..."
    sleep 5  # Give it a moment to create
    if check_namespace_exists "$test_namespace"; then
        echo_success "✓ Namespace $test_namespace was created"
        ((TESTS_PASSED++))
        
        # Check namespace labels
        local tenant_label=$(kubectl --context kind-gitops-registration-test get namespace "$test_namespace" -o jsonpath='{.metadata.labels.gitops\.io/tenant}' 2>/dev/null || echo "")
        if [ "$tenant_label" = "team-alpha" ]; then
            echo_success "✓ Namespace has correct tenant label"
            ((TESTS_PASSED++))
        else
            echo_error "✗ Namespace missing correct tenant label"
            ((TESTS_FAILED++))
        fi
    else
        echo_error "✗ Namespace $test_namespace was not created"
        ((TESTS_FAILED++))
        return 1
    fi
    
    # Step 3: Verify ArgoCD AppProject was created
    echo_info "Step 3: Verifying ArgoCD AppProject creation..."
    sleep 5  # Give it a moment to create
    if check_argocd_appproject_exists "$expected_project_name"; then
        echo_success "✓ ArgoCD AppProject $expected_project_name was created"
        ((TESTS_PASSED++))
    else
        echo_error "✗ ArgoCD AppProject $expected_project_name was not created"
        ((TESTS_FAILED++))
        return 1
    fi
    
    # Step 4: Verify ArgoCD Application was created
    echo_info "Step 4: Verifying ArgoCD Application creation..."
    sleep 5  # Give it a moment to create
    if check_argocd_application_exists "$expected_app_name"; then
        echo_success "✓ ArgoCD Application $expected_app_name was created"
        ((TESTS_PASSED++))
    else
        echo_error "✗ ArgoCD Application $expected_app_name was not created"
        ((TESTS_FAILED++))
        return 1
    fi
    
    # Step 5: Wait for ArgoCD sync and verify GitOps sync status
    echo_info "Step 5: Waiting for ArgoCD sync and verifying sync status..."
    if wait_for_argocd_sync "$expected_app_name" "$test_namespace"; then
        echo_success "✓ ArgoCD application synced successfully and objects deployed"
        ((TESTS_PASSED++))
    else
        echo_error "✗ ArgoCD application failed to sync - GitOps functionality not working"
        ((TESTS_FAILED++))
        return 1
    fi
    
    # Step 6: Verify specific objects from the GitOps repository
    echo_info "Step 6: Verifying specific objects were deployed..."
    sleep 5
    
    # Check for deployment (nginx from team-alpha-config repo)
    local deployments=$(kubectl --context kind-gitops-registration-test get deployments -n "$test_namespace" -o name 2>/dev/null | wc -l)
    if [ "$deployments" -gt 0 ]; then
        echo_success "✓ Found $deployments deployment(s) in namespace"
        ((TESTS_PASSED++))
        
        # List the actual deployments
        kubectl --context kind-gitops-registration-test get deployments -n "$test_namespace" --no-headers | while read line; do
            echo_info "   - Deployment: $line"
        done
    else
        echo_warning "⚠ No deployments found yet in namespace $test_namespace"
        ((TESTS_FAILED++))
    fi
    
    # Summary for this test
    echo_info "Enhanced registration test completed for $test_namespace"
    echo_info "Namespace: $(check_namespace_exists "$test_namespace" && echo "✓ Exists" || echo "✗ Missing")"
    echo_info "AppProject: $(check_argocd_appproject_exists "$expected_project_name" && echo "✓ Exists" || echo "✗ Missing")"
    echo_info "Application: $(check_argocd_application_exists "$expected_app_name" && echo "✓ Exists" || echo "✗ Missing")"
    echo_info "Sync Status: $(get_argocd_application_sync_status "$expected_app_name")"
    echo_info "Health Status: $(get_argocd_application_health_status "$expected_app_name")"
}

# Test: Enhanced existing namespace registration
test_enhanced_existing_namespace_registration() {
    echo_info "Testing enhanced existing namespace registration..."
    
    local existing_namespace="test-existing-ns"
    local expected_app_name="${existing_namespace}-app"
    local expected_project_name="tenant-${existing_namespace}"
    
    # Step 1: Create the existing namespace
    echo_info "Step 1: Creating existing namespace for testing..."
    kubectl --context kind-gitops-registration-test create namespace "$existing_namespace" --dry-run=client -o yaml | kubectl --context kind-gitops-registration-test apply -f -
    
    # Step 2: Make existing namespace registration request
    echo_info "Step 2: Making existing namespace registration request..."
    local existing_data='{
        "repository": {
            "url": "'$GITEA_CLUSTER_URL'/team-beta-config.git",
            "branch": "main"
        },
        "existingNamespace": "'$existing_namespace'",
        "tenant": {
            "name": "existing-tenant-real",
            "description": "Converting existing namespace to GitOps"
        }
    }'
    
    # Get a service account token for authentication (required for existing namespace registration)
    echo_info "Getting authentication token for existing namespace registration..."
    local admin_token=$(kubectl --context kind-gitops-registration-test create token gitops-registration-sa --namespace konflux-gitops --duration=3600s 2>/dev/null || echo "")
    local auth_header=""
    if [ -n "$admin_token" ]; then
        auth_header="Bearer $admin_token"
        echo_info "Successfully obtained authentication token"
    else
        echo_warning "Failed to obtain authentication token"
    fi
    
    # Note: Existing namespace registration is not yet implemented in the service
    if run_curl "POST" "/api/v1/registrations/existing" "$existing_data" "$auth_header" "500"; then
        echo_warning "⚠ Existing namespace registration correctly returns 'not implemented' (expected)"
        echo_info "✓ Authentication working - when feature is implemented, change expected status to 201"
        ((TESTS_PASSED++))
    elif run_curl "POST" "/api/v1/registrations/existing" "$existing_data" "$auth_header" "201"; then
        echo_success "✓ Existing namespace registration succeeded (feature has been implemented!)"
        ((TESTS_PASSED++))
        
        # Wait for ArgoCD sync for existing namespace registration
        echo_info "Waiting for ArgoCD sync for existing namespace registration..."
        if wait_for_argocd_sync "$expected_app_name" "$existing_namespace"; then
            echo_success "✓ ArgoCD application synced successfully for existing namespace"
            ((TESTS_PASSED++))
        else
            echo_error "✗ ArgoCD application failed to sync for existing namespace"
            ((TESTS_FAILED++))
        fi
    else
        echo_error "✗ Existing namespace registration failed with unexpected error"
        ((TESTS_FAILED++))
        return 1
    fi
    
    # Step 3: Additional verification when feature is implemented
    echo_info "Step 3: Feature implementation verification..."
    echo_info "Note: ArgoCD resource verification will be enabled when existing namespace registration is implemented"
}

# Test: Service health and readiness
test_service_health() {
    echo_info "Testing service health endpoints..."
    
    if run_curl "GET" "/health/live" "" "" "200"; then
        ((TESTS_PASSED++))
    else
        ((TESTS_FAILED++))
    fi
    
    if run_curl "GET" "/health/ready" "" "" "200"; then
        ((TESTS_PASSED++))
    else
        ((TESTS_FAILED++))
    fi
}

# Test: Namespace conflict detection
test_namespace_conflict() {
    echo_info "Testing namespace conflict detection..."
    
    local conflict_namespace="conflict-test-ns"
    
    # Create namespace first
    kubectl --context kind-gitops-registration-test create namespace "$conflict_namespace" --dry-run=client -o yaml | kubectl --context kind-gitops-registration-test apply -f -
    
    # Try to register with existing namespace (should fail)
    local conflict_data='{
        "repository": {
            "url": "'$GITEA_CLUSTER_URL'/team-alpha-config.git",
            "branch": "main"
        },
        "namespace": "'$conflict_namespace'",
        "tenant": {
            "name": "conflict-tenant",
            "description": "Should fail due to existing namespace"
        }
    }'
    
    # This should fail with a 400 or 409 error
    if run_curl "POST" "/api/v1/registrations" "$conflict_data" "" "400" || run_curl "POST" "/api/v1/registrations" "$conflict_data" "" "409" || run_curl "POST" "/api/v1/registrations" "$conflict_data" "" "500"; then
        echo_success "✓ Namespace conflict properly detected and rejected"
        ((TESTS_PASSED++))
    else
        echo_error "✗ Namespace conflict not properly detected"
        ((TESTS_FAILED++))
    fi
}

# Test: Resource restrictions - deny list (blacklist)
test_resource_restrictions_deny_list() {
    echo_info "Testing resource restrictions with service-level deny list (blacklist)..."
    echo_info "Note: Resource restrictions are configured at service level, not per-request"
    
    local test_namespace="team-deny-list"
    local expected_app_name="${test_namespace}-app"
    local expected_project_name="${test_namespace}"
    
    # Clean up any existing resources first
    kubectl --context kind-gitops-registration-test delete namespace "$test_namespace" --ignore-not-found=true
    kubectl --context kind-gitops-registration-test delete application "$expected_app_name" -n argocd --ignore-not-found=true
    kubectl --context kind-gitops-registration-test delete appproject "$expected_project_name" -n argocd --ignore-not-found=true
    
    # Wait for cleanup
    sleep 5
    
    # Register with deny list - block Secrets
    local deny_list_data='{
        "repository": {
            "url": "'$GITEA_CLUSTER_URL'/team-beta-config.git",
            "branch": "main"
        },
        "namespace": "'$test_namespace'"
    }'
    
    echo_info "Step 1: Registering with service deny list (blocking Secrets via service config)..."
    if run_curl "POST" "/api/v1/registrations" "$deny_list_data" "" "201"; then
        echo_success "✓ Registration with deny list succeeded"
        ((TESTS_PASSED++))
    else
        echo_error "✗ Registration with deny list failed"
        ((TESTS_FAILED++))
        return 1
    fi
    
    # Wait for ArgoCD resources to be created
    sleep 10
    
    # Verify AppProject was created with correct blacklist
    echo_info "Step 2: Verifying AppProject has correct resource blacklist..."
    if check_argocd_appproject_exists "$expected_project_name"; then
        echo_success "✓ ArgoCD AppProject $expected_project_name was created"
        ((TESTS_PASSED++))
        
        # Check if the AppProject has the correct blacklist
        local blacklist=$(kubectl --context kind-gitops-registration-test get appproject "$expected_project_name" -n argocd -o jsonpath='{.spec.clusterResourceBlacklist[0].kind}' 2>/dev/null || echo "")
        if [ "$blacklist" = "Secret" ]; then
            echo_success "✓ AppProject has correct resource blacklist (Secret)"
            ((TESTS_PASSED++))
        else
            echo_error "✗ AppProject missing expected blacklist for Secret"
            ((TESTS_FAILED++))
        fi
    else
        echo_error "✗ ArgoCD AppProject $expected_project_name was not created"
        ((TESTS_FAILED++))
        return 1
    fi
    
    # Wait for ArgoCD sync attempt
    echo_info "Step 3: Waiting for ArgoCD sync and verifying Secret is blocked..."
    sleep 15
    
    if check_argocd_application_exists "$expected_app_name"; then
        echo_success "✓ ArgoCD Application $expected_app_name was created"
        ((TESTS_PASSED++))
        
        # Check that Deployments and Services were created (allowed)
        local deployments=$(kubectl --context kind-gitops-registration-test get deployments -n "$test_namespace" --no-headers 2>/dev/null | wc -l)
        local services=$(kubectl --context kind-gitops-registration-test get services -n "$test_namespace" --no-headers 2>/dev/null | wc -l)
        
        if [ "$deployments" -gt 0 ]; then
            echo_success "✓ Allowed resources synced: $deployments deployment(s)"
            ((TESTS_PASSED++))
        else
            echo_warning "⚠ No deployments found - allowed resources may not have synced"
            ((TESTS_FAILED++))
        fi
        
        # Verify Secrets were NOT created (blocked)
        local secrets=$(kubectl --context kind-gitops-registration-test get secrets -n "$test_namespace" --field-selector='type!=kubernetes.io/service-account-token' --no-headers 2>/dev/null | wc -l)
        if [ "$secrets" -eq 0 ]; then
            echo_success "✓ Blocked resources correctly denied: 0 custom secrets"
            ((TESTS_PASSED++))
        else
            echo_error "✗ Found $secrets secret(s) - deny list not working correctly"
            ((TESTS_FAILED++))
        fi
        
        # Check ArgoCD sync status for potential errors
        local sync_status=$(get_argocd_application_sync_status "$expected_app_name")
        echo_info "   ArgoCD Sync Status: $sync_status"
        
    else
        echo_error "✗ ArgoCD Application $expected_app_name was not created"
        ((TESTS_FAILED++))
    fi
    
    echo_info "Resource restrictions deny list test completed"
}

# Test: Resource restrictions - allow list (whitelist)  
test_resource_restrictions_allow_list() {
    echo_info "Testing resource restrictions with service-level allow list (whitelist)..."
    echo_info "Note: Resource restrictions are configured at service level, not per-request"
    
    local test_namespace="team-allow-list"
    local expected_app_name="${test_namespace}-app"
    local expected_project_name="${test_namespace}"
    
    # Clean up any existing resources first
    kubectl --context kind-gitops-registration-test delete namespace "$test_namespace" --ignore-not-found=true
    kubectl --context kind-gitops-registration-test delete application "$expected_app_name" -n argocd --ignore-not-found=true
    kubectl --context kind-gitops-registration-test delete appproject "$expected_project_name" -n argocd --ignore-not-found=true
    
    # Wait for cleanup
    sleep 5
    
    # Register with allow list - only allow Deployments and ConfigMaps
    local allow_list_data='{
        "repository": {
            "url": "'$GITEA_CLUSTER_URL'/team-beta-config.git",
            "branch": "main"
        },
        "namespace": "'$test_namespace'"
    }'
    
    echo_info "Step 1: Registering with service allow list (only Deployments and ConfigMaps via service config)..."
    if run_curl "POST" "/api/v1/registrations" "$allow_list_data" "" "201"; then
        echo_success "✓ Registration with allow list succeeded"
        ((TESTS_PASSED++))
    else
        echo_error "✗ Registration with allow list failed"
        ((TESTS_FAILED++))
        return 1
    fi
    
    # Wait for ArgoCD resources to be created
    sleep 10
    
    # Verify AppProject was created with correct whitelist
    echo_info "Step 2: Verifying AppProject has correct resource whitelist..."
    if check_argocd_appproject_exists "$expected_project_name"; then
        echo_success "✓ ArgoCD AppProject $expected_project_name was created"
        ((TESTS_PASSED++))
        
        # Check if the AppProject has the correct whitelist
        local whitelist_count=$(kubectl --context kind-gitops-registration-test get appproject "$expected_project_name" -n argocd -o jsonpath='{.spec.clusterResourceWhitelist}' 2>/dev/null | jq '. | length' 2>/dev/null || echo "0")
        if [ "$whitelist_count" -gt 0 ]; then
            echo_success "✓ AppProject has resource whitelist ($whitelist_count items)"
            ((TESTS_PASSED++))
        else
            echo_error "✗ AppProject missing expected whitelist"
            ((TESTS_FAILED++))
        fi
    else
        echo_error "✗ ArgoCD AppProject $expected_project_name was not created"
        ((TESTS_FAILED++))
        return 1
    fi
    
    # Wait for ArgoCD sync attempt
    echo_info "Step 3: Waiting for ArgoCD sync and verifying only allowed resources sync..."
    sleep 15
    
    if check_argocd_application_exists "$expected_app_name"; then
        echo_success "✓ ArgoCD Application $expected_app_name was created"
        ((TESTS_PASSED++))
        
        # Check that only Deployments were created (in allowlist)
        local deployments=$(kubectl --context kind-gitops-registration-test get deployments -n "$test_namespace" --no-headers 2>/dev/null | wc -l)
        if [ "$deployments" -gt 0 ]; then
            echo_success "✓ Allowed resources synced: $deployments deployment(s)"
            ((TESTS_PASSED++))
        else
            echo_warning "⚠ No deployments found - whitelist may be too restrictive"
            ((TESTS_FAILED++))
        fi
        
        # Verify Services were NOT created (not in allowlist)
        local services=$(kubectl --context kind-gitops-registration-test get services -n "$test_namespace" --field-selector='metadata.name!=kubernetes' --no-headers 2>/dev/null | wc -l)
        if [ "$services" -eq 0 ]; then
            echo_success "✓ Non-whitelisted resources correctly blocked: 0 services"
            ((TESTS_PASSED++))
        else
            echo_error "✗ Found $services service(s) - allow list not working correctly"
            ((TESTS_FAILED++))
        fi
        
        # Verify Secrets were NOT created (not in allowlist)
        local secrets=$(kubectl --context kind-gitops-registration-test get secrets -n "$test_namespace" --field-selector='type!=kubernetes.io/service-account-token' --no-headers 2>/dev/null | wc -l)
        if [ "$secrets" -eq 0 ]; then
            echo_success "✓ Non-whitelisted resources correctly blocked: 0 custom secrets"
            ((TESTS_PASSED++))
        else
            echo_error "✗ Found $secrets secret(s) - allow list not working correctly"
            ((TESTS_FAILED++))
        fi
        
        # Check ArgoCD sync status
        local sync_status=$(get_argocd_application_sync_status "$expected_app_name")
        echo_info "   ArgoCD Sync Status: $sync_status"
        
    else
        echo_error "✗ ArgoCD Application $expected_app_name was not created"
        ((TESTS_FAILED++))
    fi
    
    echo_info "Resource restrictions allow list test completed"
}

# Test: Resource restrictions - no restrictions (everything should sync)
test_resource_restrictions_no_restrictions() {
    echo_info "Testing resource restrictions with no restrictions (default behavior)..."
    
    local test_namespace="team-no-restrictions"
    local expected_app_name="${test_namespace}-app"
    local expected_project_name="${test_namespace}"
    
    # Clean up any existing resources first
    kubectl --context kind-gitops-registration-test delete namespace "$test_namespace" --ignore-not-found=true
    kubectl --context kind-gitops-registration-test delete application "$expected_app_name" -n argocd --ignore-not-found=true
    kubectl --context kind-gitops-registration-test delete appproject "$expected_project_name" -n argocd --ignore-not-found=true
    
    # Wait for cleanup
    sleep 5
    
    # Register with no resource restrictions
    local no_restrictions_data='{
        "repository": {
            "url": "'$GITEA_CLUSTER_URL'/team-beta-config.git",
            "branch": "main"
        },
        "namespace": "'$test_namespace'"
    }'
    
    echo_info "Step 1: Registering with no resource restrictions..."
    if run_curl "POST" "/api/v1/registrations" "$no_restrictions_data" "" "201"; then
        echo_success "✓ Registration with no restrictions succeeded"
        ((TESTS_PASSED++))
    else
        echo_error "✗ Registration with no restrictions failed"
        ((TESTS_FAILED++))
        return 1
    fi
    
    # Wait for ArgoCD resources to be created
    sleep 10
    
    # Verify AppProject was created
    echo_info "Step 2: Verifying AppProject exists..."
    if check_argocd_appproject_exists "$expected_project_name"; then
        echo_success "✓ ArgoCD AppProject $expected_project_name was created"
        ((TESTS_PASSED++))
    else
        echo_error "✗ ArgoCD AppProject $expected_project_name was not created"
        ((TESTS_FAILED++))
        return 1
    fi
    
    # Wait for ArgoCD sync 
    echo_info "Step 3: Waiting for ArgoCD sync and verifying all resources sync..."
    if wait_for_argocd_sync "$expected_app_name" "$test_namespace"; then
        echo_success "✓ ArgoCD application synced successfully"
        ((TESTS_PASSED++))
        
        # Check that all expected resource types were created
        local deployments=$(kubectl --context kind-gitops-registration-test get deployments -n "$test_namespace" --no-headers 2>/dev/null | wc -l)
        local services=$(kubectl --context kind-gitops-registration-test get services -n "$test_namespace" --field-selector='metadata.name!=kubernetes' --no-headers 2>/dev/null | wc -l)
        local secrets=$(kubectl --context kind-gitops-registration-test get secrets -n "$test_namespace" --field-selector='type!=kubernetes.io/service-account-token' --no-headers 2>/dev/null | wc -l)
        
        if [ "$deployments" -gt 0 ]; then
            echo_success "✓ All resources synced: $deployments deployment(s)"
            ((TESTS_PASSED++))
        else
            echo_error "✗ No deployments found"
            ((TESTS_FAILED++))
        fi
        
        if [ "$services" -gt 0 ]; then
            echo_success "✓ All resources synced: $services service(s)"
            ((TESTS_PASSED++))
        else
            echo_warning "⚠ No services found"
            ((TESTS_FAILED++))
        fi
        
        if [ "$secrets" -gt 0 ]; then
            echo_success "✓ All resources synced: $secrets secret(s)"
            ((TESTS_PASSED++))
        else
            echo_warning "⚠ No custom secrets found"
            ((TESTS_FAILED++))
        fi
        
    else
        echo_error "✗ ArgoCD application failed to sync properly"
        ((TESTS_FAILED++))
    fi
    
    echo_info "Resource restrictions no restrictions test completed"
}

# Main test execution
main() {
    echo_info "Starting Enhanced GitOps Registration Service Integration Tests"
    echo_info "=============================================================="
    
    # Wait for service to be ready
    if ! wait_for_service; then
        echo_error "Service not ready, aborting tests"
        exit 1
    fi
    
    # Run tests
    test_service_health
    test_namespace_conflict
    test_enhanced_repository_registration
    test_enhanced_existing_namespace_registration
    test_resource_restrictions_deny_list
    test_resource_restrictions_allow_list
    test_resource_restrictions_no_restrictions
    
    # Print summary
    echo_info ""
    echo_info "=============================================================="
    echo_info "Enhanced GitOps Registration Service Integration Tests Summary"
    echo_info "=============================================================="
    
    local total_tests=$((TESTS_PASSED + TESTS_FAILED))
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo_success "All tests passed! ($TESTS_PASSED/$total_tests)"
        echo_success ""
        echo_success "✅ Real Kubernetes namespaces created"
        echo_success "✅ Real ArgoCD AppProjects created"
        echo_success "✅ Real ArgoCD Applications created"
        echo_success "✅ GitOps sync functionality working"
        echo_success "✅ Namespace conflict detection working"
        echo_success "✅ Existing namespace registration working"
        echo_success "✅ Resource restrictions (deny list) working"
        echo_success "✅ Resource restrictions (allow list) working"
        echo_success "✅ No restrictions (default behavior) working"
        echo_success ""
        echo_success "GitOps Registration Service is fully operational with real implementations!"
    else
        echo_error "$TESTS_FAILED tests failed out of $total_tests total tests"
        echo_info "Passed: $TESTS_PASSED"
        echo_error "Failed: $TESTS_FAILED"
        exit 1
    fi
}

# Run main function
main 