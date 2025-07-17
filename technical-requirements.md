# GitOps Registration Service - Technical Requirements Document

## Executive Summary

The GitOps Registration Service is a critical component of the Konflux-CI GitOps onboarding redesign (as outlined in [ADR 47](https://raw.githubusercontent.com/konflux-ci/architecture/d91958f414e7b7cc5293bbbbe33cad9ad7552991/ADR/0047-gitops-onboarding-redesign.md)). This service provides an API for automated registration and lifecycle management of GitOps repositories as first-class tenants in Konflux-CI, implementing secure multi-tenant GitOps workflows with ArgoCD integration.

## Business Context

Konflux-CI currently suffers from a fragmented onboarding experience requiring users to choose between UI-based configuration (easy but not GitOps-compliant) and manual GitOps setup (complex and error-prone). The GitOps Registration Service addresses this by providing a unified, secure, and automated way to onboard GitOps repositories as tenants while maintaining strict security boundaries and tenant isolation.

## Architecture Overview

### Core Principles
- **1:1 Mapping**: Each GitOps repository maps to exactly one Kubernetes namespace
- **Tenant Definition**: A tenant equals a Kubernetes namespace  
- **Security-First**: Implement strict tenant isolation using ArgoCD AppProjects and service account impersonation
- **Immutable Registration**: Prevent namespace conflicts through registration immutability
- **Resource Constraints**: Limit manageable Kubernetes resources to prevent privilege escalation

### Technology Stack
- **Runtime**: Knative service for serverless, event-driven operation
- **Language**: Go for performance, strong typing, and Kubernetes ecosystem integration
- **API**: RESTful HTTP API with OpenAPI 3.0 specification
- **Security**: ArgoCD AppProject-based tenant isolation with service account impersonation
- **Testing**: Comprehensive integration tests running in Kind clusters

## Functional Requirements

### FR-001: Repository Registration
**Priority**: Critical
**Description**: Enable registration of GitOps repositories as Konflux tenants

#### Acceptance Criteria:
- Accept repository URL, namespace name, and tenant metadata
- Validate repository accessibility and structure
- Prevent duplicate registrations for the same repository
- Return registration token/identifier for future operations
- Log all registration attempts for audit purposes

#### API Specification:
```http
POST /api/v1/registrations
Content-Type: application/json

{
  "repository": {
    "url": "https://github.com/team/config",
    "branch": "main",
    "credentials": {
      "type": "token|ssh",
      "secretRef": "github-token-secret"
    }
  },
  "namespace": "team-prod",
  "tenant": {
    "name": "team-alpha",
    "description": "Alpha team production environment",
    "contacts": ["team-alpha@company.com"],
    "labels": {
      "team": "alpha",
      "environment": "production"
    }
  }
}
```

### FR-002: Namespace Provisioning  
**Priority**: Critical
**Description**: Automatically provision Kubernetes namespaces for registered repositories

#### Acceptance Criteria:
- Create namespace with specified name
- Apply standard labels and annotations
- Create namespace-scoped service account for ArgoCD impersonation
- Configure RBAC permissions for the service account
- Validate namespace naming conventions

#### Security Requirements:
- Service account limited to specified resource types: `jobs`, `cronjobs`, `secrets`, `rolebindings`
- No cluster-level permissions granted to tenant service accounts
- Namespace labels include tenant identification and registration metadata

### FR-003: ArgoCD AppProject Configuration
**Priority**: Critical  
**Description**: Configure ArgoCD AppProjects for secure tenant isolation

#### Acceptance Criteria:
- Create dedicated AppProject per registered repository
- Configure source repository restrictions
- Configure destination namespace restrictions  
- Configure allowed resource types (jobs, cronjobs, secrets, rolebindings)
- Configure service account impersonation settings
- Validate AppProject configuration before activation

#### Security Configuration:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: tenant-{namespace}
  namespace: argocd
spec:
  sourceRepos:
  - {repository-url}
  destinations:
  - namespace: {namespace}
    server: https://kubernetes.default.svc
  roles:
  - name: tenant-role
    policies:
    - p, proj:tenant-{namespace}:tenant-role, applications, sync, tenant-{namespace}/*, allow
  clusterResourceWhitelist:
  - group: ""
    kind: Namespace
  namespaceResourceWhitelist:
  - group: ""
    kind: Job
  - group: batch
    kind: Job
  - group: batch
    kind: CronJob  
  - group: ""
    kind: Secret
  - group: rbac.authorization.k8s.io
    kind: RoleBinding
```

### FR-004: ArgoCD Application Creation
**Priority**: Critical
**Description**: Create ArgoCD Applications for continuous GitOps synchronization

#### Acceptance Criteria:
- Create ArgoCD Application targeting registered repository
- Configure automatic sync policies
- Configure service account impersonation
- Set appropriate retry and timeout policies
- Enable health checks and sync status monitoring

### FR-005: Registration Lifecycle Management
**Priority**: High
**Description**: Manage the complete lifecycle of repository registrations

#### Acceptance Criteria:
- Support registration updates (limited scope)
- Support registration deactivation/reactivation
- Support complete registration deletion with cleanup
- Maintain audit trail of all lifecycle events
- Prevent orphaned resources during deletion

#### API Specifications:
```http
GET /api/v1/registrations/{id}
PUT /api/v1/registrations/{id}
DELETE /api/v1/registrations/{id}
GET /api/v1/registrations/{id}/status
POST /api/v1/registrations/{id}/sync
```

### FR-006: Configuration Validation
**Priority**: High
**Description**: Validate GitOps repository configuration and structure

#### Acceptance Criteria:
- Validate YAML schema compliance for Konflux resources
- Check for required repository structure
- Validate resource naming conventions
- Detect potential security issues in configurations
- Provide detailed validation feedback

### FR-007: Multi-Tenant Security Enforcement
**Priority**: Critical
**Description**: Enforce strict security boundaries between tenants

#### Acceptance Criteria:
- Prevent cross-namespace resource access
- Validate that tenant resources only target their assigned namespace
- Audit all resource creation attempts
- Block unauthorized resource types
- Implement resource quotas per tenant

### FR-008: Existing Namespace GitOps Conversion
**Priority**: High
**Description**: Enable users with appropriate permissions to convert existing namespaces to GitOps management

#### Acceptance Criteria:
- Validate user has `konflux-admin-user-actions` role on the specific namespace being registered
- Support registration of existing namespaces without creating new ones
- Prevent users from registering namespaces where they lack proper permissions
- Maintain audit trail of conversion attempts and authorization checks
- Ensure existing resources in namespace remain untouched during conversion

#### Authorization Requirements:
- User must have `konflux-admin-user-actions` ClusterRole bound to them in the target namespace
- Service must perform RBAC authorization check using Kubernetes SubjectAccessReview API
- Authorization check must specifically verify permissions on the target namespace
- Users with the role in different namespaces must not be able to register other namespaces

#### API Specification:
```http
POST /api/v1/registrations/existing
Content-Type: application/json
Authorization: Bearer {user-token}

{
  "repository": {
    "url": "https://github.com/team/config",
    "branch": "main",
    "credentials": {
      "type": "token",
      "secretRef": "github-token-secret"
    }
  },
  "existingNamespace": "existing-team-namespace",
  "tenant": {
    "name": "existing-team",
    "description": "Converting existing namespace to GitOps",
    "contacts": ["existing-team@company.com"]
  }
}
```

#### Security Validation Process:
1. Extract user identity from request authentication token
2. Perform SubjectAccessReview to verify user can perform required actions in target namespace:
   ```yaml
   apiVersion: authorization.k8s.io/v1
   kind: SubjectAccessReview
   spec:
     user: {extracted-username}
     resourceAttributes:
       namespace: {target-namespace}
       verb: create
       group: batch
       resource: jobs
   ```
3. Validate user has all required permissions defined in `konflux-admin-user-actions` role
4. Reject registration if any required permission is missing

### FR-009: Cluster Capacity Management
**Priority**: High
**Description**: Implement cluster capacity controls to prevent resource exhaustion while allowing existing namespace registration

#### Acceptance Criteria:
- Support configurable cluster capacity limits
- Block new namespace creation when capacity limits are reached
- Allow existing namespace registration even when at capacity
- Provide clear error messages when capacity limits prevent registration
- Support administrative override for capacity limits
- Track and report current cluster utilization metrics

#### Configuration Schema:
```yaml
capacity:
  enabled: true
  limits:
    maxNamespaces: 1000
    maxTenantsPerUser: 10
    emergencyThreshold: 0.95  # Block new namespaces at 95% capacity
  overrides:
    adminUsers: ["admin@company.com"]
    emergencyBypass: false
  monitoring:
    alertThreshold: 0.85  # Alert at 85% capacity
```

#### API Response Examples:
```json
// Capacity limit reached - new namespace
{
  "error": "CAPACITY_LIMIT_REACHED",
  "message": "Cluster has reached maximum namespace capacity (950/1000). New namespace creation is disabled. Existing namespace registration is still available.",
  "details": {
    "currentNamespaces": 950,
    "maxNamespaces": 1000,
    "utilizationPercent": 95.0,
    "alternativeAction": "Use existing namespace registration endpoint"
  }
}

// Successful existing namespace registration during capacity limit
{
  "id": "reg-existing-12345",
  "status": "active",
  "message": "Successfully registered existing namespace 'team-prod' for GitOps management",
  "namespace": "team-prod",
  "created": true,
  "capacityBypass": true
}
```

## Non-Functional Requirements

### NFR-001: Performance
- **Response Time**: API responses < 200ms for registration queries, < 5s for registration creation
- **Throughput**: Support 100 concurrent registrations
- **Scalability**: Horizontal scaling via Knative autoscaling

### NFR-002: Reliability  
- **Availability**: 99.9% uptime for registration service
- **Fault Tolerance**: Graceful handling of ArgoCD/Kubernetes API failures
- **Backup/Recovery**: Registration data recoverable from Kubernetes resources

### NFR-003: Security
- **Authentication**: Integration with Konflux-CI authentication system
- **Authorization**: RBAC-based access control for registration operations
- **Audit Logging**: Complete audit trail of all registration operations
- **Secrets Management**: Secure handling of repository credentials

### NFR-004: Observability
- **Metrics**: Prometheus metrics for registration operations, success/failure rates
- **Logging**: Structured logging with correlation IDs
- **Tracing**: OpenTelemetry tracing for request flows
- **Health Checks**: Kubernetes readiness/liveness probes

## Integration Requirements

### ArgoCD Integration
- **Version Compatibility**: ArgoCD 2.8+ 
- **API Access**: Full access to ArgoCD API for AppProject/Application management
- **Event Handling**: Subscribe to ArgoCD events for sync status monitoring
- **Custom Health Checks**: Implement Konflux-specific health checks

### Kubernetes Integration  
- **RBAC**: Cluster-admin permissions for namespace/RBAC management
- **API Access**: Native Kubernetes client-go integration
- **Event Handling**: Watch for namespace/resource events
- **CRD Support**: Support for Konflux-CI custom resources

### Git Provider Integration
- **Multi-Provider Support**: GitHub, GitLab, Gitea, Generic Git
- **Authentication**: Support for tokens, SSH keys, GitHub Apps
- **Webhook Support**: Optional webhook registration for repository events
- **Repository Validation**: Connectivity and permission validation

## Testing Requirements

### Unit Testing
- **Coverage**: Minimum 80% code coverage
- **Framework**: Go standard testing + Testify
- **Mocking**: Interface-based mocking for external dependencies
- **Test Data**: Comprehensive test fixtures for various scenarios

### Integration Testing  
- **Environment**: Kind cluster with Knative and ArgoCD
- **Test Scenarios**:
  - Full registration workflow end-to-end
  - Security boundary enforcement
  - Multi-tenant isolation validation
  - Error handling and recovery scenarios
  - Resource cleanup verification
  - Existing namespace GitOps conversion
  - RBAC authorization validation
  - Cluster capacity management

### Test Infrastructure Setup
```bash
# Integration test environment setup
kind create cluster --config kind-config.yaml
kubectl apply -f https://github.com/knative/serving/releases/download/knative-v1.12.0/serving-crds.yaml
kubectl apply -f https://github.com/knative/serving/releases/download/knative-v1.12.0/serving-core.yaml
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Setup test users and namespaces for FR-008 testing
kubectl create namespace test-namespace-alpha
kubectl create namespace test-namespace-beta
kubectl create namespace existing-namespace-convert

# Create test users with different permission levels
kubectl create serviceaccount valid-user-alpha
kubectl create serviceaccount valid-user-beta  
kubectl create serviceaccount invalid-user
kubectl create serviceaccount cross-namespace-user

# Apply konflux-admin-user-actions role (from referenced RBAC)
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
EOF

# Create valid role bindings for testing
kubectl create rolebinding valid-user-alpha-binding \
  --clusterrole=konflux-admin-user-actions \
  --serviceaccount=default:valid-user-alpha \
  --namespace=test-namespace-alpha

kubectl create rolebinding valid-user-beta-binding \
  --clusterrole=konflux-admin-user-actions \
  --serviceaccount=default:valid-user-beta \
  --namespace=test-namespace-beta

# Cross-namespace user has access only to namespace-alpha
kubectl create rolebinding cross-namespace-user-binding \
  --clusterrole=konflux-admin-user-actions \
  --serviceaccount=default:cross-namespace-user \
  --namespace=test-namespace-alpha

# Setup capacity testing environment
kubectl create configmap gitops-registration-test-config \
  --from-literal=capacity.enabled=true \
  --from-literal=capacity.limits.maxNamespaces=5 \
  --from-literal=capacity.limits.emergencyThreshold=0.8
```

#### Automated Test Scenarios

**Test Script: test-existing-namespace-authorization.sh**
```bash
#!/bin/bash
set -e

echo "Testing FR-008: Existing Namespace GitOps Conversion Authorization"

# Test 1: Valid user registering their namespace
echo "Test 1: Valid authorization - user with role in target namespace"
VALID_TOKEN=$(kubectl create token valid-user-alpha --duration=3600s)
curl -X POST http://gitops-registration-service/api/v1/registrations/existing \
  -H "Authorization: Bearer $VALID_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "repository": {"url": "https://github.com/test/valid-repo", "branch": "main"},
    "existingNamespace": "test-namespace-alpha",
    "tenant": {"name": "alpha-team", "description": "Alpha team namespace"}
  }' \
  --fail || { echo "FAIL: Valid user should be able to register their namespace"; exit 1; }
echo "PASS: Valid user successfully registered their namespace"

# Test 2: Invalid user - no permissions 
echo "Test 2: Invalid authorization - user without role"
INVALID_TOKEN=$(kubectl create token invalid-user --duration=3600s)
curl -X POST http://gitops-registration-service/api/v1/registrations/existing \
  -H "Authorization: Bearer $INVALID_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "repository": {"url": "https://github.com/test/invalid-repo", "branch": "main"},
    "existingNamespace": "test-namespace-alpha",
    "tenant": {"name": "invalid-team", "description": "Should fail"}
  }' \
  --fail && { echo "FAIL: Invalid user should not be able to register namespace"; exit 1; }
echo "PASS: Invalid user correctly rejected"

# Test 3: Cross-namespace access attempt
echo "Test 3: Cross-namespace access - user with role in different namespace"
CROSS_TOKEN=$(kubectl create token cross-namespace-user --duration=3600s)
curl -X POST http://gitops-registration-service/api/v1/registrations/existing \
  -H "Authorization: Bearer $CROSS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "repository": {"url": "https://github.com/test/cross-repo", "branch": "main"},
    "existingNamespace": "test-namespace-beta",
    "tenant": {"name": "cross-team", "description": "Should fail"}
  }' \
  --fail && { echo "FAIL: Cross-namespace access should be denied"; exit 1; }
echo "PASS: Cross-namespace access correctly denied"

echo "All FR-008 authorization tests passed!"
```

**Test Script: test-capacity-management.sh**
```bash
#!/bin/bash
set -e

echo "Testing FR-009: Cluster Capacity Management"

# Set up capacity limit of 5 namespaces
CURRENT_NS_COUNT=$(kubectl get namespaces --no-headers | wc -l)
echo "Current namespace count: $CURRENT_NS_COUNT"

# Test 1: Normal operation below capacity
echo "Test 1: Registration below capacity limit"
curl -X POST http://gitops-registration-service/api/v1/registrations \
  -H "Content-Type: application/json" \
  -d '{
    "repository": {"url": "https://github.com/test/capacity-test-1", "branch": "main"},
    "namespace": "capacity-test-1",
    "tenant": {"name": "capacity-team-1", "description": "Below capacity test"}
  }' \
  --fail || { echo "FAIL: Registration should succeed below capacity"; exit 1; }
echo "PASS: Registration succeeded below capacity"

# Create additional namespaces to reach capacity limit
for i in {2..4}; do
  kubectl create namespace "filler-namespace-$i"
done

# Test 2: New namespace creation at capacity limit
echo "Test 2: New namespace creation at capacity limit"
curl -X POST http://gitops-registration-service/api/v1/registrations \
  -H "Content-Type: application/json" \
  -d '{
    "repository": {"url": "https://github.com/test/capacity-fail", "branch": "main"},
    "namespace": "capacity-fail-new",
    "tenant": {"name": "capacity-fail-team", "description": "Should fail at capacity"}
  }' \
  --fail && { echo "FAIL: New namespace creation should be blocked at capacity"; exit 1; }
echo "PASS: New namespace creation correctly blocked at capacity"

# Test 3: Existing namespace registration at capacity limit
echo "Test 3: Existing namespace registration at capacity limit"
kubectl create namespace existing-at-capacity
VALID_TOKEN=$(kubectl create token valid-user-alpha --duration=3600s)
kubectl create rolebinding existing-at-capacity-binding \
  --clusterrole=konflux-admin-user-actions \
  --serviceaccount=default:valid-user-alpha \
  --namespace=existing-at-capacity

curl -X POST http://gitops-registration-service/api/v1/registrations/existing \
  -H "Authorization: Bearer $VALID_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "repository": {"url": "https://github.com/test/existing-capacity", "branch": "main"},
    "existingNamespace": "existing-at-capacity",
    "tenant": {"name": "existing-capacity-team", "description": "Should succeed"}
  }' \
  --fail || { echo "FAIL: Existing namespace registration should succeed at capacity"; exit 1; }
echo "PASS: Existing namespace registration succeeded at capacity"

echo "All FR-009 capacity management tests passed!"
```

### Security Testing
- **Tenant Isolation**: Verify strict namespace boundaries
- **Resource Restrictions**: Validate allowed resource type enforcement  
- **Privilege Escalation**: Test prevention of unauthorized access
- **RBAC Validation**: Verify service account permissions
- **Input Validation**: Test API input sanitization and validation

#### FR-008 Security Test Scenarios:
1. **Valid Authorization Test**:
   - User with `konflux-admin-user-actions` role in target namespace successfully registers existing namespace
   - Verify SubjectAccessReview API is called correctly
   - Confirm existing namespace resources remain unchanged

2. **Invalid Authorization Tests**:
   - User without any role cannot register any namespace (should fail)
   - User with `konflux-admin-user-actions` role in namespace-A cannot register namespace-B (should fail)
   - User with different role (e.g., read-only) in target namespace cannot register it (should fail)
   - Anonymous/unauthenticated requests to existing namespace endpoint (should fail)

3. **Permission Boundary Tests**:
   - Test each permission in `konflux-admin-user-actions` role individually
   - Verify service correctly validates all required permissions
   - Test partial permissions (user missing some required actions) - should fail

4. **Audit and Logging Tests**:
   - Verify all authorization attempts are logged with user identity and namespace
   - Failed authorization attempts include detailed reason for denial
   - Successful registrations log the validated permissions

#### FR-009 Capacity Management Test Scenarios:
1. **Capacity Enforcement Tests**:
   - Normal operation below capacity threshold
   - Block new namespace creation at capacity limit
   - Allow existing namespace registration at capacity limit
   - Administrative override functionality

2. **Configuration Tests**:
   - Dynamic capacity limit updates
   - Emergency threshold behavior
   - Per-user tenant limit enforcement

3. **Edge Case Tests**:
   - Concurrent registration attempts at capacity limit
   - Capacity calculation accuracy during namespace deletion
   - Behavior when capacity limits are disabled/enabled

### Test GitOps Repositories
Create several test repositories to validate different scenarios:

#### Repository 1: Valid Multi-Resource Repository
```
test-repos/valid-multi-resource/
├── konflux/
│   ├── applications/
│   │   └── web-app.yaml
│   ├── components/
│   │   ├── frontend.yaml
│   │   └── backend.yaml
│   ├── integrationtests/
│   │   └── smoke-test.yaml
│   └── releases/
│       └── production.yaml
└── README.md
```

#### Repository 2: Security Violation Repository
```
test-repos/security-violation/
├── konflux/
│   ├── applications/
│   │   └── malicious-app.yaml  # Contains cluster-admin role
│   └── components/
│       └── privileged-component.yaml  # Attempts cross-namespace access
└── README.md
```

#### Repository 3: Invalid Configuration Repository  
```
test-repos/invalid-config/
├── konflux/
│   ├── applications/
│   │   └── broken-app.yaml  # Invalid YAML schema
│   └── components/
│       └── missing-required-fields.yaml
└── README.md
```

### Podman Integration for macOS
Given macOS usage, implement TAR-based image handling:

```bash
# Build and save service image
podman build -t gitops-registration-service:latest .
podman save gitops-registration-service:latest -o gitops-registration-service.tar

# Load in Kind cluster
kind load image-archive gitops-registration-service.tar --name konflux-test
```

## Configuration Management

### Service Configuration
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gitops-registration-config
data:
  config.yaml: |
    server:
      port: 8080
      timeout: 30s
    argocd:
      server: "argocd-server.argocd.svc.cluster.local"
      namespace: "argocd"
      grpc: true
    kubernetes:
      namespace: "gitops-registration-system"
    security:
      allowedResourceTypes:
      - jobs
      - cronjobs  
      - secrets
      - rolebindings
      requireAppProjectPerTenant: true
      enableServiceAccountImpersonation: true
    tenants:
      namespacePrefix: ""
      defaultResourceQuota:
        requests.cpu: "1"
        requests.memory: "2Gi"
        limits.cpu: "4"  
        limits.memory: "8Gi"
        persistentvolumeclaims: "10"
    capacity:
      enabled: true
      limits:
        maxNamespaces: 1000
        maxTenantsPerUser: 10
        emergencyThreshold: 0.95
      overrides:
        adminUsers: ["admin@company.com"]
        emergencyBypass: false
      monitoring:
        alertThreshold: 0.85
    authorization:
      requiredRole: "konflux-admin-user-actions"
      enableSubjectAccessReview: true
      auditFailedAttempts: true
```

## API Specification

### OpenAPI 3.0 Schema
```yaml
openapi: 3.0.0
info:
  title: GitOps Registration Service API
  version: 1.0.0
  description: API for registering GitOps repositories as Konflux tenants

paths:
  /api/v1/registrations:
    post:
      summary: Register a new GitOps repository
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/RegistrationRequest'
      responses:
        '201':
          description: Registration created successfully
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Registration'
        '400':
          description: Invalid request
        '409':
          description: Repository already registered
    get:
      summary: List all registrations
      parameters:
      - name: namespace
        in: query
        schema:
          type: string
      responses:
        '200':
          description: List of registrations
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Registration'

  /api/v1/registrations/{id}:
    get:
      summary: Get registration details
      parameters:
      - name: id
        in: path
        required: true
        schema:
          type: string
      responses:
        '200':
          description: Registration details
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Registration'
        '404':
          description: Registration not found
    delete:
      summary: Delete registration
      parameters:
      - name: id
        in: path
        required: true
        schema:
          type: string
      responses:
        '204':
          description: Registration deleted successfully
        '404':
          description: Registration not found

  /api/v1/registrations/existing:
    post:
      summary: Register an existing namespace for GitOps management
      security:
      - bearerAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ExistingNamespaceRegistrationRequest'
      responses:
        '201':
          description: Existing namespace registered successfully
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Registration'
        '400':
          description: Invalid request
        '403':
          description: Insufficient permissions for target namespace
        '409':
          description: Namespace already registered

  /api/v1/capacity:
    get:
      summary: Get current cluster capacity information
      responses:
        '200':
          description: Cluster capacity status
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/CapacityStatus'
    put:
      summary: Update capacity limits (admin only)
      security:
      - bearerAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CapacityConfiguration'
      responses:
        '200':
          description: Capacity limits updated
        '403':
          description: Admin privileges required

components:
  schemas:
    RegistrationRequest:
      type: object
      required:
      - repository
      - namespace
      - tenant
      properties:
        repository:
          $ref: '#/components/schemas/Repository'
        namespace:
          type: string
          pattern: '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$'
        tenant:
          $ref: '#/components/schemas/Tenant'

    Repository:
      type: object
      required:
      - url
      properties:
        url:
          type: string
          format: uri
        branch:
          type: string
          default: main
        credentials:
          $ref: '#/components/schemas/Credentials'

    Tenant:
      type: object
      required:
      - name
      properties:
        name:
          type: string
        description:
          type: string
        contacts:
          type: array
          items:
            type: string
            format: email
        labels:
          type: object
          additionalProperties:
            type: string

    Registration:
      type: object
      properties:
        id:
          type: string
        repository:
          $ref: '#/components/schemas/Repository'
        namespace:
          type: string
        tenant:
          $ref: '#/components/schemas/Tenant'
        status:
          $ref: '#/components/schemas/RegistrationStatus'
        createdAt:
          type: string
          format: date-time
        updatedAt:
          type: string
          format: date-time

    RegistrationStatus:
      type: object
      properties:
        phase:
          type: string
          enum: [pending, active, failed, deleting]
        message:
          type: string
        argocdApplication:
          type: string
        argocdAppProject:
          type: string
        lastSyncTime:
          type: string
          format: date-time

    ExistingNamespaceRegistrationRequest:
      type: object
      required:
      - repository
      - existingNamespace
      - tenant
      properties:
        repository:
          $ref: '#/components/schemas/Repository'
        existingNamespace:
          type: string
          pattern: '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$'
          description: Name of existing namespace to register for GitOps management
        tenant:
          $ref: '#/components/schemas/Tenant'

    CapacityStatus:
      type: object
      properties:
        enabled:
          type: boolean
        current:
          type: object
          properties:
            namespaces:
              type: integer
            tenants:
              type: integer
            utilizationPercent:
              type: number
              format: float
        limits:
          type: object
          properties:
            maxNamespaces:
              type: integer
            maxTenantsPerUser:
              type: integer
            emergencyThreshold:
              type: number
              format: float
        status:
          type: string
          enum: [normal, warning, capacity_reached, emergency]
        message:
          type: string
        allowNewNamespaces:
          type: boolean
          description: Whether new namespace creation is currently allowed
        allowExistingNamespaces:
          type: boolean
          description: Whether existing namespace registration is currently allowed

    CapacityConfiguration:
      type: object
      properties:
        enabled:
          type: boolean
        limits:
          type: object
          properties:
            maxNamespaces:
              type: integer
              minimum: 1
            maxTenantsPerUser:
              type: integer
              minimum: 1
            emergencyThreshold:
              type: number
              format: float
              minimum: 0.5
              maximum: 1.0
        overrides:
          type: object
          properties:
            adminUsers:
              type: array
              items:
                type: string
                format: email
            emergencyBypass:
              type: boolean
        monitoring:
          type: object
          properties:
            alertThreshold:
              type: number
              format: float
              minimum: 0.1
              maximum: 1.0

  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
```

## Implementation Roadmap

### Phase 1: Core Service Development (4 weeks)
- [ ] Basic Knative service structure
- [ ] Kubernetes client integration  
- [ ] REST API implementation
- [ ] Basic registration workflow
- [ ] Unit test coverage

### Phase 2: ArgoCD Integration (3 weeks)
- [ ] ArgoCD client integration
- [ ] AppProject creation and management
- [ ] Application creation and management
- [ ] Service account impersonation setup
- [ ] Integration tests with ArgoCD

### Phase 3: Security Implementation (3 weeks)  
- [ ] Resource type restrictions
- [ ] Tenant isolation enforcement
- [ ] RBAC configuration
- [ ] Security validation and testing
- [ ] Audit logging

### Phase 4: Enhanced Features (3 weeks)
- [ ] Repository validation
- [ ] Configuration schema validation
- [ ] Lifecycle management APIs
- [ ] **FR-008: Existing Namespace GitOps Conversion**
  - [ ] SubjectAccessReview API integration
  - [ ] User authorization validation
  - [ ] Cross-namespace security enforcement
  - [ ] Comprehensive RBAC testing
- [ ] **FR-009: Cluster Capacity Management**
  - [ ] Namespace counting and monitoring
  - [ ] Capacity threshold enforcement
  - [ ] Administrative override mechanisms
  - [ ] Capacity API endpoints
- [ ] Observability and monitoring
- [ ] Performance optimization

### Phase 5: Testing and Documentation (2 weeks)
- [ ] Comprehensive integration test suite
- [ ] Security penetration testing
- [ ] Performance benchmarking
- [ ] Documentation and examples
- [ ] Deployment automation

## Deployment Architecture

### Knative Service Configuration
```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: gitops-registration-service
  namespace: konflux-gitops
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/minScale: "1"
        autoscaling.knative.dev/maxScale: "10"
    spec:
      serviceAccountName: gitops-registration-sa
      containers:
      - image: quay.io/konflux/gitops-registration-service:latest
        ports:
        - containerPort: 8080
        env:
        - name: CONFIG_PATH
          value: /etc/config/config.yaml
        - name: ARGOCD_SERVER
          value: argocd-server.argocd.svc.cluster.local:443
        volumeMounts:
        - name: config
          mountPath: /etc/config
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
      volumes:
      - name: config
        configMap:
          name: gitops-registration-config
```

### RBAC Configuration
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: gitops-registration-sa
  namespace: konflux-gitops
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: gitops-registration-controller
rules:
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["create", "get", "list", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["serviceaccounts"]
  verbs: ["create", "get", "list", "update", "patch", "delete"]
- apiGroups: ["rbac.authorization.k8s.io"]
  resources: ["roles", "rolebindings"]
  verbs: ["create", "get", "list", "update", "patch", "delete"]
- apiGroups: ["argoproj.io"]
  resources: ["appprojects", "applications"]
  verbs: ["create", "get", "list", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: gitops-registration-controller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gitops-registration-controller
subjects:
- kind: ServiceAccount
  name: gitops-registration-sa
  namespace: konflux-gitops
```

## Monitoring and Observability

### Prometheus Metrics
```go
var (
    registrationTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "gitops_registrations_total",
            Help: "Total number of GitOps registrations",
        },
        []string{"status", "namespace"},
    )
    
    registrationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "gitops_registration_duration_seconds", 
            Help: "Time taken to complete registration",
        },
        []string{"operation"},
    )

    argoCDOperationTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "argocd_operations_total",
            Help: "Total ArgoCD operations performed",
        },
        []string{"operation", "status"},
    )
)
```

### Health Check Endpoints
```go
// /health/live - Liveness probe
func (s *Server) healthLive(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "ok",
        "timestamp": time.Now().UTC().Format(time.RFC3339),
    })
}

// /health/ready - Readiness probe  
func (s *Server) healthReady(w http.ResponseWriter, r *http.Request) {
    if err := s.checkDependencies(); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]string{
            "status": "not ready",
            "error": err.Error(),
        })
        return
    }
    
    w.WriteHeader(http.StatusOK) 
    json.NewEncoder(w).Encode(map[string]string{
        "status": "ready",
        "timestamp": time.Now().UTC().Format(time.RFC3339),
    })
}
```

## Risk Assessment and Mitigation

### High Risks
1. **Security Boundary Violation**
   - *Risk*: ArgoCD misconfiguration allows cross-tenant access
   - *Mitigation*: Comprehensive security testing, AppProject validation, service account impersonation

2. **Resource Exhaustion**  
   - *Risk*: Malicious repositories consume excessive cluster resources
   - *Mitigation*: Resource quotas, admission controllers, monitoring alerts

3. **ArgoCD Dependency**
   - *Risk*: Service becomes unusable if ArgoCD fails
   - *Mitigation*: ArgoCD high availability, fallback modes, health monitoring

### Medium Risks
1. **Repository Access Issues**
   - *Risk*: Credential management complexity
   - *Mitigation*: Multiple authentication methods, credential validation, secure storage

2. **Performance Degradation**
   - *Risk*: Service becomes slow under load
   - *Mitigation*: Knative autoscaling, performance testing, monitoring

3. **Authorization Bypass (FR-008)**
   - *Risk*: User gains unauthorized access to namespaces through RBAC misconfiguration
   - *Mitigation*: Comprehensive SubjectAccessReview validation, extensive security testing, audit logging

4. **Capacity Miscalculation (FR-009)**
   - *Risk*: Incorrect capacity tracking leads to cluster resource exhaustion
   - *Mitigation*: Real-time namespace monitoring, conservative thresholds, administrative overrides

### Low Risks
1. **Existing Resource Conflicts**
   - *Risk*: Converting existing namespaces conflicts with pre-existing ArgoCD applications
   - *Mitigation*: Pre-conversion validation, conflict detection, rollback procedures

2. **Capacity Configuration Drift**
   - *Risk*: Capacity limits become outdated as cluster scales
   - *Mitigation*: Automated monitoring alerts, regular capacity reviews, dynamic adjustment APIs

## Conclusion

The GitOps Registration Service represents a critical component in Konflux-CI's evolution toward a unified GitOps-centric onboarding experience. By implementing secure multi-tenant capabilities with ArgoCD integration, this service will significantly improve developer productivity while maintaining strict security boundaries.

The focus on AppProject-based security implementation, comprehensive testing infrastructure, and careful resource limitation ensures that the service meets enterprise security requirements while providing a streamlined developer experience aligned with GitOps principles.

## References

- [ADR 47: GitOps Onboarding Redesign](https://raw.githubusercontent.com/konflux-ci/architecture/d91958f414e7b7cc5293bbbbe33cad9ad7552991/ADR/0047-gitops-onboarding-redesign.md)
- [ArgoCD Application Sync Using Impersonation](https://argo-cd.readthedocs.io/en/latest/operator-manual/app-sync-using-impersonation/)
- [Leveraging ArgoCD in Multi-tenanted Platforms](https://www.cecg.io/blog/multi-tenant-argocd/)
- [Konflux-CI Documentation](https://konflux-ci.dev/docs/)
- [Knative Serving Documentation](https://knative.dev/docs/serving/) 