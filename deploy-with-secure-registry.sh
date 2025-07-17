#!/bin/bash
set -o errexit

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Deploying GitOps Registration Service with Secure Registry${NC}"
echo -e "${BLUE}=======================================================${NC}"

# Step 1: Set up secure registry with cert-manager
echo -e "\n${GREEN}Step 1: Setting up secure registry with cert-manager...${NC}"
./setup-secure-registry.sh

# Step 2: Build and tag the application image
echo -e "\n${GREEN}Step 2: Building application image...${NC}"
podman build -t gitops-registration:latest .

# Step 3: Tag and push to secure registry
echo -e "\n${GREEN}Step 3: Pushing image to secure registry...${NC}"
podman tag gitops-registration:latest localhost:5001/gitops-registration:latest
podman push localhost:5001/gitops-registration:latest

echo -e "${GREEN}✓ Image pushed successfully!${NC}"

# Step 4: Verify image is in registry
echo -e "\n${GREEN}Step 4: Verifying image in registry...${NC}"
if curl -k -s https://localhost:5001/v2/gitops-registration/tags/list | grep -q latest; then
    echo -e "${GREEN}✓ Image verified in secure registry${NC}"
else
    echo -e "${RED}✗ Image not found in registry${NC}"
    exit 1
fi

# Step 5: Deploy the Knative service
echo -e "\n${GREEN}Step 5: Deploying Knative service...${NC}"
kubectl apply -f knative-service-secure.yaml

# Step 6: Wait for service to be ready
echo -e "\n${GREEN}Step 6: Waiting for service to be ready...${NC}"
kubectl wait --for=condition=Ready ksvc/gitops-registration-service -n konflux-gitops --timeout=300s

# Step 7: Get service URL
echo -e "\n${GREEN}Step 7: Getting service information...${NC}"
SERVICE_URL=$(kubectl get ksvc gitops-registration-service -n konflux-gitops -o jsonpath='{.status.url}')

echo -e "\n${BLUE}🎉 Deployment Complete!${NC}"
echo -e "${BLUE}===================${NC}"
echo -e "${GREEN}Service URL: ${SERVICE_URL}${NC}"
echo -e "${GREEN}Registry URL: https://localhost:5001${NC}"
echo ""
echo -e "${YELLOW}Test the service:${NC}"
echo -e "${YELLOW}  curl \${SERVICE_URL}/health/live${NC}"
echo -e "${YELLOW}  curl \${SERVICE_URL}/health/ready${NC}"
echo ""
echo -e "${YELLOW}To run integration tests:${NC}"
echo -e "${YELLOW}  cd test/integration && ./run-tests.sh${NC}"
echo ""
echo -e "${YELLOW}To stop the registry port forwarding:${NC}"
echo -e "${YELLOW}  kill \$(cat /tmp/registry-port-forward.pid)${NC}" 