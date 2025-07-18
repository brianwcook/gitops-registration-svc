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

# Configuration
CLUSTER_NAME="${CLUSTER_NAME:-gitops-registration-test}"
KNATIVE_VERSION="v1.12.0"
ARGOCD_VERSION="v2.8.4"
USE_SINGLE_NODE="${USE_SINGLE_NODE:-true}"

echo_info "Setting up integration test environment for GitOps Registration Service"

# Check prerequisites
echo_info "Checking prerequisites..."

if ! command -v kind &> /dev/null; then
    echo_error "kind is not installed. Please install kind: https://kind.sigs.k8s.io/docs/user/quick-start/"
    exit 1
fi

if ! command -v kubectl &> /dev/null; then
    echo_error "kubectl is not installed. Please install kubectl: https://kubernetes.io/docs/tasks/tools/"
    exit 1
fi

if ! command -v podman &> /dev/null; then
    echo_error "podman is not installed. Please install podman: https://podman.io/getting-started/installation"
    exit 1
fi

echo_success "Prerequisites check passed"

# Create Kind cluster
echo_info "Creating Kind cluster: $CLUSTER_NAME"
if kind get clusters | grep -q "^$CLUSTER_NAME$"; then
    echo_warning "Cluster $CLUSTER_NAME already exists. Deleting..."
    kind delete cluster --name "$CLUSTER_NAME"
fi

if [ "$USE_SINGLE_NODE" = "true" ]; then
    echo_info "Using single-node KIND cluster configuration"
    ./setup-kind-cluster.sh
else
    echo_info "Using multi-node KIND cluster configuration"
    kind create cluster --name "$CLUSTER_NAME" --config kind-config.yaml
fi
echo_success "Kind cluster created successfully"

# Wait for cluster to be ready
echo_info "Waiting for cluster to be ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=300s
echo_success "Cluster nodes are ready"

# Deploy all infrastructure components in parallel for faster setup
echo_info "Deploying infrastructure components in parallel..."
echo_info "  - Knative Serving $KNATIVE_VERSION"
echo_info "  - ArgoCD $ARGOCD_VERSION" 
echo_info "  - Smart HTTP Git Servers"

# 1. Deploy Knative Serving (CRDs + Core)
echo_info "Installing Knative Serving $KNATIVE_VERSION..."
kubectl apply -f https://github.com/knative/serving/releases/download/knative-${KNATIVE_VERSION}/serving-crds.yaml
kubectl apply -f https://github.com/knative/serving/releases/download/knative-${KNATIVE_VERSION}/serving-core.yaml

# 2. Deploy ArgoCD
echo_info "Installing ArgoCD $ARGOCD_VERSION..."
kubectl create namespace argocd || echo_warning "ArgoCD namespace already exists"
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# 3. Deploy Smart Git Servers  
echo_info "Installing smart HTTP git servers..."
kubectl apply -f smart-git-servers.yaml

# Now wait for all components to be ready in parallel
echo_info "Waiting for all infrastructure components to be ready..."

# Wait for Knative to be ready
echo_info "Waiting for Knative to be ready..."
kubectl wait --for=condition=Ready pod --all -n knative-serving --timeout=300s

# Wait for ArgoCD to be ready  
echo_info "Waiting for ArgoCD to be ready..."
kubectl wait --for=condition=Ready pod --all -n argocd --timeout=600s

# Configure ArgoCD with namespace enforcement (CRITICAL for security!)
echo_info "Configuring ArgoCD with namespace enforcement..."
kubectl patch configmap argocd-cm -n argocd --type merge -p '{"data":{"application.namespaceEnforcement":"true"}}'

# Restart ArgoCD server to pick up new configuration
echo_info "Restarting ArgoCD server to apply namespace enforcement..."
kubectl rollout restart deployment argocd-server -n argocd
kubectl rollout restart statefulset argocd-application-controller -n argocd

# Wait for ArgoCD to be ready again after restart
echo_info "Waiting for ArgoCD to be ready after configuration..."
kubectl wait --for=condition=Ready pod --all -n argocd --timeout=600s

# Wait for git servers to be ready
echo_info "Waiting for git servers to be ready..."
kubectl wait --for=condition=Ready pod -l app=git-servers -n git-servers --timeout=300s

# Additional Knative webhook setup (after basic pods are ready)
echo_info "Waiting for Knative webhooks to be ready..."
sleep 30
for i in {1..12}; do
    if kubectl get validatingwebhookconfiguration config.webhook.serving.knative.dev -o jsonpath='{.webhooks[0].clientConfig.service.name}' 2>/dev/null; then
        echo_info "Webhook configuration exists, testing connectivity..."
        if kubectl -n knative-serving get endpoints webhook | grep -q webhook 2>/dev/null; then
            echo_success "Webhook endpoints are ready"
            break
        fi
    fi
    echo_info "Waiting for webhooks to be ready... (attempt $i/12)"
    sleep 10
done

# Install Knative networking layer (Kourier) - after core is ready
echo_info "Installing Knative networking layer (Kourier)..."
kubectl apply -f https://github.com/knative/net-kourier/releases/download/knative-${KNATIVE_VERSION}/kourier.yaml

# Configure Knative to use Kourier
kubectl patch configmap/config-network \
  --namespace knative-serving \
  --type merge \
  --patch '{"data":{"ingress-class":"kourier.ingress.networking.knative.dev"}}'

# Wait for Kourier to be ready
kubectl wait --for=condition=Ready pod --all -n kourier-system --timeout=300s

# Configure ArgoCD for testing (after ArgoCD is ready)
kubectl patch svc argocd-server -n argocd -p '{"spec":{"type":"NodePort","ports":[{"name":"https","port":443,"protocol":"TCP","targetPort":8080,"nodePort":30080}]}}'

echo_success "Knative Serving installed successfully"
echo_success "ArgoCD installed successfully" 
echo_success "Smart HTTP git servers installed successfully"

# Apply konflux-admin-user-actions ClusterRole for testing
echo_info "Creating konflux-admin-user-actions ClusterRole for testing..."
kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: konflux-admin-user-actions
  labels:
    konflux-cluster-role: "true"
rules:
- verbs: [get, list, watch, create, update, patch, delete, deletecollection]
  apiGroups: [appstudio.redhat.com]
  resources: [applications, components, imagerepositories]
- verbs: [get, list, watch, create, update, patch, delete]
  apiGroups: [appstudio.redhat.com]
  resources: [snapshots]
- verbs: [get, list, watch]
  apiGroups: [tekton.dev]
  resources: [taskruns]
- verbs: [get, list, watch, create, update, patch, delete]
  apiGroups: [tekton.dev]
  resources: [pipelineruns]
- verbs: [get, list, watch, create, update, patch, delete]
  apiGroups: [batch]
  resources: [cronjobs, jobs]
- verbs: [get, list, watch, create, update, patch, delete]
  apiGroups: [""]
  resources: [secrets]
- verbs: [get, list, create, update, patch, delete]
  apiGroups: [rbac.authorization.k8s.io]
  resources: [roles, rolebindings]
- verbs: [get, list, watch, create, update, patch, delete]
  apiGroups: [""]
  resources: [serviceaccounts]
- verbs: [get, list, watch, create, update, patch]
  apiGroups: [""]
  resources: [serviceaccounts/token]
EOF

# Setup test namespaces and users for FR-008 testing
echo_info "Setting up test users and namespaces for FR-008 testing..."

# Create test namespaces
kubectl create namespace test-namespace-alpha || echo_warning "test-namespace-alpha already exists"
kubectl create namespace test-namespace-beta || echo_warning "test-namespace-beta already exists"
kubectl create namespace existing-namespace-convert || echo_warning "existing-namespace-convert already exists"

# Create test service accounts (representing users)
kubectl create serviceaccount valid-user-alpha --namespace default || echo_warning "valid-user-alpha already exists"
kubectl create serviceaccount valid-user-beta --namespace default || echo_warning "valid-user-beta already exists"
kubectl create serviceaccount invalid-user --namespace default || echo_warning "invalid-user already exists"
kubectl create serviceaccount cross-namespace-user --namespace default || echo_warning "cross-namespace-user already exists"

# Create role bindings for testing authorization
kubectl create rolebinding valid-user-alpha-binding \
  --clusterrole=konflux-admin-user-actions \
  --serviceaccount=default:valid-user-alpha \
  --namespace=test-namespace-alpha || echo_warning "valid-user-alpha-binding already exists"

kubectl create rolebinding valid-user-beta-binding \
  --clusterrole=konflux-admin-user-actions \
  --serviceaccount=default:valid-user-beta \
  --namespace=test-namespace-beta || echo_warning "valid-user-beta-binding already exists"

# Cross-namespace user has access only to namespace-alpha
kubectl create rolebinding cross-namespace-user-binding \
  --clusterrole=konflux-admin-user-actions \
  --serviceaccount=default:cross-namespace-user \
  --namespace=test-namespace-alpha || echo_warning "cross-namespace-user-binding already exists"

echo_success "Test users and namespaces configured"

# Note: Service deployment is handled by deploy-service.sh in the Makefile pipeline
echo_info "GitOps Registration Service will be deployed separately via deploy-service.sh"

# Display connection information
echo_info "Test environment setup complete!"
echo_info "Cluster: $CLUSTER_NAME"
echo_info "ArgoCD UI: https://localhost:30080 (admin/$(kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d))"
echo_info ""
echo_info "Next steps:"
echo_info "  1. Deploy service: make deploy-service IMAGE=<your-image>"
echo_info "  2. Run tests: make run-integration-tests"
echo_info ""
echo_info "To clean up:"
echo_info "  kind delete cluster --name $CLUSTER_NAME" 