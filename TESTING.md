# GitOps Registration Service Testing Guide

## How to Run Tests

### Make Targets Overview

The GitOps Registration Service provides several make targets for different testing scenarios:

```bash
# Quick reference
make help                                                          # Show all available targets
make test-integration-full IMAGE=[container pull spec] # Complete test from scratch (recommended)
make test-unit                                                     # Unit tests only
make test-integration                                              # Integration tests (requires existing cluster)
```

### Complete Integration Test (Recommended)

**Full end-to-end testing with custom image:**
```bash
make test-integration-full IMAGE=quay.io/bcook/gitops-registration:latest
```

This command:
1. ✅ Tears down any existing KIND cluster
2. ✅ Creates fresh KIND cluster with Knative + ArgoCD
3. ✅ Deploys your specified image
4. ✅ Sets up smart git servers (Apache + git-http-backend)
5. ✅ Populates test repositories
6. ✅ Runs enhanced tests with ArgoCD sync verification

**Using local image:**
```bash
# Build and test with local image
make build-image
make test-integration-full IMAGE=localhost/gitops-registration:latest
```

### Individual Test Targets

#### Unit Tests
```bash
make test-unit
```
- Runs Go unit tests for all internal packages
- Fast execution (< 30 seconds)
- No external dependencies required

#### Integration Tests (Existing Environment)
```bash
make test-integration
```
- Runs enhanced integration tests on existing cluster
- Requires pre-configured environment
- Includes ArgoCD sync verification

### Development Workflow

#### Set Up Development Environment
```bash
make dev-setup IMAGE=quay.io/bcook/gitops-registration:latest
```
- Creates complete development environment
- Deploys service with specified image
- Shows access URLs for services

#### Quick Development Testing
```bash
make dev-test IMAGE=localhost/my-custom:latest
```
- Redeploys service with new image
- Runs integration tests immediately
- Faster than full reset

#### Environment Status
```bash
make status
```
- Shows status of all components
- Useful for debugging issues

### Manual Test Execution

If you prefer running tests manually:

```bash
cd test/integration

# 1. Set up environment
./setup-test-env.sh

# 2. Deploy service
./deploy-service.sh quay.io/bcook/gitops-registration:latest

# 3. Set up git repositories (CRITICAL: Use smart git servers)
kubectl apply -f smart-git-servers.yaml
kubectl wait --for=condition=Ready pod -l app=git-servers -n git-servers --timeout=300s
./populate-git-repos.sh

# 4. Run tests
./run-enhanced-tests.sh  # Recommended - includes ArgoCD sync verification
./run-real-tests.sh      # Alternative - focuses on git repository access
./run-tests.sh           # Basic - API functionality only
```

### Test Types Explained

| Test Script | Focus | ArgoCD Sync | Duration |
|-------------|-------|-------------|----------|
| `run-enhanced-tests.sh` | **End-to-end GitOps** | ✅ **Waits for sync** | ~3-5 min |
| `run-real-tests.sh` | Git integration | ✅ **Waits for sync** | ~2-3 min |
| `run-tests.sh` | API functionality | ❌ No sync waiting | ~1-2 min |

### Expected Test Results

**Current Implementation Status:**
- ✅ **New Namespace Registration**: Fully implemented and working
- ⚠️ **Existing Namespace Registration**: Authentication working, feature not yet implemented

**Test Outcomes:**
- **New registrations** should create namespaces, AppProjects, Applications, and sync successfully
- **Existing namespace registrations** should return "not implemented" (500) with proper authentication
- When existing namespace feature is implemented, tests will automatically detect and verify it

### Environment Cleanup

```bash
make clean-all          # Clean everything (cluster + artifacts)
make teardown-kind       # Remove KIND cluster only
make clean              # Remove build artifacts only
```

### CI/CD Usage

```bash
make ci-test            # Suitable for CI/CD pipelines
```

## Git Repository Serving for ArgoCD Integration

### ⚠️ CRITICAL: Working Configuration

The **ONLY** configuration that works reliably with ArgoCD is the **Apache HTTP Server + git-http-backend** setup using Git Smart HTTP protocol. This is implemented in `test/integration/smart-git-servers.yaml`.

### Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   ArgoCD        │    │  Apache Server   │    │ Git Repositories│
│                 │───▶│  + git-http-     │───▶│ (Bare repos)    │
│ (git client)    │    │    backend CGI   │    │ /var/www/git/   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

### Key Components

#### 1. Init Container (alpine/git)
- **Purpose**: Creates proper bare git repositories using standard git commands
- **Image**: `alpine/git:latest`
- **Key Operations**:
  ```bash
  git init --bare /var/www/git/team-alpha-config.git
  git update-server-info  # Essential for HTTP access
  git branch -M main --force  # Ensure main branch exists
  ```

#### 2. Apache HTTP Server
- **Image**: `httpd:2.4`
- **Modules**: `cgi_module`, `alias_module`, `env_module`
- **Configuration**:
  ```apache
  SetEnv GIT_PROJECT_ROOT /var/www/git
  SetEnv GIT_HTTP_EXPORT_ALL 1
  
  ScriptAlias /git/ /usr/libexec/git-core/git-http-backend/
  ScriptAlias /gitops-admin/ /usr/libexec/git-core/git-http-backend/
  ```

#### 3. Git Repository Structure
```
/var/www/git/
├── team-alpha-config.git/
│   ├── HEAD
│   ├── config
│   ├── description
│   ├── hooks/
│   ├── info/
│   │   ├── refs
│   │   └── packs
│   ├── objects/
│   │   ├── info/
│   │   └── pack/
│   └── refs/
│       ├── heads/
│       │   └── main
│       └── tags/
└── team-beta-config.git/
    └── (same structure)
```

### Environment Variables

| Variable | Value | Purpose |
|----------|-------|---------|
| `GIT_PROJECT_ROOT` | `/var/www/git` | Root directory for git repositories |
| `GIT_HTTP_EXPORT_ALL` | `1` | Export all repositories via HTTP |

### URL Patterns Supported

- `http://git-servers.git-servers.svc.cluster.local/git/<repo>.git`
- `http://git-servers.git-servers.svc.cluster.local/gitops-admin/<repo>.git`
- `http://git-servers.git-servers.svc.cluster.local/<repo>.git`

### Health Check

```bash
curl http://git-servers.git-servers.svc.cluster.local/health
# Expected: "Git servers healthy"
```

### Git Smart HTTP Protocol Verification

```bash
curl -H "Git-Protocol: version=2" \
     "http://git-servers.git-servers.svc.cluster.local/git/team-alpha-config.git/info/refs?service=git-upload-pack"
# Expected: 001e# service=git-upload-pack...
```

## Testing Configurations

### Enhanced Tests (Recommended)
```bash
cd test/integration
./run-enhanced-tests.sh
```
Features:
- **Waits for ArgoCD sync completion**
- Validates end-to-end GitOps functionality
- Checks actual resource deployment in namespaces
- 2-minute timeout for sync operations

### Real Tests
```bash
cd test/integration
./run-real-tests.sh
```
Features:
- Tests with actual git repositories
- ArgoCD integration verification
- Comprehensive error handling

### Main Tests
```bash
cd test/integration
./run-tests.sh
```
Features:
- Basic API functionality
- Authentication/authorization
- Error scenarios

## Environment Setup

### Prerequisites
- KIND cluster with Knative Serving
- ArgoCD installed
- Git servers deployed using `smart-git-servers.yaml`

### Quick Setup
```bash
# Create KIND cluster
cd test/integration
./setup-kind-cluster.sh

# Deploy test environment
./setup-test-env.sh

# Deploy smart git servers (REQUIRED for ArgoCD)
kubectl apply -f smart-git-servers.yaml

# Populate repositories
./populate-git-repos.sh

# Run tests
./run-enhanced-tests.sh
```

## Debugging Git Server Issues

### Check Pod Status
```bash
kubectl get pods -n git-servers
kubectl logs -n git-servers deployment/git-servers
```

### Test Git Clone
```bash
kubectl run git-test --image=alpine/git --rm -it --restart=Never -- \
  git clone http://git-servers.git-servers.svc.cluster.local/git/team-alpha-config.git /tmp/test
```

### Test HTTP Access
```bash
kubectl run curl-test --image=curlimages/curl --rm -it --restart=Never -- \
  curl -v http://git-servers.git-servers.svc.cluster.local/health
```

### Check ArgoCD Application Status
```bash
kubectl get applications -n argocd
kubectl describe application <app-name> -n argocd
```

## Performance Considerations

- **Memory**: Minimum 512Mi for git operations
- **CPU**: 100m-500m depending on repository size
- **Storage**: EmptyDir volume sufficient for test repositories
- **Network**: Internal cluster communication only

## Security Notes

- Git servers are **read-only** (no authentication required)
- Repositories are served over HTTP (not HTTPS) for internal cluster use
- No sensitive data should be stored in test repositories

## Maintenance

### Updating Repositories
1. Update content in `populate-git-repos.sh`
2. Redeploy git servers: `kubectl delete -f smart-git-servers.yaml && kubectl apply -f smart-git-servers.yaml`
3. Re-run population script: `./populate-git-repos.sh`

### Adding New Repositories
1. Add repository creation to init container script in `smart-git-servers.yaml`
2. Add content population to `populate-git-repos.sh`
3. Update test cases to use new repositories

---

## 🚨 IMPORTANT REMINDERS

1. **ALWAYS use `smart-git-servers.yaml`** for ArgoCD integration tests
2. **NEVER revert to nginx-based configurations** - they don't work with ArgoCD
3. **Test git clone functionality** before assuming ArgoCD will work
4. **Wait for sync completion** in tests - don't accept "Unknown" status
5. **Use proper git directory structure** - not flat file serving 