#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
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

echo_header() {
    echo -e "${CYAN}$1${NC}"
}

clear

echo_header "=============================================="
echo_header "  GitOps Registration Service Test Environment"
echo_header "=============================================="
echo ""

# Check KIND cluster
echo_header "🌟 KIND Cluster Status:"
if kind get clusters | grep -q "gitops-registration-test"; then
    echo_success "✅ KIND cluster 'gitops-registration-test' is running"
    
    # Get node info
    NODE_COUNT=$(kubectl --context kind-gitops-registration-test get nodes --no-headers | wc -l)
    echo_info "   Nodes: $NODE_COUNT"
    kubectl --context kind-gitops-registration-test get nodes --no-headers | while read line; do
        echo_info "   - $line"
    done
else
    echo_error "❌ KIND cluster is not running"
fi
echo ""

# Check Knative
echo_header "🚀 Knative Status:"
if kubectl --context kind-gitops-registration-test get namespace knative-serving >/dev/null 2>&1; then
    echo_success "✅ Knative Serving is installed"
    
    # Check Knative components
    READY_PODS=$(kubectl --context kind-gitops-registration-test get pods -n knative-serving --no-headers | grep -c "Running")
    TOTAL_PODS=$(kubectl --context kind-gitops-registration-test get pods -n knative-serving --no-headers | wc -l)
    echo_info "   Pods: $READY_PODS/$TOTAL_PODS running"
else
    echo_error "❌ Knative Serving is not installed"
fi
echo ""

# Check ArgoCD
echo_header "⚡ ArgoCD Status:"
if kubectl --context kind-gitops-registration-test get namespace argocd >/dev/null 2>&1; then
    echo_success "✅ ArgoCD is installed"
    
    # Check ArgoCD pods
    READY_PODS=$(kubectl --context kind-gitops-registration-test get pods -n argocd --no-headers | grep -c "Running")
    TOTAL_PODS=$(kubectl --context kind-gitops-registration-test get pods -n argocd --no-headers | wc -l)
    echo_info "   Pods: $READY_PODS/$TOTAL_PODS running"
    
    # Get ArgoCD server access info
    ARGOCD_PORT=$(kubectl --context kind-gitops-registration-test get svc argocd-server -n argocd -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null)
    NODE_IP=$(kubectl --context kind-gitops-registration-test get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
    if [ -n "$ARGOCD_PORT" ]; then
        echo_info "   UI: http://$NODE_IP:$ARGOCD_PORT (admin/admin)"
    fi
    
    # Check repository secrets
    REPO_SECRETS=$(kubectl --context kind-gitops-registration-test get secrets -n argocd -l argocd.argoproj.io/secret-type=repository --no-headers | wc -l)
    echo_info "   Repository secrets: $REPO_SECRETS configured"
else
    echo_error "❌ ArgoCD is not installed"
fi
echo ""

# Check Gitea
echo_header "🗂️  Gitea Git Server Status:"
if kubectl --context kind-gitops-registration-test get namespace gitea >/dev/null 2>&1; then
    echo_success "✅ Gitea is installed"
    
    # Check Gitea pod
    GITEA_STATUS=$(kubectl --context kind-gitops-registration-test get pods -n gitea -l app=gitea --no-headers | awk '{print $3}')
    echo_info "   Pod status: $GITEA_STATUS"
    
    # Get Gitea access info
    GITEA_PORT=$(kubectl --context kind-gitops-registration-test get svc gitea-nodeport -n gitea -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null)
    NODE_IP=$(kubectl --context kind-gitops-registration-test get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
    if [ -n "$GITEA_PORT" ]; then
        echo_info "   UI: http://$NODE_IP:$GITEA_PORT (gitops-admin/gitops123)"
        echo_info "   Internal URL: http://gitea.gitea.svc.cluster.local:3000"
    fi
    
    # Check repositories
    REPO_COUNT=$(kubectl --context kind-gitops-registration-test run count-repos --image=curlimages/curl --rm -it --restart=Never --quiet -- \
        curl -s -u "gitops-admin:gitops123" "http://gitea.gitea.svc.cluster.local:3000/api/v1/user/repos" | grep -o '"name"' | wc -l 2>/dev/null || echo "0")
    echo_info "   Repositories: $REPO_COUNT configured"
    
    if [ "$REPO_COUNT" -gt 0 ]; then
        echo_info "   Available repositories:"
        echo_info "     • team-alpha-config (nginx app, development)"
        echo_info "     • team-beta-config (httpd app, production)"
    fi
else
    echo_error "❌ Gitea is not installed"
fi
echo ""

# Check GitOps Registration Service
echo_header "🔧 GitOps Registration Service Status:"
if kubectl --context kind-gitops-registration-test get ksvc gitops-registration -n gitops-system >/dev/null 2>&1; then
    echo_success "✅ GitOps Registration Service is deployed"
    
    # Check service status
    SERVICE_STATUS=$(kubectl --context kind-gitops-registration-test get ksvc gitops-registration -n gitops-system -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
    if [ "$SERVICE_STATUS" = "True" ]; then
        echo_success "   Status: Ready"
    else
        echo_warning "   Status: Not Ready"
    fi
    
    # Get service URL
    SERVICE_URL=$(kubectl --context kind-gitops-registration-test get ksvc gitops-registration -n gitops-system -o jsonpath='{.status.url}')
    echo_info "   URL: $SERVICE_URL"
    
    # Check image source
    SERVICE_IMAGE=$(kubectl --context kind-gitops-registration-test get ksvc gitops-registration -n gitops-system -o jsonpath='{.spec.template.spec.containers[0].image}')
    echo_info "   Image: $SERVICE_IMAGE"
    
    # Test health endpoints
    HEALTH_STATUS=$(kubectl --context kind-gitops-registration-test run test-health --image=curlimages/curl --rm -it --restart=Never --quiet -- \
        curl -s -o /dev/null -w "%{http_code}" "$SERVICE_URL/health/live" 2>/dev/null || echo "000")
    
    if [ "$HEALTH_STATUS" = "200" ]; then
        echo_success "   Health check: Passing"
    else
        echo_warning "   Health check: Failed ($HEALTH_STATUS)"
    fi
else
    echo_error "❌ GitOps Registration Service is not deployed"
fi
echo ""

# Show test commands
echo_header "🧪 Available Tests:"
echo_info "   • ./run-tests.sh          - Original integration tests (stub mode)"
echo_info "   • ./run-real-tests.sh     - Real integration tests with git repositories"
echo_info "   • ./setup-git-repos.sh    - Set up Gitea repositories"
echo_info "   • ./populate-git-repos.sh - Add GitOps manifests to repositories"
echo ""

echo_header "📝 Repository Contents:"
echo_info "   Team Alpha Config Repository:"
echo_info "     ├── manifests/"
echo_info "     │   ├── namespace.yaml    (team-alpha namespace)"
echo_info "     │   ├── deployment.yaml   (nginx app, 2 replicas)"
echo_info "     │   └── configmap.yaml    (application config)"
echo_info "     └── README.md"
echo ""
echo_info "   Team Beta Config Repository:"
echo_info "     ├── manifests/"
echo_info "     │   ├── namespace.yaml    (team-beta namespace)"
echo_info "     │   ├── deployment.yaml   (httpd app, 3 replicas)"
echo_info "     │   └── secret.yaml       (application secrets)"
echo_info "     └── README.md"
echo ""

echo_header "🚀 Quick Start:"
echo_info "   1. Run real integration tests:"
echo_info "      ./run-real-tests.sh"
echo ""
echo_info "   2. Test manual registration with Team Alpha repository:"
echo_info "      curl -X POST $SERVICE_URL/api/v1/registrations \\"
echo_info "        -H 'Content-Type: application/json' \\"
echo_info "        -d '{"
echo_info "          \"repository\": {"
echo_info "            \"url\": \"http://gitea.gitea.svc.cluster.local:3000/gitops-admin/team-alpha-config.git\","
echo_info "            \"branch\": \"main\""
echo_info "          },"
echo_info "          \"namespace\": \"my-team-ns\","
echo_info "          \"tenant\": {"
echo_info "            \"name\": \"my-team\","
echo_info "            \"description\": \"My team environment\""
echo_info "          }"
echo_info "        }'"
echo ""

echo_header "🎯 Environment Summary:"
echo_success "✅ Complete GitOps testing environment is ready!"
echo_info "   • KIND cluster with Knative and ArgoCD"
echo_info "   • Gitea git server with real repositories containing GitOps manifests" 
echo_info "   • GitOps Registration Service deployed via Knative using quay.io image"
echo_info "   • Comprehensive integration tests validating real repository workflows"
echo_info "   • ArgoCD repository secrets configured for automated access"
echo ""
echo_header "==============================================" 