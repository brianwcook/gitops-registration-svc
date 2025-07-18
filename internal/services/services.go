package services

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/konflux-ci/gitops-registration-service/internal/config"
	"github.com/konflux-ci/gitops-registration-service/internal/types"
	"github.com/sirupsen/logrus"
)

// Constants for impersonation labels and annotations
const (
	RepositoryHashLabel = "gitops.io/repository-hash"
	ServiceAccountLabel = "gitops.io/service-account"
)

// GenerateRepositoryHash creates a consistent hash for repository URLs
func GenerateRepositoryHash(repositoryURL string) string {
	hash := sha256.Sum256([]byte(repositoryURL))
	return fmt.Sprintf("%x", hash)[:8] // Use first 8 characters for readability
}

// Services holds all service dependencies
type Services struct {
	Kubernetes          KubernetesService
	ArgoCD              ArgoCDService
	Registration        RegistrationService
	RegistrationControl RegistrationControlService
	Authorization       AuthorizationService
}

// KubernetesService interface for Kubernetes operations
type KubernetesService interface {
	HealthCheck(ctx context.Context) error
	CreateNamespace(ctx context.Context, name string, labels map[string]string) error
	CreateNamespaceWithMetadata(ctx context.Context, name string, labels, annotations map[string]string) error
	UpdateNamespaceLabels(ctx context.Context, name string, labels map[string]string) error
	UpdateNamespaceMetadata(ctx context.Context, name string, labels, annotations map[string]string) error
	DeleteNamespace(ctx context.Context, name string) error
	NamespaceExists(ctx context.Context, name string) (bool, error)
	CountNamespaces(ctx context.Context) (int, error)
	CreateServiceAccount(ctx context.Context, namespace, name string) error
	CreateRoleBinding(ctx context.Context, namespace, name, role, serviceAccount string) error
	// New impersonation methods
	ValidateClusterRole(ctx context.Context, name string) (*ClusterRoleValidation, error)
	CreateServiceAccountWithGenerateName(ctx context.Context, namespace, baseName string) (string, error)
	CreateRoleBindingForServiceAccount(ctx context.Context, namespace, name, clusterRole, serviceAccountName string) error
	CheckAppProjectConflict(ctx context.Context, repositoryHash string) (bool, error)
}

// ArgoCDService interface for ArgoCD operations
type ArgoCDService interface {
	HealthCheck(ctx context.Context) error
	CreateAppProject(ctx context.Context, project *types.AppProject) error
	DeleteAppProject(ctx context.Context, name string) error
	CreateApplication(ctx context.Context, app *types.Application) error
	DeleteApplication(ctx context.Context, name string) error
	GetApplicationStatus(ctx context.Context, name string) (*types.ApplicationStatus, error)
	// New impersonation method
	CheckAppProjectConflict(ctx context.Context, repositoryHash string) (bool, error)
}

// RegistrationService interface for registration management
type RegistrationService interface {
	CreateRegistration(ctx context.Context, req *types.RegistrationRequest) (*types.Registration, error)
	GetRegistration(ctx context.Context, id string) (*types.Registration, error)
	ListRegistrations(ctx context.Context, filters map[string]string) ([]*types.Registration, error)
	DeleteRegistration(ctx context.Context, id string) error
	RegisterExistingNamespace(
		ctx context.Context, req *types.ExistingNamespaceRequest, userInfo *types.UserInfo,
	) (*types.Registration, error)
	ValidateRegistration(ctx context.Context, req *types.RegistrationRequest) error
	ValidateExistingNamespaceRequest(ctx context.Context, req *types.ExistingNamespaceRequest) error
}

// RegistrationControlService interface for registration control
type RegistrationControlService interface {
	GetRegistrationStatus(ctx context.Context) (*types.ServiceRegistrationStatus, error)
	IsNewNamespaceAllowed(ctx context.Context) error
}

// AuthorizationService interface for authorization checks
type AuthorizationService interface {
	ValidateNamespaceAccess(ctx context.Context, userInfo *types.UserInfo, namespace string) error
	ExtractUserInfo(ctx context.Context, token string) (*types.UserInfo, error)
	IsAdminUser(userInfo *types.UserInfo) bool
}

// ClusterRoleValidation holds the result of ClusterRole validation
type ClusterRoleValidation struct {
	Exists               bool     `json:"exists"`
	HasClusterAdmin      bool     `json:"hasClusterAdmin"`
	HasNamespaceSpanning bool     `json:"hasNamespaceSpanning"`
	HasClusterScoped     bool     `json:"hasClusterScoped"`
	Warnings             []string `json:"warnings"`
	ResourceTypes        []string `json:"resourceTypes"`
}

// New creates a new Services instance using production factories
func New(cfg *config.Config, logger *logrus.Logger) (*Services, error) {
	k8sFactory := &InClusterKubernetesFactory{}
	argoCDFactory := &InClusterArgoCDFactory{}
	return NewWithFactories(cfg, logger, k8sFactory, argoCDFactory)
}

// NewWithFactories creates a new Services instance using the provided factories
func NewWithFactories(cfg *config.Config, logger *logrus.Logger, k8sFactory KubernetesClientFactory, argoCDFactory ArgoCDClientFactory) (*Services, error) {
	// Initialize Kubernetes service using factory
	k8sService, err := NewKubernetesServiceWithFactory(cfg, logger, k8sFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes service: %w", err)
	}

	// Initialize ArgoCD service using factory
	argoCDService, err := NewArgoCDServiceWithFactory(cfg, logger, argoCDFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create argocd service: %w", err)
	}

	// Initialize Authorization service
	authService := NewAuthorizationService(cfg, k8sService, logger)

	// Initialize RegistrationControl service
	registrationControlService := NewRegistrationControlService(cfg, logger)

	// Initialize Registration service (real implementation)
	registrationService := NewRegistrationServiceReal(cfg, k8sService, argoCDService, logger)

	return &Services{
		Kubernetes:          k8sService,
		ArgoCD:              argoCDService,
		Registration:        registrationService,
		RegistrationControl: registrationControlService,
		Authorization:       authService,
	}, nil
}
