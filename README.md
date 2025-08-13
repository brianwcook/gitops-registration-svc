# GitOps Registration Service

## What this service does
The GitOps Registration Service securely enables self-service onboarding of Git repositories into a Kubernetes cluster with ArgoCD. For each repository you register, it:

- Creates or updates a tenant namespace with GitOps metadata
- Provisions a dedicated ServiceAccount for ArgoCD sync (impersonation)
- Binds a ClusterRole to that ServiceAccount using a namespaced RoleBinding
- Creates an ArgoCD AppProject and Application targeting the tenant namespace
- Enforces repository isolation and optional resource-type restrictions

This design prevents privilege escalation and cross‑tenant access by using per‑tenant ServiceAccounts and ArgoCD namespace enforcement.

## How to use it

### Prerequisites
- A Kubernetes cluster with ArgoCD installed and namespace enforcement enabled (`application.namespaceEnforcement: "true"` in `argocd-cm`).
- A pullable container image for the service (for example `quay.io/bcook/gitops-registration:dev`).
- Cluster-level RBAC to allow the service to manage namespaces, service accounts, role bindings, and ArgoCD resources.

### Deploy
1) Apply RBAC and configuration
```bash
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/configmap.yaml
```

2) Deploy the Knative Service (or use your preferred Deployment)
```bash
kubectl apply -f deploy/knative-service.yaml
```

3) Verify the service
```bash
kubectl get ksvc -n konflux-gitops
curl -s http://gitops-registration.konflux-gitops.svc.cluster.local/health/ready
```

### Register a repository
Create a registration to onboard a repo into a namespace (created if new):
```bash
curl -X POST http://<service-url>/api/v1/registrations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "repository": { "url": "https://github.com/org/config", "branch": "main" },
    "namespace": "team-alpha"
  }'
```

To register an existing namespace with the same repo model:
```bash
curl -X POST http://<service-url>/api/v1/registrations/existing \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "repository": { "url": "https://github.com/org/config", "branch": "main" },
    "existingNamespace": "existing-team"
  }'
```

### Configuration essentials
- **ArgoCD**
  - `argocd.namespace` must match your ArgoCD install (default `argocd`).
  - Enable namespace enforcement in `argocd-cm`.
- **Security**
  - `security.impersonation.enabled: true` to use per‑tenant ServiceAccounts.
  - `security.impersonation.clusterRole: <name>` ClusterRole bound to tenant SAs.
  - `security.serviceAccountNamespace: gitops-registrations-sa` Dedicated namespace where tenant SAs are created (default).
- **Registration control**
  - `registration.allowNewNamespaces: true|false` to enable/disable onboarding.

### Image and registry
Build and push the image, then point deployment manifests to it:
```bash
podman login quay.io
podman build -t quay.io/<org>/gitops-registration:dev .
podman push quay.io/<org>/gitops-registration:dev
```

### Health and metrics
- Liveness: `/health/live`
- Readiness: `/health/ready`
- Metrics: `/metrics` (Prometheus format)

### Security notes
- Service creates RoleBindings to a ClusterRole for the tenant SA; ensure least privilege.
- With impersonation enabled, ArgoCD syncs with the tenant SA, not its own.
- The service rejects conflicting registrations of the same repo (when enabled).

## Support
Open an issue in this repository with environment details and relevant logs if you encounter problems. 