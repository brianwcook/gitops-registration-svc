#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Setting up simple local registry for Knative + Kind + Podman...${NC}"

# 1. Create simple registry container
echo -e "${YELLOW}Creating local registry container...${NC}"
podman run -d \
  --restart=always \
  -p 5000:5000 \
  --name local-registry \
  docker.io/library/registry:2 || echo "Registry already running"

# 2. Create Kind cluster with registry support
echo -e "${YELLOW}Creating Kind cluster configuration...${NC}"
cat > /tmp/kind-registry-config.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:5000"]
    endpoint = ["http://local-registry:5000"]
EOF

# 3. Connect registry to Kind network
echo -e "${YELLOW}Connecting registry to Kind network...${NC}"
if ! podman network exists kind; then
    podman network create kind
fi

podman network connect kind local-registry 2>/dev/null || echo "Already connected"

# 4. Configure cluster nodes to use registry
echo -e "${YELLOW}Configuring cluster nodes...${NC}"
for node in $(kind get nodes --name gitops-registration-test); do
  echo "Configuring node: $node"
  
  # Create registry config directory
  podman exec "$node" mkdir -p /etc/containerd/certs.d/localhost:5000
  
  # Add registry configuration
  cat <<EOT | podman exec -i "$node" cp /dev/stdin /etc/containerd/certs.d/localhost:5000/hosts.toml
server = "http://localhost:5000"

[host."http://local-registry:5000"]
  capabilities = ["pull", "resolve"]
EOT

  # Restart containerd
  podman exec "$node" systemctl restart containerd
done

echo -e "${GREEN}✓ Local registry setup complete!${NC}"
echo -e "${GREEN}Registry available at: localhost:5000${NC}"
echo -e "${GREEN}To push images: podman push localhost:5000/<image>${NC}" 