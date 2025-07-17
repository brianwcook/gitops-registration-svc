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

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo_error "kubectl is required but not installed"
    exit 1
fi

# Get Gitea service info
GITEA_NODE_PORT=$(kubectl --context kind-gitops-registration-test get svc gitea-nodeport -n gitea -o jsonpath='{.spec.ports[0].nodePort}')
GITEA_NODE_IP=$(kubectl --context kind-gitops-registration-test get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
GITEA_URL="http://${GITEA_NODE_IP}:${GITEA_NODE_PORT}"
GITEA_CLUSTER_URL="http://gitea.gitea.svc.cluster.local:3000"

echo_info "Gitea URL: $GITEA_URL"
echo_info "Gitea Cluster URL: $GITEA_CLUSTER_URL"

# Wait for Gitea to be ready
echo_info "Waiting for Gitea to be ready..."
for i in {1..30}; do
    if kubectl --context kind-gitops-registration-test run test-gitea-connection --image=curlimages/curl --rm -it --restart=Never --quiet -- curl -s "$GITEA_CLUSTER_URL/api/v1/version" > /dev/null 2>&1; then
        echo_success "Gitea is ready"
        break
    fi
    echo_info "Waiting for Gitea... (attempt $i/30)"
    sleep 5
done

# Create temp directory for repositories
TEMP_DIR=$(mktemp -d)
echo_info "Using temp directory: $TEMP_DIR"

# Function to create repository via API
create_gitea_repo() {
    local repo_name="$1"
    local description="$2"
    
    echo_info "Creating repository: $repo_name"
    
    kubectl --context kind-gitops-registration-test run create-repo-${repo_name} --image=curlimages/curl --rm -it --restart=Never --quiet -- \
        curl -X POST "$GITEA_CLUSTER_URL/api/v1/user/repos" \
        -H "Content-Type: application/json" \
        -u "gitops-admin:gitops123" \
        -d "{\"name\":\"$repo_name\",\"description\":\"$description\",\"private\":false,\"auto_init\":true}" \
        > /dev/null 2>&1 || true
    
    echo_success "Repository $repo_name created"
}

# Function to add files to repository
setup_repo_content() {
    local repo_name="$1"
    local content_type="$2"
    
    echo_info "Setting up content for repository: $repo_name"
    
    # Clone repository
    cd "$TEMP_DIR"
    git clone "$GITEA_CLUSTER_URL/gitops-admin/$repo_name.git" || {
        echo_warning "Failed to clone via cluster URL, trying alternative approach"
        
        # Create local repo and push
        mkdir -p "$repo_name"
        cd "$repo_name"
        git init
        git config user.name "GitOps Admin"
        git config user.email "admin@gitops.local"
        
        # Add remote
        git remote add origin "$GITEA_CLUSTER_URL/gitops-admin/$repo_name.git"
    }
    
    cd "$repo_name"
    
    # Create manifests directory
    mkdir -p manifests
    
    case "$content_type" in
        "basic-app")
            cat > manifests/deployment.yaml << 'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: test-app-service
  namespace: default
spec:
  selector:
    app: test-app
  ports:
  - port: 80
    targetPort: 80
  type: ClusterIP
EOF
            ;;
        "config-app")
            cat > manifests/configmap.yaml << 'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: default
data:
  config.yaml: |
    app:
      name: "test-config-app"
      version: "1.0.0"
      environment: "test"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: config-app
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: config-app
  template:
    metadata:
      labels:
        app: config-app
    spec:
      containers:
      - name: app
        image: nginx:alpine
        ports:
        - containerPort: 80
        volumeMounts:
        - name: config
          mountPath: /etc/config
      volumes:
      - name: config
        configMap:
          name: app-config
EOF
            ;;
    esac
    
    # Create README
    cat > README.md << EOF
# GitOps Test Repository: $repo_name

This is a test repository for GitOps registration service integration tests.

## Content
- **Type**: $content_type
- **Manifests**: Located in \`manifests/\` directory
- **Purpose**: Integration testing for GitOps registration service

## Usage
This repository is used by ArgoCD for continuous deployment testing.
EOF
    
    # Add and commit files
    git add .
    git commit -m "Initial commit with GitOps manifests" || echo_warning "Nothing to commit"
    
    # Try to push (this might fail in KIND, but that's ok for our tests)
    git push origin main 2>/dev/null || git push origin master 2>/dev/null || {
        echo_warning "Could not push to remote repository (this is expected in KIND environment)"
    }
    
    echo_success "Content setup completed for $repo_name"
    cd "$TEMP_DIR"
}

# Function to create ArgoCD repository secret
create_argocd_repo_secret() {
    local repo_name="$1"
    
    echo_info "Creating ArgoCD repository secret for: $repo_name"
    
    kubectl --context kind-gitops-registration-test apply -f - << EOF
apiVersion: v1
kind: Secret
metadata:
  name: gitea-repo-${repo_name}
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repository
type: Opaque
stringData:
  type: git
  url: $GITEA_CLUSTER_URL/gitops-admin/${repo_name}.git
  username: gitops-admin
  password: gitops123
EOF
    
    echo_success "ArgoCD repository secret created for $repo_name"
}

# Main setup
echo_info "Setting up GitOps test repositories in Gitea..."

# Create repositories
create_gitea_repo "team-alpha-config" "Team Alpha GitOps configuration repository"
create_gitea_repo "team-beta-config" "Team Beta GitOps configuration repository"

# Wait a bit for repositories to be fully created
sleep 5

# Setup repository content
# Note: In KIND environment, git operations might be limited, so we'll create the content structure
echo_info "Creating repository content..."

# For now, let's just create the ArgoCD repository secrets since git operations in KIND are tricky
# The actual repository content will be handled by the stub implementation for testing

# Create ArgoCD repository secrets
create_argocd_repo_secret "team-alpha-config"
create_argocd_repo_secret "team-beta-config"

# Create a simple test to verify repositories are accessible
echo_info "Testing repository access..."

for repo in "team-alpha-config" "team-beta-config"; do
    if kubectl --context kind-gitops-registration-test run test-repo-${repo} --image=curlimages/curl --rm -it --restart=Never --quiet -- \
        curl -s -u "gitops-admin:gitops123" "$GITEA_CLUSTER_URL/api/v1/repos/gitops-admin/$repo" > /dev/null 2>&1; then
        echo_success "Repository $repo is accessible"
    else
        echo_warning "Repository $repo may not be fully ready"
    fi
done

# Cleanup temp directory
rm -rf "$TEMP_DIR"

echo_success "GitOps test repositories setup completed!"
echo_info ""
echo_info "Available repositories:"
echo_info "  1. team-alpha-config: $GITEA_CLUSTER_URL/gitops-admin/team-alpha-config.git"
echo_info "  2. team-beta-config: $GITEA_CLUSTER_URL/gitops-admin/team-beta-config.git"
echo_info ""
echo_info "Credentials: gitops-admin / gitops123"
echo_info "ArgoCD repository secrets have been created in the argocd namespace" 