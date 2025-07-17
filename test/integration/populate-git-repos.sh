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

GITEA_CLUSTER_URL="http://gitea.gitea.svc.cluster.local:3000"

# Function to create file in repository using Gitea API
create_file_in_repo() {
    local repo_name="$1"
    local file_path="$2"
    local content="$3"
    local commit_message="$4"
    
    echo_info "Creating file $file_path in repository $repo_name"
    
    # Base64 encode the content
    local encoded_content=$(echo -n "$content" | base64 | tr -d '\n')
    
    kubectl --context kind-gitops-registration-test run create-file-${repo_name}-$(date +%s) --image=curlimages/curl --rm -it --restart=Never --quiet -- \
        curl -X POST "$GITEA_CLUSTER_URL/api/v1/repos/gitops-admin/$repo_name/contents/$file_path" \
        -H "Content-Type: application/json" \
        -u "gitops-admin:gitops123" \
        -d "{\"message\":\"$commit_message\",\"content\":\"$encoded_content\"}" > /dev/null 2>&1 || {
        echo_warning "Failed to create $file_path (file might already exist)"
    }
}

# Function to update existing file in repository using Gitea API
update_file_in_repo() {
    local repo_name="$1"
    local file_path="$2"
    local content="$3"
    local commit_message="$4"
    
    echo_info "Updating file $file_path in repository $repo_name"
    
    # Get file SHA first
    local sha=$(kubectl --context kind-gitops-registration-test run get-sha-${repo_name}-$(date +%s) --image=curlimages/curl --rm -it --restart=Never --quiet -- \
        curl -s -u "gitops-admin:gitops123" "$GITEA_CLUSTER_URL/api/v1/repos/gitops-admin/$repo_name/contents/$file_path" | \
        grep '"sha"' | cut -d'"' -f4 2>/dev/null || echo "")
    
    if [ -n "$sha" ]; then
        # Base64 encode the content
        local encoded_content=$(echo -n "$content" | base64 | tr -d '\n')
        
        kubectl --context kind-gitops-registration-test run update-file-${repo_name}-$(date +%s) --image=curlimages/curl --rm -it --restart=Never --quiet -- \
            curl -X PUT "$GITEA_CLUSTER_URL/api/v1/repos/gitops-admin/$repo_name/contents/$file_path" \
            -H "Content-Type: application/json" \
            -u "gitops-admin:gitops123" \
            -d "{\"message\":\"$commit_message\",\"content\":\"$encoded_content\",\"sha\":\"$sha\"}" > /dev/null 2>&1 || {
            echo_warning "Failed to update $file_path"
        }
    else
        # File doesn't exist, create it
        create_file_in_repo "$repo_name" "$file_path" "$content" "$commit_message"
    fi
}

# Team Alpha Configuration Repository
echo_info "Populating team-alpha-config repository..."

# Create deployment manifest for team alpha
ALPHA_DEPLOYMENT_CONTENT='apiVersion: apps/v1
kind: Deployment
metadata:
  name: team-alpha-app
  namespace: team-alpha
  labels:
    app: team-alpha-app
    team: alpha
spec:
  replicas: 2
  selector:
    matchLabels:
      app: team-alpha-app
  template:
    metadata:
      labels:
        app: team-alpha-app
        team: alpha
    spec:
      containers:
      - name: nginx
        image: nginx:1.21-alpine
        ports:
        - containerPort: 80
        env:
        - name: TEAM
          value: "alpha"
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 200m
            memory: 256Mi
---
apiVersion: v1
kind: Service
metadata:
  name: team-alpha-service
  namespace: team-alpha
  labels:
    app: team-alpha-app
    team: alpha
spec:
  selector:
    app: team-alpha-app
  ports:
  - port: 80
    targetPort: 80
    name: http
  type: ClusterIP'

create_file_in_repo "team-alpha-config" "manifests/deployment.yaml" "$ALPHA_DEPLOYMENT_CONTENT" "Add team alpha deployment manifests"

# Create ConfigMap for team alpha
ALPHA_CONFIG_CONTENT='apiVersion: v1
kind: ConfigMap
metadata:
  name: team-alpha-config
  namespace: team-alpha
  labels:
    app: team-alpha-app
    team: alpha
data:
  app.properties: |
    team.name=alpha
    team.environment=development
    team.region=us-east-1
    app.version=1.2.3
    features.experimental=true
  nginx.conf: |
    server {
        listen 80;
        server_name localhost;
        
        location / {
            root /usr/share/nginx/html;
            index index.html;
        }
        
        location /health {
            return 200 "Team Alpha - Healthy\n";
            add_header Content-Type text/plain;
        }
    }'

create_file_in_repo "team-alpha-config" "manifests/configmap.yaml" "$ALPHA_CONFIG_CONTENT" "Add team alpha configuration"

# Create namespace manifest for team alpha
ALPHA_NAMESPACE_CONTENT='apiVersion: v1
kind: Namespace
metadata:
  name: team-alpha
  labels:
    team: alpha
    managed-by: gitops-registration-service
    project: team-alpha-config
  annotations:
    gitops.io/repository: http://gitea.gitea.svc.cluster.local:3000/gitops-admin/team-alpha-config.git
    gitops.io/branch: main'

create_file_in_repo "team-alpha-config" "manifests/namespace.yaml" "$ALPHA_NAMESPACE_CONTENT" "Add team alpha namespace"

# Update README for team alpha
ALPHA_README_CONTENT='# Team Alpha GitOps Configuration

This repository contains the GitOps configuration for Team Alpha'"'"'s applications and infrastructure.

## Structure

- `manifests/namespace.yaml` - Namespace definition for team-alpha
- `manifests/deployment.yaml` - Application deployment and service
- `manifests/configmap.yaml` - Application configuration

## Team Information

- **Team**: Alpha
- **Environment**: Development
- **Namespace**: team-alpha
- **Applications**: team-alpha-app

## Deployment

This repository is managed by ArgoCD for continuous deployment. Any changes to the manifests will be automatically applied to the cluster.

## Contact

Team Alpha Lead: alpha-team@company.com'

update_file_in_repo "team-alpha-config" "README.md" "$ALPHA_README_CONTENT" "Update README with team alpha details"

# Team Beta Configuration Repository
echo_info "Populating team-beta-config repository..."

# Create deployment manifest for team beta
BETA_DEPLOYMENT_CONTENT='apiVersion: apps/v1
kind: Deployment
metadata:
  name: team-beta-app
  namespace: team-beta
  labels:
    app: team-beta-app
    team: beta
spec:
  replicas: 3
  selector:
    matchLabels:
      app: team-beta-app
  template:
    metadata:
      labels:
        app: team-beta-app
        team: beta
    spec:
      containers:
      - name: app
        image: httpd:2.4-alpine
        ports:
        - containerPort: 80
        env:
        - name: TEAM
          value: "beta"
        - name: ENVIRONMENT
          value: "production"
        resources:
          requests:
            cpu: 200m
            memory: 256Mi
          limits:
            cpu: 500m
            memory: 512Mi
        livenessProbe:
          httpGet:
            path: /
            port: 80
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /
            port: 80
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: team-beta-service
  namespace: team-beta
  labels:
    app: team-beta-app
    team: beta
spec:
  selector:
    app: team-beta-app
  ports:
  - port: 80
    targetPort: 80
    name: http
  type: ClusterIP
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: team-beta-network-policy
  namespace: team-beta
spec:
  podSelector:
    matchLabels:
      app: team-beta-app
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: team-beta
    ports:
    - protocol: TCP
      port: 80
  egress:
  - to: []
    ports:
    - protocol: TCP
      port: 80
    - protocol: TCP
      port: 443'

create_file_in_repo "team-beta-config" "manifests/deployment.yaml" "$BETA_DEPLOYMENT_CONTENT" "Add team beta deployment manifests"

# Create Secret for team beta
BETA_SECRET_CONTENT='apiVersion: v1
kind: Secret
metadata:
  name: team-beta-secrets
  namespace: team-beta
  labels:
    app: team-beta-app
    team: beta
type: Opaque
data:
  # These are base64 encoded test values
  database-url: cG9zdGdyZXNxbDovL3Rlc3QtZGI6NTQzMi90ZWFtYmV0YQ==
  api-key: dGVhbS1iZXRhLWFwaS1rZXktMTIzNDU2
  jwt-secret: dGVhbS1iZXRhLWp3dC1zZWNyZXQtYWJjZGVmZ2g='

create_file_in_repo "team-beta-config" "manifests/secret.yaml" "$BETA_SECRET_CONTENT" "Add team beta secrets"

# Create namespace manifest for team beta
BETA_NAMESPACE_CONTENT='apiVersion: v1
kind: Namespace
metadata:
  name: team-beta
  labels:
    team: beta
    managed-by: gitops-registration-service
    project: team-beta-config
    environment: production
  annotations:
    gitops.io/repository: http://gitea.gitea.svc.cluster.local:3000/gitops-admin/team-beta-config.git
    gitops.io/branch: main'

create_file_in_repo "team-beta-config" "manifests/namespace.yaml" "$BETA_NAMESPACE_CONTENT" "Add team beta namespace"

# Update README for team beta
BETA_README_CONTENT='# Team Beta GitOps Configuration

This repository contains the GitOps configuration for Team Beta'"'"'s production applications and infrastructure.

## Structure

- `manifests/namespace.yaml` - Namespace definition for team-beta
- `manifests/deployment.yaml` - Application deployment, service, and network policy
- `manifests/secret.yaml` - Application secrets (encrypted)

## Team Information

- **Team**: Beta
- **Environment**: Production
- **Namespace**: team-beta
- **Applications**: team-beta-app

## Security

This repository includes:
- Network policies for enhanced security
- Encrypted secrets for sensitive data
- Resource limits and requests
- Health checks and probes

## Deployment

This repository is managed by ArgoCD for continuous deployment. Any changes to the manifests will be automatically applied to the cluster.

## Contact

Team Beta Lead: beta-team@company.com'

update_file_in_repo "team-beta-config" "README.md" "$BETA_README_CONTENT" "Update README with team beta details"

echo_success "Repository population completed!"
echo_info ""
echo_info "Team Alpha Repository Contents:"
echo_info "  - manifests/namespace.yaml (team-alpha namespace)"
echo_info "  - manifests/deployment.yaml (nginx app with 2 replicas)"
echo_info "  - manifests/configmap.yaml (team configuration)"
echo_info ""
echo_info "Team Beta Repository Contents:"
echo_info "  - manifests/namespace.yaml (team-beta namespace)"
echo_info "  - manifests/deployment.yaml (httpd app with 3 replicas + network policy)"
echo_info "  - manifests/secret.yaml (encrypted secrets)"
echo_info ""
echo_info "Both repositories are now ready for GitOps registration testing!" 