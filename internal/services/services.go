package services

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/konflux-ci/gitops-registration-service/internal/config"
	"github.com/konflux-ci/gitops-registration-service/internal/types"
)

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
	UpdateNamespaceLabels(ctx context.Context, name string, labels map[string]string) error
	DeleteNamespace(ctx context.Context, name string) error
	NamespaceExists(ctx context.Context, name string) (bool, error)
	CountNamespaces(ctx context.Context) (int, error)
	CreateServiceAccount(ctx context.Context, namespace, name string) error
	CreateRoleBinding(ctx context.Context, namespace, name, role, serviceAccount string) error
}

// ArgoCDService interface for ArgoCD operations
type ArgoCDService interface {
	HealthCheck(ctx context.Context) error
	CreateAppProject(ctx context.Context, project *types.AppProject) error
	DeleteAppProject(ctx context.Context, name string) error
	CreateApplication(ctx context.Context, app *types.Application) error
	DeleteApplication(ctx context.Context, name string) error
	GetApplicationStatus(ctx context.Context, name string) (*types.ApplicationStatus, error)
}

// RegistrationService interface for registration management
type RegistrationService interface {
	CreateRegistration(ctx context.Context, req *types.RegistrationRequest) (*types.Registration, error)
	GetRegistration(ctx context.Context, id string) (*types.Registration, error)
	ListRegistrations(ctx context.Context, filters map[string]string) ([]*types.Registration, error)
	DeleteRegistration(ctx context.Context, id string) error
	RegisterExistingNamespace(ctx context.Context, req *types.ExistingNamespaceRequest, userInfo *types.UserInfo) (*types.Registration, error)
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

// New creates a new Services instance
func New(cfg *config.Config, logger *logrus.Logger) (*Services, error) {
	// Initialize Kubernetes service (real implementation)
	k8sService, err := NewKubernetesServiceReal(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes service: %w", err)
	}

	// Initialize ArgoCD service (real implementation)
	argoCDService, err := NewArgoCDServiceReal(cfg, logger)
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
