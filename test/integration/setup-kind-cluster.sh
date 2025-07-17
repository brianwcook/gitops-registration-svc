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
CLUSTER_NAME="gitops-registration-test"

echo_info "Setting up single-node KIND cluster: $CLUSTER_NAME"

# Check if cluster already exists
if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
    echo_warning "KIND cluster '$CLUSTER_NAME' already exists"
    exit 0
fi

# Create KIND cluster configuration for single node
cat > kind-single-node-config.yaml << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: ${CLUSTER_NAME}
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 8080
    protocol: TCP
  - containerPort: 443
    hostPort: 8443
    protocol: TCP
  - containerPort: 30080
    hostPort: 30080
    protocol: TCP
  - containerPort: 30300
    hostPort: 30300
    protocol: TCP
EOF

echo_info "Creating KIND cluster with single node..."
kind create cluster --config kind-single-node-config.yaml --wait 300s

echo_info "Verifying cluster is ready..."
kubectl cluster-info --context kind-${CLUSTER_NAME}

echo_success "KIND cluster '$CLUSTER_NAME' created successfully!"

# Clean up config file
rm -f kind-single-node-config.yaml

echo_info "Cluster nodes:"
kubectl --context kind-${CLUSTER_NAME} get nodes 