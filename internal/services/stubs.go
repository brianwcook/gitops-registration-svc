package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/konflux-ci/gitops-registration-service/internal/config"
	"github.com/konflux-ci/gitops-registration-service/internal/types"
	"github.com/sirupsen/logrus"
)

// Stub implementations to satisfy interfaces and allow compilation

// kubernetesServiceStub is a stub implementation of KubernetesService
type kubernetesServiceStub struct {
	logger *logrus.Logger
}

func NewKubernetesService(cfg *config.Config, logger *logrus.Logger) (KubernetesService, error) {
	return &kubernetesServiceStub{logger: logger}, nil
}

func (k *kubernetesServiceStub) HealthCheck(ctx context.Context) error {
	// TODO: Implement actual Kubernetes health check
	return nil
}

func (k *kubernetesServiceStub) CreateNamespace(ctx context.Context, name string, labels map[string]string) error {
	// TODO: Implement namespace creation
	k.logger.WithField("namespace", name).Info("Creating namespace (stub)")
	return nil
}

func (k *kubernetesServiceStub) CreateNamespaceWithMetadata(
	ctx context.Context, name string, labels, annotations map[string]string,
) error {
	log.Printf("STUB: Creating namespace %s with metadata", name)
	return nil
}

func (k *kubernetesServiceStub) DeleteNamespace(ctx context.Context, name string) error {
	// TODO: Implement namespace deletion
	k.logger.WithField("namespace", name).Info("Deleting namespace (stub)")
	return nil
}

func (k *kubernetesServiceStub) UpdateNamespaceLabels(ctx context.Context, name string, labels map[string]string) error {
	// TODO: Implement namespace label update
	k.logger.WithFields(logrus.Fields{
		"namespace": name,
		"labels":    labels,
	}).Info("Updating namespace labels (stub)")
	return nil
}

func (k *kubernetesServiceStub) UpdateNamespaceMetadata(
	ctx context.Context, name string, labels, annotations map[string]string,
) error {
	log.Printf("STUB: Updating namespace %s metadata", name)
	return nil
}

func (k *kubernetesServiceStub) NamespaceExists(ctx context.Context, name string) (bool, error) {
	// TODO: Implement namespace existence check
	return false, nil
}

func (k *kubernetesServiceStub) CountNamespaces(ctx context.Context) (int, error) {
	// TODO: Implement namespace counting
	return 5, nil // Stub value
}

func (k *kubernetesServiceStub) CreateServiceAccount(ctx context.Context, namespace, name string) error {
	// TODO: Implement service account creation
	k.logger.WithFields(logrus.Fields{
		"namespace": namespace,
		"name":      name,
	}).Info("Creating service account (stub)")
	return nil
}

func (k *kubernetesServiceStub) CreateRoleBinding(ctx context.Context, namespace, name, role, serviceAccount string) error {
	// TODO: Implement role binding creation
	k.logger.WithFields(logrus.Fields{
		"namespace":      namespace,
		"name":           name,
		"role":           role,
		"serviceAccount": serviceAccount,
	}).Info("Creating role binding (stub)")
	return nil
}

// ValidateClusterRole validates a ClusterRole (stub implementation)
func (k *kubernetesServiceStub) ValidateClusterRole(ctx context.Context, name string) (*ClusterRoleValidation, error) {
	// Return a valid ClusterRole for testing
	return &ClusterRoleValidation{
		Exists:               true,
		HasClusterAdmin:      false,
		HasNamespaceSpanning: false,
		HasClusterScoped:     false,
		Warnings:             []string{},
		ResourceTypes:        []string{"secrets", "configmaps", "deployments"},
	}, nil
}

// CreateServiceAccountWithGenerateName creates a service account with generated name (stub)
func (k *kubernetesServiceStub) CreateServiceAccountWithGenerateName(
	ctx context.Context, namespace, baseName string,
) (string, error) {
	generatedName := fmt.Sprintf("%s-%s", baseName, randomString(8))
	log.Printf("STUB: Creating service account %s in namespace %s", generatedName, namespace)
	return generatedName, nil
}

// CreateRoleBindingForServiceAccount creates a RoleBinding (stub)
func (k *kubernetesServiceStub) CreateRoleBindingForServiceAccount(
	ctx context.Context, namespace, name, clusterRole, serviceAccountName string,
) error {
	log.Printf("STUB: Creating role binding %s for service account %s in namespace %s",
		name, serviceAccountName, namespace)
	return nil
}

// CheckAppProjectConflict checks for conflicts (stub)
func (k *kubernetesServiceStub) CheckAppProjectConflict(ctx context.Context, repositoryHash string) (bool, error) {
	// Always return no conflict for testing
	return false, nil
}

// argoCDServiceStub is a stub implementation of ArgoCDService
type argoCDServiceStub struct {
	logger *logrus.Logger
}

func NewArgoCDService(cfg *config.Config, logger *logrus.Logger) (ArgoCDService, error) {
	return &argoCDServiceStub{logger: logger}, nil
}

func (a *argoCDServiceStub) HealthCheck(ctx context.Context) error {
	// TODO: Implement actual ArgoCD health check
	return nil
}

func (a *argoCDServiceStub) CreateAppProject(ctx context.Context, project *types.AppProject) error {
	// TODO: Implement AppProject creation
	a.logger.WithField("project", project.Name).Info("Creating AppProject (stub)")
	return nil
}

func (a *argoCDServiceStub) DeleteAppProject(ctx context.Context, name string) error {
	// TODO: Implement AppProject deletion
	a.logger.WithField("project", name).Info("Deleting AppProject (stub)")
	return nil
}

func (a *argoCDServiceStub) CreateApplication(ctx context.Context, app *types.Application) error {
	// TODO: Implement Application creation
	a.logger.WithField("application", app.Name).Info("Creating Application (stub)")
	return nil
}

func (a *argoCDServiceStub) DeleteApplication(ctx context.Context, name string) error {
	// TODO: Implement Application deletion
	a.logger.WithField("application", name).Info("Deleting Application (stub)")
	return nil
}

func (a *argoCDServiceStub) GetApplicationStatus(ctx context.Context, name string) (*types.ApplicationStatus, error) {
	a.logger.WithField("application", name).Info("Getting application status (stub)")
	return &types.ApplicationStatus{
		Phase:   "Synced",
		Message: "Application is healthy (stub)",
		Health:  "Healthy",
		Sync:    "Synced",
	}, nil
}

func (a *argoCDServiceStub) convertResourceListToInterface(resources []types.AppProjectResource) []interface{} {
	result := make([]interface{}, len(resources))
	for i, resource := range resources {
		result[i] = map[string]interface{}{
			"group": resource.Group,
			"kind":  resource.Kind,
		}
	}
	return result
}

// CheckAppProjectConflict checks for repository conflicts (stub)
func (a *argoCDServiceStub) CheckAppProjectConflict(ctx context.Context, repositoryHash string) (bool, error) {
	// Always return no conflict for stub testing
	return false, nil
}

// authorizationServiceStub is a stub implementation of AuthorizationService
type authorizationServiceStub struct {
	cfg    *config.Config
	k8s    KubernetesService
	logger *logrus.Logger
}

func NewAuthorizationService(cfg *config.Config, k8s KubernetesService, logger *logrus.Logger) AuthorizationService {
	return &authorizationServiceStub{
		cfg:    cfg,
		k8s:    k8s,
		logger: logger,
	}
}

func (a *authorizationServiceStub) ValidateNamespaceAccess(
	ctx context.Context, userInfo *types.UserInfo, namespace string,
) error {
	log.Printf("STUB: Validating access for user %s to namespace %s", userInfo.Username, namespace)
	return nil
}

func (a *authorizationServiceStub) ExtractUserInfo(ctx context.Context, token string) (*types.UserInfo, error) {
	// TODO: Implement token validation and user info extraction
	return &types.UserInfo{
		Username: "stub-user",
		Email:    "stub@example.com",
		Groups:   []string{"stub-group"},
	}, nil
}

func (a *authorizationServiceStub) IsAdminUser(userInfo *types.UserInfo) bool {
	// TODO: Implement admin user check
	return false
}

// registrationControlServiceStub is a stub implementation of RegistrationControlService
type registrationControlServiceStub struct {
	cfg    *config.Config
	logger *logrus.Logger
}

func NewRegistrationControlService(cfg *config.Config, logger *logrus.Logger) RegistrationControlService {
	return &registrationControlServiceStub{
		cfg:    cfg,
		logger: logger,
	}
}

func (r *registrationControlServiceStub) GetRegistrationStatus(
	ctx context.Context,
) (*types.ServiceRegistrationStatus, error) {
	log.Printf("STUB: Getting registration status")
	return &types.ServiceRegistrationStatus{
		AllowNewNamespaces: r.cfg.Registration.AllowNewNamespaces,
		Message:            "Registration status based on configuration",
	}, nil
}

func (r *registrationControlServiceStub) IsNewNamespaceAllowed(ctx context.Context) error {
	if !r.cfg.Registration.AllowNewNamespaces {
		return errors.New("new namespace registration is currently disabled")
	}
	return nil
}

// registrationServiceStub is a stub implementation of RegistrationService
type registrationServiceStub struct {
	cfg    *config.Config
	k8s    KubernetesService
	argocd ArgoCDService
	logger *logrus.Logger
}

func NewRegistrationService(
	cfg *config.Config, k8s KubernetesService, argocd ArgoCDService, logger *logrus.Logger,
) RegistrationService {
	log.Printf("STUB: Creating registration service")
	return &registrationServiceStub{cfg: cfg, k8s: k8s, argocd: argocd, logger: logger}
}

func (r *registrationServiceStub) CreateRegistration(
	ctx context.Context, req *types.RegistrationRequest,
) (*types.Registration, error) {
	log.Printf("STUB: Creating registration for namespace %s", req.Namespace)
	return &types.Registration{
		ID:        "stub-reg-123",
		Namespace: req.Namespace,
		Repository: types.Repository{
			URL:    req.Repository.URL,
			Branch: req.Repository.Branch,
		},
		Status: types.RegistrationStatus{
			Phase:   "pending",
			Message: "Registration created (stub)",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Labels: map[string]string{
			"gitops.io/managed-by": "gitops-registration-service",
		},
	}, nil
}

func (r *registrationServiceStub) GetRegistration(ctx context.Context, id string) (*types.Registration, error) {
	// TODO: Implement registration retrieval
	return nil, errors.New("registration not found (stub)")
}

func (r *registrationServiceStub) ListRegistrations(
	ctx context.Context, filters map[string]string,
) ([]*types.Registration, error) {
	log.Printf("STUB: Listing registrations")
	return []*types.Registration{}, nil
}

func (r *registrationServiceStub) DeleteRegistration(ctx context.Context, id string) error {
	// TODO: Implement registration deletion
	r.logger.WithField("id", id).Info("Deleting registration (stub)")
	return nil
}

func (r *registrationServiceStub) RegisterExistingNamespace(
	ctx context.Context, req *types.ExistingNamespaceRequest, userInfo *types.UserInfo,
) (*types.Registration, error) {
	log.Printf("STUB: Registering existing namespace %s for user %s",
		req.ExistingNamespace, userInfo.Username)
	return &types.Registration{
		ID:        "stub-existing-reg-123",
		Namespace: req.ExistingNamespace,
		Repository: types.Repository{
			URL:    req.Repository.URL,
			Branch: req.Repository.Branch,
		},
		Status: types.RegistrationStatus{
			Phase:              "active",
			Message:            "Existing namespace registered (stub)",
			NamespaceCreated:   false, // Existing namespace, not created by us
			AppProjectCreated:  true,
			ApplicationCreated: true,
			ArgoCDApplication:  fmt.Sprintf("%s-app", req.ExistingNamespace),
			ArgoCDAppProject:   req.ExistingNamespace,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Labels: map[string]string{
			"gitops.io/managed-by": "gitops-registration-service",
		},
	}, nil
}

func (r *registrationServiceStub) ValidateRegistration(ctx context.Context, req *types.RegistrationRequest) error {
	r.logger.Info("Validating registration (stub)")

	// Basic validation
	if req.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	if req.Repository.URL == "" {
		return fmt.Errorf("repository URL is required")
	}

	return nil
}

func (r *registrationServiceStub) ValidateExistingNamespaceRequest(
	ctx context.Context, req *types.ExistingNamespaceRequest,
) error {
	log.Printf("STUB: Validating existing namespace request for %s", req.ExistingNamespace)
	return nil
}

// randomString generates a random string of the specified length
func randomString(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(bytes)[:length]
}
