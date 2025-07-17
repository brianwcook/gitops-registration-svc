#!/bin/bash
set -o errexit

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Registry settings
REG_NAME='kind-registry'
REG_PORT='5001'
CLUSTER_NAME='gitops-registration-test'

echo -e "${GREEN}Setting up secure local registry with cert-manager...${NC}"

# Function to wait for deployment to be ready
wait_for_deployment() {
    local namespace=$1
    local deployment=$2
    echo -e "${YELLOW}Waiting for $deployment in $namespace to be ready...${NC}"
    kubectl wait --for=condition=available --timeout=300s deployment/$deployment -n $namespace
}

# Function to wait for pods to be ready
wait_for_pods() {
    local namespace=$1
    local label_selector=$2
    echo -e "${YELLOW}Waiting for pods with label $label_selector in $namespace to be ready...${NC}"
    kubectl wait --for=condition=ready --timeout=300s pod -l $label_selector -n $namespace
}

# Step 1: Install cert-manager
echo -e "${GREEN}Step 1: Installing cert-manager...${NC}"
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.2/cert-manager.yaml

# Wait for cert-manager to be ready
wait_for_deployment cert-manager cert-manager
wait_for_deployment cert-manager cert-manager-cainjector
wait_for_deployment cert-manager cert-manager-webhook

echo -e "${GREEN}cert-manager installed successfully!${NC}"

# Step 2: Create self-signed cluster issuer
echo -e "${GREEN}Step 2: Creating self-signed cluster issuer...${NC}"
cat <<EOF | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: selfsigned-issuer
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: registry-ca
  namespace: cert-manager
spec:
  isCA: true
  commonName: registry-ca
  secretName: registry-ca-secret
  issuerRef:
    name: selfsigned-issuer
    kind: ClusterIssuer
    group: cert-manager.io
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: registry-ca-issuer
spec:
  ca:
    secretName: registry-ca-secret
EOF

# Wait for certificate to be ready
echo -e "${YELLOW}Waiting for CA certificate to be ready...${NC}"
kubectl wait --for=condition=ready certificate/registry-ca -n cert-manager --timeout=120s

# Step 3: Create namespace for registry
echo -e "${GREEN}Step 3: Creating registry namespace...${NC}"
kubectl create namespace registry --dry-run=client -o yaml | kubectl apply -f -

# Step 4: Create certificate for registry
echo -e "${GREEN}Step 4: Creating registry certificate...${NC}"
cat <<EOF | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: registry-tls
  namespace: registry
spec:
  secretName: registry-tls-secret
  issuerRef:
    name: registry-ca-issuer
    kind: ClusterIssuer
    group: cert-manager.io
  dnsNames:
  - kind-registry
  - localhost
  - registry.registry.svc.cluster.local
  ipAddresses:
  - 127.0.0.1
  - ::1
EOF

# Wait for registry certificate to be ready
echo -e "${YELLOW}Waiting for registry certificate to be ready...${NC}"
kubectl wait --for=condition=ready certificate/registry-tls -n registry --timeout=120s

# Step 5: Create registry deployment with TLS
echo -e "${GREEN}Step 5: Creating secure registry deployment...${NC}"
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: registry
  namespace: registry
  labels:
    app: registry
spec:
  replicas: 1
  selector:
    matchLabels:
      app: registry
  template:
    metadata:
      labels:
        app: registry
    spec:
      containers:
      - name: registry
        image: registry:2
        ports:
        - containerPort: 5000
        env:
        - name: REGISTRY_HTTP_TLS_CERTIFICATE
          value: /etc/certs/tls.crt
        - name: REGISTRY_HTTP_TLS_KEY
          value: /etc/certs/tls.key
        - name: REGISTRY_HTTP_ADDR
          value: 0.0.0.0:5000
        volumeMounts:
        - name: certs
          mountPath: /etc/certs
          readOnly: true
        - name: registry-data
          mountPath: /var/lib/registry
      volumes:
      - name: certs
        secret:
          secretName: registry-tls-secret
      - name: registry-data
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: registry
  namespace: registry
spec:
  selector:
    app: registry
  ports:
  - port: 5000
    targetPort: 5000
    nodePort: 30001
  type: NodePort
EOF

# Wait for registry deployment to be ready
wait_for_deployment registry registry

# Step 6: Extract CA certificate and configure nodes
echo -e "${GREEN}Step 6: Configuring cluster nodes to trust registry CA...${NC}"

# Extract CA certificate
kubectl get secret registry-ca-secret -n cert-manager -o jsonpath='{.data.tls\.crt}' | base64 -d > /tmp/registry-ca.crt

# Configure each node to trust the CA
for node in $(kind get nodes --name $CLUSTER_NAME); do
  echo -e "${YELLOW}Configuring node: $node${NC}"
  
  # Copy CA certificate to node
  podman cp /tmp/registry-ca.crt $node:/usr/local/share/ca-certificates/registry-ca.crt
  
  # Update CA certificates
  podman exec $node update-ca-certificates
  
  # Configure containerd for the registry
  REGISTRY_DIR="/etc/containerd/certs.d/localhost:${REG_PORT}"
  podman exec $node mkdir -p "$REGISTRY_DIR"
  
  cat <<EOT | podman exec -i $node cp /dev/stdin "$REGISTRY_DIR/hosts.toml"
[host."https://registry.registry.svc.cluster.local:5000"]
  ca = ["/usr/local/share/ca-certificates/registry-ca.crt"]
[host."https://localhost:${REG_PORT}"]
  ca = ["/usr/local/share/ca-certificates/registry-ca.crt"]
EOT

  # Restart containerd to pick up new configuration
  podman exec $node systemctl restart containerd
done

# Step 7: Create port forward for external access
echo -e "${GREEN}Step 7: Setting up port forwarding...${NC}"

# Kill any existing port forward
pkill -f "kubectl.*port-forward.*registry" || true

# Start port forward in background
kubectl port-forward -n registry service/registry ${REG_PORT}:5000 &
PORT_FORWARD_PID=$!

echo "Port forward PID: $PORT_FORWARD_PID" > /tmp/registry-port-forward.pid

# Wait a moment for port forward to establish
sleep 5

# Step 8: Configure local Docker/Podman to trust the CA
echo -e "${GREEN}Step 8: Configuring local container runtime...${NC}"

# For Docker (if running)
if command -v docker &> /dev/null; then
    sudo mkdir -p /etc/docker/certs.d/localhost:${REG_PORT}
    sudo cp /tmp/registry-ca.crt /etc/docker/certs.d/localhost:${REG_PORT}/ca.crt
    echo -e "${GREEN}Docker configured to trust registry CA${NC}"
fi

# For Podman
mkdir -p ~/.config/containers/certs.d/localhost:${REG_PORT}
cp /tmp/registry-ca.crt ~/.config/containers/certs.d/localhost:${REG_PORT}/ca.crt

echo -e "${GREEN}Podman configured to trust registry CA${NC}"

# Step 9: Test the secure connection
echo -e "${GREEN}Step 9: Testing secure registry connection...${NC}"
sleep 5

if curl -k --cacert /tmp/registry-ca.crt https://localhost:${REG_PORT}/v2/ &>/dev/null; then
    echo -e "${GREEN}✓ Secure registry is accessible!${NC}"
else
    echo -e "${RED}✗ Failed to connect to secure registry${NC}"
    exit 1
fi

# Cleanup temp files
rm -f /tmp/registry-ca.crt

echo -e "${GREEN}Secure local registry setup complete!${NC}"
echo -e "${GREEN}Registry available at: https://localhost:${REG_PORT}${NC}"
echo -e "${GREEN}To use images:${NC}"
echo -e "${YELLOW}  podman tag <image> localhost:${REG_PORT}/<image>${NC}"
echo -e "${YELLOW}  podman push localhost:${REG_PORT}/<image>${NC}"
echo ""
echo -e "${GREEN}To stop port forwarding:${NC}"
echo -e "${YELLOW}  kill \$(cat /tmp/registry-port-forward.pid)${NC}"
echo ""
echo -e "${GREEN}CA certificate is trusted by the cluster and your local container runtime.${NC}" 