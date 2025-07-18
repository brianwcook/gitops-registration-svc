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
IMAGE_NAME="${1:-quay.io/bcook/gitops-registration:latest}"  # Accept image as first argument, default to quay.io
SERVICE_NAME="gitops-registration"
NAMESPACE="konflux-gitops"

echo_info "Building and deploying GitOps registration service..."
echo_info "Using image: $IMAGE_NAME"

# Go to project root
cd "$(dirname "$0")/../.."

# Check if this is a local build or remote image
if [[ "$IMAGE_NAME" == quay.io/* ]] || [[ "$IMAGE_NAME" == docker.io/* ]] || [[ "$IMAGE_NAME" == gcr.io/* ]] || [[ "$IMAGE_NAME" == ghcr.io/* ]]; then
    echo_info "Using remote image: $IMAGE_NAME"
    echo_info "Skipping local build - using published image"
    IMAGE_PULL_POLICY="Always"
else
    echo_info "Building local image: $IMAGE_NAME"
    
    # Build the service binary
    echo_info "Building service binary..."
    go build -o bin/gitops-registration-service cmd/server/main.go

    # Build Docker image
    echo_info "Building Docker image: $IMAGE_NAME"
    if command -v podman &> /dev/null; then
        podman build -t $IMAGE_NAME .
        # Load image into KIND cluster
        echo_info "Loading image into KIND cluster..."
        # Podman doesn't work well with kind load docker-image, so use save/load
        podman save $IMAGE_NAME -o /tmp/gitops-registration.tar
        kind load image-archive /tmp/gitops-registration.tar --name $CLUSTER_NAME
        rm -f /tmp/gitops-registration.tar
    elif command -v docker &> /dev/null; then
        docker build -t $IMAGE_NAME .
        # Load image into KIND cluster
        echo_info "Loading image into KIND cluster..."
        kind load docker-image $IMAGE_NAME --name $CLUSTER_NAME
    else
        echo_error "Neither podman nor docker found. Please install one of them."
        exit 1
    fi
    IMAGE_PULL_POLICY="Never"
fi

# Create namespace if it doesn't exist
echo_info "Creating namespace: $NAMESPACE"
kubectl --context kind-$CLUSTER_NAME create namespace $NAMESPACE --dry-run=client -o yaml | kubectl --context kind-$CLUSTER_NAME apply -f -

# Apply RBAC
echo_info "Applying RBAC configuration..."
kubectl --context kind-$CLUSTER_NAME apply -f deploy/rbac.yaml

# Create the gitops-role ClusterRole that the service expects to bind to
echo_info "Creating gitops-role ClusterRole..."
kubectl --context kind-$CLUSTER_NAME apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: gitops-role
  labels:
    app: gitops-registration-service
rules:
# Limited permissions for GitOps tenants
- apiGroups: [""]
  resources: ["configmaps", "secrets", "services"]
  verbs: ["create", "get", "list", "watch", "update", "patch", "delete"]

# Deployments and ReplicaSets
- apiGroups: ["apps"]
  resources: ["deployments", "replicasets"]
  verbs: ["create", "get", "list", "watch", "update", "patch", "delete"]

# Jobs and CronJobs
- apiGroups: ["batch"]
  resources: ["jobs", "cronjobs"]
  verbs: ["create", "get", "list", "watch", "update", "patch", "delete"]

# Role and RoleBinding management within namespace
- apiGroups: ["rbac.authorization.k8s.io"]
  resources: ["roles", "rolebindings"]
  verbs: ["create", "get", "list", "watch", "update", "patch", "delete"]

# Network policies for tenant isolation
- apiGroups: ["networking.k8s.io"]
  resources: ["networkpolicies"]
  verbs: ["create", "get", "list", "watch", "update", "patch", "delete"]
EOF

# Grant permission to bind the gitops-role
echo_info "Granting permission to bind gitops-role..."
kubectl --context kind-$CLUSTER_NAME patch clusterrole gitops-registration-controller --type='json' -p='[{"op": "add", "path": "/rules/-", "value": {"apiGroups": ["rbac.authorization.k8s.io"], "resources": ["clusterroles"], "verbs": ["bind"], "resourceNames": ["gitops-role"]}}]'

# Apply ConfigMap
echo_info "Applying service configuration..."
kubectl --context kind-$CLUSTER_NAME apply -f deploy/configmap.yaml

# Deploy the service
echo_info "Deploying Knative service..."
cat > deploy/knative-service-local.yaml << EOF
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: $SERVICE_NAME
  namespace: $NAMESPACE
  labels:
    app: gitops-registration-service
spec:
  template:
    metadata:
      labels:
        app: gitops-registration-service
      annotations:
        autoscaling.knative.dev/minScale: "1"
        autoscaling.knative.dev/maxScale: "3"
    spec:
      serviceAccountName: gitops-registration-sa
      containers:
      - name: gitops-registration
        image: $IMAGE_NAME
        imagePullPolicy: $IMAGE_PULL_POLICY
        ports:
        - containerPort: 8080
          protocol: TCP
        env:
        - name: LOG_LEVEL
          value: "info"
        - name: CONFIG_PATH
          value: "/etc/config/config.yaml"
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        volumeMounts:
        - name: config
          mountPath: /etc/config
          readOnly: true
      volumes:
      - name: config
        configMap:
          name: gitops-registration-config
EOF

# Deploy Knative service with the correct image
echo_info "Deploying Knative service..."
sed -e "s|quay.io/bcook/gitops-registration:latest|$IMAGE_NAME|g" \
    -e "s|imagePullPolicy: Always|imagePullPolicy: $IMAGE_PULL_POLICY|g" \
    deploy/knative-service-local.yaml | kubectl --context kind-$CLUSTER_NAME apply -f -

# Wait for deployment to be ready
echo_info "Waiting for service to be ready..."
kubectl --context kind-$CLUSTER_NAME wait --for=condition=Ready ksvc/$SERVICE_NAME -n $NAMESPACE --timeout=300s

# Get service URL
SERVICE_URL=$(kubectl --context kind-$CLUSTER_NAME get ksvc $SERVICE_NAME -n $NAMESPACE -o jsonpath='{.status.url}')

echo_success "Service deployed successfully!"
echo_info "Service URL: $SERVICE_URL"
echo_info "Service status:"
kubectl --context kind-$CLUSTER_NAME get ksvc $SERVICE_NAME -n $NAMESPACE

# Test health endpoints
echo_info "Testing health endpoints..."
sleep 5
kubectl --context kind-$CLUSTER_NAME run test-health --image=curlimages/curl --rm -it --restart=Never -- curl -s "$SERVICE_URL/health/live" || echo_warning "Health check failed"

echo_success "Service deployment completed!" 