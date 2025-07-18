# GitOps Registration Service

[![CI/CD Pipeline](https://github.com/brianwcook/gitops-registration-svc/actions/workflows/ci.yml/badge.svg)](https://github.com/brianwcook/gitops-registration-svc/actions/workflows/ci.yml)

<!-- Working toward completely green CI run with comprehensive linting fixes -->

A Kubernetes service for managing GitOps registrations with ArgoCD integration.

## Overview

This service implements the GitOps Registration Service as outlined in [ADR 47: GitOps Onboarding Redesign](https://raw.githubusercontent.com/konflux-ci/architecture/d91958f414e7b7cc5293bbbbe33cad9ad7552991/ADR/0047-gitops-onboarding-redesign.md). It provides:

- **Secure GitOps**: 1:1 mapping between GitOps repositories and Kubernetes namespaces
- **ArgoCD Integration**: Automated AppProject and Application creation for continuous deployment
- **Enhanced Security**: ArgoCD impersonation with dedicated service accounts per tenant
- **Resource Restrictions**: Configurable allow/deny lists for controlling which Kubernetes resources can be synced
- **Authorization Enforcement**: RBAC-based validation using SubjectAccessReview API (FR-008)
- **Registration Control**: Simple configuration to enable/disable new namespace registrations
- **Namespace Isolation**: Strict security boundaries using service account impersonation

## Key Features

### ✅ Core Functionality
- [x] Repository registration and lifecycle management
- [x] Namespace provisioning with secure isolation
- [x] ArgoCD AppProject and Application configuration
- [x] **ArgoCD namespace enforcement** - prevents cross-tenant attacks ✅
- [x] **Tenant isolation security** - prevents namespace creation privilege escalation ✅
- [x] Resource sync restrictions with allow/deny lists
- [x] RESTful API with OpenAPI 3.0 specification
- [x] Health checks and metrics endpoints

### ✅ Enhanced Security (ArgoCD Impersonation)
- [x] **Service Account Isolation** - dedicated service accounts per tenant namespace
- [x] **Least Privilege** - minimal required permissions via ClusterRole validation
- [x] **Repository Conflict Detection** - prevents multiple registrations of same repository
- [x] **Startup Validation** - ClusterRole security analysis with warnings
- [x] **Atomic Operations** - automatic cleanup on failures
- [x] **Cross-tenant Prevention** - service accounts cannot access other tenant namespaces

### ✅ FR-008: Existing Namespace GitOps Conversion
- [x] Users with `konflux-admin-user-actions` role can register existing namespaces
- [x] SubjectAccessReview validation ensures proper authorization
- [x] Cross-namespace security enforcement
- [x] Comprehensive audit logging

### ✅ Registration Control
- [x] Simple on/off configuration for new namespace registrations
- [x] Immediate response without complex calculations
- [x] Integration with external capacity management tools
- [x] Service unavailable response when registrations are disabled

### ✅ Security & Compliance
- [x] Configurable resource type restrictions via allow/deny lists
- [x] Service account impersonation for namespace operations
- [x] RBAC-based access control
- [x] Event logging for audit trails

## 🚀 CI/CD Pipeline

This repository includes a comprehensive GitHub Actions CI/CD pipeline ensuring code quality, security, and reliable deployments.

### 🔄 Automated Testing & Validation

[![CI/CD Pipeline](https://github.com/bcook/gitops-registration/actions/workflows/ci.yml/badge.svg)](https://github.com/bcook/gitops-registration/actions/workflows/ci.yml)

**Three-tier testing strategy:**

1. **🏃‍♂️ PR Validation** - Fast feedback for pull requests (2-3 minutes)
   - Code formatting and linting
   - Unit tests with coverage validation (>60%)
   - Security scanning (gosec)
   - Dockerfile and YAML validation

2. **🧪 Full CI Pipeline** - Comprehensive validation on merge (8-12 minutes)
   - Multi-platform container builds (linux/amd64, linux/arm64)
   - Integration tests across Kubernetes versions (v1.29.2, v1.30.0)
   - Vulnerability scanning with Trivy
   - Automated release tagging

3. **🔧 Manual Testing** - On-demand debugging and validation
   - Configurable test types (unit, integration, debug)
   - Custom container image support
   - Cluster preservation for inspection

### 📊 Quality Gates

- **Test Coverage**: Minimum 60% required, reported via Codecov
- **Security**: Trivy vulnerability scanning, gosec static analysis
- **Code Quality**: golangci-lint with comprehensive rule set
- **Multi-platform**: Container builds for AMD64 and ARM64
- **K8s Compatibility**: Tested against multiple Kubernetes versions

### 🛡️ Security & Compliance

- **Dependency Management**: Automated Dependabot updates
- **Container Security**: Multi-stage builds with distroless base images
- **Secret Management**: GitHub OIDC for secure container registry access
- **Audit Trail**: Complete build and deployment tracking

### 📖 CI/CD Documentation

Detailed documentation available at [`.github/README.md`](.github/README.md) including:
- Workflow configuration and customization
- Debugging failed builds
- Adding custom checks
- Best practices and maintenance

## Quick Start - Integration Testing

The easiest way to test the complete GitOps registration service is to use the integrated make targets:

```bash
# Run complete integration tests from scratch (recommended)
make test-integration-full

# Show all available make targets
make help

# Set up development environment only
make dev-setup

# Quick test (assumes environment exists)
make quick-test

# Clean up everything
make clean-all
```

The `test-integration-full` target will:
1. ✅ Create a single-node KIND cluster
2. ✅ Install Knative, ArgoCD, and Gitea
3. ✅ Set up git repositories with GitOps manifests  
4. ✅ Build and deploy the registration service
5. ✅ Run comprehensive integration tests
6. ✅ Verify actual GitOps sync and deployment

### Prerequisites for Integration Testing

- [KIND](https://kind.sigs.k8s.io/docs/user/quick-start/) 
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Docker](https://docs.docker.com/get-docker/) or [Podman](https://podman.io/getting-started/installation)
- Go 1.21+

### Example Integration Test Output

```bash
$ make test-integration-full
==========================================
GitOps Registration Service - Full Integration Test
==========================================
[INFO] Setting up integration test environment...
[SUCCESS] KIND cluster created successfully!
[SUCCESS] Knative installed and ready
[SUCCESS] ArgoCD installed and ready  
[SUCCESS] Smart HTTP Git servers installed with test repositories
[INFO] Deploying GitOps registration service...
[SUCCESS] Service deployed successfully!
[INFO] Running enhanced integration tests...
[SUCCESS] ✓ GET /health/live returned 200 as expected
[SUCCESS] ✓ POST /api/v1/registrations returned 201 as expected
[SUCCESS] ✓ Namespace team-alpha was created
[SUCCESS] ✓ ArgoCD Application team-alpha-app was created
[SUCCESS] ✓ Repository metadata verification working
[SUCCESS] ✓ Impersonation functionality working
[SUCCESS] ✓ Service account security isolation working
[SUCCESS] All tests passed! (71/71)
```

## Architecture

```mermaid
graph TB
    User[User] --> API[GitOps Registration Service]
    API --> K8s[Kubernetes API]
    API --> ArgoCD[ArgoCD API]
    API --> Auth[Authorization Service]
    
    K8s --> NS[Namespace Management]
    K8s --> SA[Service Account Creation]
    K8s --> RB[RBAC Configuration]
    
    ArgoCD --> AP[AppProject Creation]
    ArgoCD --> App[Application Creation]
    
    Auth --> SAR[SubjectAccessReview]
    Auth --> Token[Token Validation]
    
    API --> RegCtrl[Registration Control]
    RegCtrl --> Config[Configuration Check]
```

## Quick Start

### Prerequisites

- **Go 1.21+**
- **kind** (for local testing)
- **kubectl**
- **podman** (for container builds on macOS)

### Local Development

1. **Clone and build**:
   ```bash
   git clone <repository-url>
   cd gitops-registration-service
   go mod tidy
   go build -o bin/gitops-registration-service cmd/server/main.go
   ```

2. **Run locally**:
   ```bash
   ./bin/gitops-registration-service
   ```

3. **Test health endpoints**:
   ```bash
   curl http://localhost:8080/health/live
   curl http://localhost:8080/health/ready
   ```

### Integration Testing

The service includes comprehensive integration tests that run in a Kind cluster with Knative and ArgoCD.

1. **Setup test environment**:
   ```bash
   ./test/integration/setup-test-env.sh
   ```
   
   This script will:
   - Create a Kind cluster with 3 nodes
   - Install Knative Serving with Kourier networking
   - Install ArgoCD with NodePort access
   - Deploy the GitOps Registration Service
   - Configure test users and namespaces for authorization testing
   - Build and load the service image using podman

2. **Run integration tests**:
   ```bash
   cd test/integration
   ./run-tests.sh
   ```
   
   The test suite validates:
   - Health endpoints functionality
   - Registration control (enable/disable)
   - Basic registration workflows
   - Existing namespace authorization (FR-008)
   - Error handling and edge cases
   - Metrics endpoint

3. **Cleanup**:
   ```bash
   kind delete cluster --name gitops-registration-test
   ```

## API Reference

### Core Endpoints

#### Registration Management
```http
POST   /api/v1/registrations              # Create new GitOps registration
GET    /api/v1/registrations              # List all registrations
GET    /api/v1/registrations/{id}         # Get registration details
DELETE /api/v1/registrations/{id}         # Delete registration
GET    /api/v1/registrations/{id}/status  # Get registration status
POST   /api/v1/registrations/{id}/sync    # Trigger sync
```

#### Existing Namespace Registration (FR-008)
```http
POST   /api/v1/registrations/existing     # Register existing namespace
```

#### Health & Monitoring
```http
GET    /health/live                       # Liveness probe
GET    /health/ready                      # Readiness probe
GET    /metrics                           # Prometheus metrics
```

### Example Usage

#### Register New GitOps Repository
```bash
curl -X POST http://localhost:8080/api/v1/registrations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "repository": {
      "url": "https://github.com/team/config",
      "branch": "main"
    },
    "namespace": "team-production"
  }'
```

#### Register Existing Namespace (FR-008)
```bash
curl -X POST http://localhost:8080/api/v1/registrations/existing \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "repository": {
      "url": "https://github.com/team/existing-config",
      "branch": "main"
    },
    "existingNamespace": "existing-team-namespace"
  }'
```

**Note**: If new registrations are disabled, this will return:
```json
{
  "error": "REGISTRATION_DISABLED",
  "message": "New namespace registrations are currently disabled"
}
```
With HTTP status `503 Service Unavailable`.

## Resource Restrictions

The service supports configurable resource restrictions to control which Kubernetes resource types can be synced by ArgoCD. This is implemented using ArgoCD AppProject's built-in whitelist/blacklist functionality.

**Important**: Resource restrictions are configured at the **service level** by cluster administrators, not by individual users making registration requests. All AppProjects created by the service will use the same resource restrictions.

### Service Configuration

Cluster administrators configure resource restrictions in the service configuration file or via environment variables:

#### Configuration File Example (config.yaml)
```yaml
security:
  # Cluster admin can provide EITHER allowList OR denyList, not both
  
  # Allow List (Whitelist) - only these resources can be synced
  resourceAllowList:
    - group: ""
      kind: "ConfigMap"
    - group: ""
      kind: "Service"
    - group: "apps"
      kind: "Deployment"
    - group: "networking.k8s.io"
      kind: "Ingress"
  
  # OR Deny List (Blacklist) - all resources except these can be synced
  # resourceDenyList:
  #   - group: ""
  #     kind: "Secret"
  #   - group: "rbac.authorization.k8s.io"
  #     kind: "RoleBinding"
  #   - group: "kafka.strimzi.io"
  #     kind: "KafkaTopic"
```

### Allow List (Whitelist)
When an `resourceAllowList` is configured, **only** the specified resource types can be synced. All other resources will be blocked.

### Deny List (Blacklist)
When a `resourceDenyList` is configured, **all** resource types can be synced **except** those specified in the list.

### Default Behavior
If neither `resourceAllowList` nor `resourceDenyList` is configured, all resource types are allowed (no restrictions).

### Validation Rules
- **Mutually Exclusive**: Service can be configured with either `resourceAllowList` OR `resourceDenyList`, but not both
- **Service-wide**: All AppProjects created by the service use the same restrictions
- **CRD Support**: Custom Resource Definitions are supported without validation
- **Group Field**: 
  - Empty string `""` for core Kubernetes resources (Pod, Service, ConfigMap, etc.)
  - API group name for other resources (e.g., `"apps"`, `"networking.k8s.io"`)
- **Admin Control**: Only cluster administrators can modify these restrictions

### Resource Type Examples

| Resource | Group | Kind |
|----------|-------|------|
| ConfigMap | `""` | `"ConfigMap"` |
| Service | `""` | `"Service"` |
| Secret | `""` | `"Secret"` |
| Deployment | `"apps"` | `"Deployment"` |
| Ingress | `"networking.k8s.io"` | `"Ingress"` |
| Custom Resource | `"mycompany.io"` | `"MyCustomResource"` |

## Configuration

The service is configured through environment variables and/or YAML configuration files:

### Environment Variables
- `PORT` - HTTP server port (default: 8080)
- `CONFIG_PATH` - Path to YAML configuration file
- `ARGOCD_SERVER` - ArgoCD server URL
- `ARGOCD_NAMESPACE` - ArgoCD namespace (default: argocd)
- `ALLOW_NEW_NAMESPACES` - Enable/disable new registrations (default: true)

### YAML Configuration Example

```yaml
server:
  port: 8080
  timeout: "30s"

argocd:
  server: "argocd-server.argocd.svc.cluster.local"
  namespace: "argocd"
  grpc: true

kubernetes:
  namespace: "gitops-registration-system"

security:
  # ArgoCD Impersonation (Enhanced Security)
  impersonation:
    enabled: false                          # Enable service account impersonation
    clusterRole: "gitops-deployer"          # ClusterRole to bind to tenant service accounts
    serviceAccountBaseName: "gitops-sa"     # Base name for generated service accounts
    validatePermissions: true               # Validate ClusterRole on startup
    autoCleanup: true                       # Clean up resources when namespaces are deleted

  # Resource Restrictions (Legacy approach - use impersonation instead)
  allowedResourceTypes:
  - jobs
  - cronjobs
  - secrets
  - rolebindings

registration:
  allowNewNamespaces: true
  
authorization:
  requiredRole: "konflux-admin-user-actions"
  enableSubjectAccessReview: true
  auditFailedAttempts: true
```

## ArgoCD Impersonation (Enhanced Security)

The service supports ArgoCD's impersonation feature to provide enhanced security through service account isolation. When enabled, each tenant gets a dedicated service account with limited permissions instead of using a single powerful service account.

### Overview

**Traditional Approach (Insecure)**:
- Single service account with broad cluster permissions
- All tenants share the same service account
- Risk of privilege escalation and cross-tenant access

**Impersonation Approach (Secure)**:
- Each tenant gets a dedicated service account in their namespace
- Service accounts have minimal required permissions
- Complete tenant isolation and least privilege principles
- ArgoCD uses different service accounts for each tenant

### Configuration

#### Enable Impersonation

```yaml
security:
  impersonation:
    enabled: true
    clusterRole: "gitops-deployer"
    serviceAccountBaseName: "gitops-sa"
    validatePermissions: true
    autoCleanup: true
```

#### Create ClusterRole

Create a ClusterRole with minimal required permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: gitops-deployer
rules:
# Allow managing common GitOps resources
- apiGroups: [""]
  resources: ["secrets", "configmaps", "services"]
  verbs: ["create", "get", "list", "update", "patch", "delete"]
- apiGroups: ["apps"]
  resources: ["deployments", "replicasets"]
  verbs: ["create", "get", "list", "update", "patch", "delete"]
- apiGroups: ["networking.k8s.io"]
  resources: ["ingresses"]
  verbs: ["create", "get", "list", "update", "patch", "delete"]
# Intentionally exclude cluster-scoped resources and dangerous permissions
```

#### Enable ArgoCD Impersonation

Configure ArgoCD to support impersonation:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  # Enable impersonation support
  application.sync.impersonation.enabled: "true"
  # Enable namespace enforcement (REQUIRED)
  application.namespaceEnforcement: "true"
```

### How It Works

1. **Registration Request**: User registers a GitOps repository for namespace `my-app`

2. **Service Account Creation**: Service creates:
   - Namespace: `my-app`
   - ServiceAccount: `gitops-sa-abc123` (generated name) in `my-app` namespace
   - RoleBinding: Binds `gitops-deployer` ClusterRole to the ServiceAccount

3. **AppProject Configuration**: AppProject includes impersonation settings:
   ```yaml
   apiVersion: argoproj.io/v1alpha1
   kind: AppProject
   metadata:
     name: my-app
     namespace: argocd
   spec:
     destinationServiceAccounts:
     - server: https://kubernetes.default.svc
       namespace: my-app
       defaultServiceAccount: gitops-sa-abc123
     destinations:
     - namespace: my-app
       server: https://kubernetes.default.svc
   ```

4. **ArgoCD Sync**: ArgoCD uses `my-app/gitops-sa-abc123` for sync operations instead of its own service account

### Security Benefits

✅ **Namespace Isolation**: Each tenant isolated in their own namespace with dedicated service account  
✅ **Least Privilege**: Service accounts have minimal required permissions defined by ClusterRole  
✅ **No Cross-tenant Access**: Service accounts cannot access resources in other tenant namespaces  
✅ **Repository Conflicts**: Prevents multiple registrations of the same repository  
✅ **ArgoCD Separation**: ArgoCD control plane separated from tenant sync operations  
✅ **Audit Trail**: Clear mapping of service accounts to tenant namespaces  

### Repository Conflict Detection

When impersonation is enabled, the service prevents multiple registrations of the same repository:

```bash
# First registration succeeds
curl -X POST /api/v1/registrations \
  -d '{"namespace": "team-a", "repository": {"url": "https://github.com/team/config"}}'
# → 201 Created

# Second registration of same repository fails
curl -X POST /api/v1/registrations \
  -d '{"namespace": "team-b", "repository": {"url": "https://github.com/team/config"}}'
# → 409 Conflict: repository already registered
```

### Startup Validation

When impersonation is enabled, the service validates the ClusterRole on startup:

```bash
[INFO] Impersonation is enabled, validating ClusterRole: gitops-deployer
[WARN] ClusterRole gitops-deployer security warnings:
[WARN]   - ClusterRole can modify cluster-scoped resource: nodes
[INFO] ClusterRole gitops-deployer validated successfully for impersonation
```

**Security warnings are logged for**:
- ClusterRoles with `cluster-admin` level permissions
- Permissions that span namespaces (cluster-wide list/watch)
- Ability to modify cluster-scoped resources

### Error Handling

**Invalid ClusterRole**: Service logs warnings but continues to operate
**Service Account Creation Failure**: Namespace is cleaned up atomically
**RoleBinding Creation Failure**: Service account and namespace are cleaned up

### Migration Guide

**From Legacy to Impersonation**:

1. **Create ClusterRole** with appropriate permissions
2. **Enable ArgoCD impersonation** in `argocd-cm` ConfigMap
3. **Update service configuration** to enable impersonation
4. **Test with new registrations** - existing registrations continue to work
5. **Monitor logs** for security warnings about ClusterRole permissions

**Backward Compatibility**: When `impersonation.enabled: false` (default), the service behaves exactly as before.

### Registration Control

The service supports simple on/off control for new namespace registrations:

- **Enabled** (default): New namespace registrations are accepted and processed
- **Disabled**: New namespace registrations return HTTP 503 with "REGISTRATION_DISABLED" error

This allows external capacity management tools to control when new registrations are accepted without requiring complex built-in capacity calculations.

To disable new registrations:
```bash
# Environment variable
export ALLOW_NEW_NAMESPACES=false

# Or in YAML config
registration:
  allowNewNamespaces: false
```

## Deployment

### ⚠️ Critical Security Requirement

**BEFORE deploying in production**, you MUST configure ArgoCD with namespace enforcement enabled:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  # CRITICAL: Enable namespace enforcement to prevent cross-tenant attacks
  application.namespaceEnforcement: "true"
```

**Without this setting, ArgoCD will ignore AppProject destination restrictions and allow cross-namespace deployments!**

Apply this configuration and restart ArgoCD:
```bash
kubectl apply -f argocd-namespace-enforcement.yaml
kubectl rollout restart deployment argocd-server -n argocd
kubectl rollout restart deployment argocd-application-controller -n argocd
```

### Kubernetes Deployment

1. **Configure ArgoCD namespace enforcement** (see above - REQUIRED for security)

2. **Apply RBAC configuration**:
   ```bash
   kubectl apply -f deploy/rbac.yaml
   ```

3. **Deploy Knative service**:
   ```bash
   kubectl apply -f deploy/knative-service.yaml
   ```

4. **Verify deployment**:
   ```bash
   kubectl get ksvc -n konflux-gitops
   ```

### Container Image

Build and push container image:

```bash
# Build with podman (optimized for macOS)
podman build -t gitops-registration-service:latest .

# Save for Kind (macOS TAR approach)
podman save gitops-registration-service:latest -o gitops-registration-service.tar

# Load into Kind cluster  
kind load image-archive gitops-registration-service.tar --name <cluster-name>
```

## Security Considerations

### RBAC Requirements

The service requires cluster-level permissions for:
- Namespace management (create, list, update, delete)
- Service account and RBAC management
- ArgoCD AppProject and Application management
- SubjectAccessReview for authorization validation

### Resource Constraints

- Configurable resource type restrictions via allow/deny lists
- Service account impersonation prevents privilege escalation
- Namespace-scoped permissions for GitOps operations
- ArgoCD AppProject-based resource filtering

### Authorization Flow (FR-008)

1. Extract Bearer token from request Authorization header
2. Validate token using TokenReview API
3. Perform SubjectAccessReview to verify user permissions on target namespace
4. Check specific permissions required by `konflux-admin-user-actions` role
5. Audit all authorization attempts with user identity and result

## Monitoring & Observability

### Prometheus Metrics

- `gitops_registrations_total` - Total number of registrations by status
- `gitops_registration_duration_seconds` - Time taken for registration operations
- `argocd_operations_total` - ArgoCD operations performed
- `registration_disabled_requests_total` - Number of requests rejected due to disabled registrations

### Health Checks

- **Liveness**: `/health/live` - Basic service health
- **Readiness**: `/health/ready` - Dependency availability (Kubernetes API, ArgoCD)

### Logging

Structured JSON logging with correlation IDs for request tracing. All authorization attempts are logged with user identity for audit purposes.

## Development

### Project Structure

```
├── cmd/server/              # Main application entry point
├── internal/
│   ├── config/             # Configuration management
│   ├── handlers/           # HTTP handlers
│   ├── server/             # HTTP server setup
│   ├── services/           # Business logic services
│   └── types/              # Data structures
├── deploy/                 # Kubernetes manifests
├── test/integration/       # Integration tests
├── Dockerfile             # Container image definition
└── README.md              # This file
```

### Testing Strategy

- **Unit Tests**: Service-level testing with mocked dependencies
- **Integration Tests**: Full workflow testing in Kind cluster
- **Security Tests**: Authorization and RBAC validation
- **Registration Control Tests**: Enable/disable functionality

### Adding New Features

1. Update types in `internal/types/`
2. Implement service logic in `internal/services/`
3. Add HTTP handlers in `internal/handlers/`
4. Update OpenAPI specification
5. Add integration tests
6. Update documentation

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes with tests
4. Run integration test suite
5. Submit pull request

## License

This project is part of the Konflux-CI ecosystem and follows the same licensing terms.

## Support

For issues and questions:
- Check existing issues in the repository
- Review the technical requirements document
- Run integration tests to verify functionality
- Check ArgoCD and Kubernetes logs for troubleshooting 