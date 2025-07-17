package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/konflux-ci/gitops-registration-service/internal/config"
	"github.com/konflux-ci/gitops-registration-service/internal/types"
)

// NamespaceConflictError represents a namespace already exists error
type NamespaceConflictError struct {
	Namespace string
}

func (e *NamespaceConflictError) Error() string {
	return fmt.Sprintf("namespace %s already exists", e.Namespace)
}

// registrationService is the real implementation of RegistrationService
type registrationService struct {
	cfg    *config.Config
	k8s    KubernetesService
	argocd ArgoCDService
	logger *logrus.Logger
}

// NewRegistrationServiceReal creates a new real RegistrationService implementation
func NewRegistrationServiceReal(cfg *config.Config, k8s KubernetesService, argocd ArgoCDService, logger *logrus.Logger) RegistrationService {
	return &registrationService{
		cfg:    cfg,
		k8s:    k8s,
		argocd: argocd,
		logger: logger,
	}
}

func (r *registrationService) CreateRegistration(ctx context.Context, req *types.RegistrationRequest) (*types.Registration, error) {
	registrationID := uuid.New().String()

	r.logger.WithFields(logrus.Fields{
		"namespace":      req.Namespace,
		"repository":     req.Repository.URL,
		"registrationID": registrationID,
	}).Info("Creating registration")

	// Check if namespace already exists
	exists, err := r.k8s.NamespaceExists(ctx, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to check namespace existence: %w", err)
	}
	if exists {
		// Return a structured error that can be handled with appropriate HTTP status
		return nil, &NamespaceConflictError{Namespace: req.Namespace}
	}

	// Create registration record
	registration := &types.Registration{
		ID:        registrationID,
		Namespace: req.Namespace,
		Repository: types.Repository{
			URL:    req.Repository.URL,
			Branch: req.Repository.Branch,
		},
		Status: types.RegistrationStatus{
			Phase:   "creating",
			Message: "Registration in progress",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Labels: map[string]string{
			"gitops.io/managed-by":         "gitops-registration-service",
			"app.kubernetes.io/managed-by": "gitops-registration-service",
		},
	}

	// Step 1: Create namespace
	r.logger.WithField("namespace", req.Namespace).Info("Creating namespace")

	// Create a valid label-safe repository identifier
	repoHash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.Repository.URL)))[:8]

	namespaceLabels := map[string]string{
		"gitops.io/registration-id":    registrationID[:8], // Truncate to fit label limits
		"gitops.io/repository-hash":    repoHash,
		"gitops.io/managed-by":         "gitops-registration-service",
		"app.kubernetes.io/managed-by": "gitops-registration-service",
	}

	if err := r.k8s.CreateNamespace(ctx, req.Namespace, namespaceLabels); err != nil {
		registration.Status.Phase = "failed"
		registration.Status.Message = fmt.Sprintf("Failed to create namespace: %v", err)
		return nil, fmt.Errorf("failed to create namespace: %w", err)
	}

	// Step 2: Create service account
	serviceAccountName := "gitops"
	if err := r.k8s.CreateServiceAccount(ctx, req.Namespace, serviceAccountName); err != nil {
		registration.Status.Phase = "failed"
		registration.Status.Message = fmt.Sprintf("Failed to create service account: %v", err)
		return nil, fmt.Errorf("failed to create service account: %w", err)
	}

	// Step 3: Create role binding for the service account
	roleBindingName := "gitops-binding"
	if err := r.k8s.CreateRoleBinding(ctx, req.Namespace, roleBindingName, "gitops-role", serviceAccountName); err != nil {
		registration.Status.Phase = "failed"
		registration.Status.Message = fmt.Sprintf("Failed to create role binding: %v", err)
		return nil, fmt.Errorf("failed to create role binding: %w", err)
	}

	// Step 4: Create ArgoCD AppProject
	projectName := req.Namespace
	appProject := r.buildAppProject(projectName, req.Namespace, req.Repository.URL)

	if err := r.argocd.CreateAppProject(ctx, appProject); err != nil {
		registration.Status.Phase = "failed"
		registration.Status.Message = fmt.Sprintf("Failed to create ArgoCD AppProject: %v", err)
		return nil, fmt.Errorf("failed to create ArgoCD AppProject: %w", err)
	}

	// Step 5: Create ArgoCD Application
	appName := fmt.Sprintf("%s-app", req.Namespace)
	application := &types.Application{
		Name:    appName,
		Project: projectName,
		Source: types.ApplicationSource{
			RepoURL:        req.Repository.URL,
			TargetRevision: req.Repository.Branch,
			Path:           "manifests", // Default path where GitOps manifests are stored
		},
		Destination: types.ApplicationDestination{
			Server:    "https://kubernetes.default.svc",
			Namespace: req.Namespace,
		},
	}

	if err := r.argocd.CreateApplication(ctx, application); err != nil {
		registration.Status.Phase = "failed"
		registration.Status.Message = fmt.Sprintf("Failed to create ArgoCD Application: %v", err)
		return nil, fmt.Errorf("failed to create ArgoCD Application: %w", err)
	}

	// Update registration status
	registration.Status.Phase = "active"
	registration.Status.Message = "Registration completed successfully"
	registration.Status.ArgoCDApplication = appName
	registration.Status.ArgoCDAppProject = projectName
	registration.Status.LastSyncTime = time.Now()
	registration.Status.NamespaceCreated = true
	registration.Status.AppProjectCreated = true
	registration.Status.ApplicationCreated = true
	registration.UpdatedAt = time.Now()

	r.logger.WithFields(logrus.Fields{
		"namespace":         req.Namespace,
		"registrationID":    registrationID,
		"argoCDApplication": appName,
		"argoCDAppProject":  projectName,
	}).Info("Successfully completed registration")

	return registration, nil
}

func (r *registrationService) GetRegistration(ctx context.Context, id string) (*types.Registration, error) {
	// For now, return a simple stub - in a real implementation this would query a database
	return &types.Registration{
		ID: id,
		Status: types.RegistrationStatus{
			Phase:   "active",
			Message: "Registration found",
		},
	}, nil
}

func (r *registrationService) ListRegistrations(ctx context.Context, filters map[string]string) ([]*types.Registration, error) {
	// For now, return empty list - in a real implementation this would query a database
	return []*types.Registration{}, nil
}

func (r *registrationService) DeleteRegistration(ctx context.Context, id string) error {
	// For now, return nil - in a real implementation this would clean up resources
	r.logger.WithField("registrationID", id).Info("Registration deletion (stub)")
	return nil
}

func (r *registrationService) RegisterExistingNamespace(ctx context.Context, req *types.ExistingNamespaceRequest, userInfo *types.UserInfo) (*types.Registration, error) {
	registrationID := uuid.New().String()

	r.logger.WithFields(logrus.Fields{
		"namespace":      req.ExistingNamespace,
		"repository":     req.Repository.URL,
		"registrationID": registrationID,
		"user":           userInfo.Username,
	}).Info("Converting existing namespace to GitOps management")

	// Check if namespace exists (required for existing namespace registration)
	exists, err := r.k8s.NamespaceExists(ctx, req.ExistingNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to check namespace existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("namespace %s does not exist", req.ExistingNamespace)
	}

	// Create registration record
	registration := &types.Registration{
		ID:        registrationID,
		Namespace: req.ExistingNamespace,
		Repository: types.Repository{
			URL:    req.Repository.URL,
			Branch: req.Repository.Branch,
		},
		Status: types.RegistrationStatus{
			Phase:   "creating",
			Message: "Converting existing namespace to GitOps management",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Labels: map[string]string{
			"gitops.io/managed-by":         "gitops-registration-service",
			"app.kubernetes.io/managed-by": "gitops-registration-service",
		},
	}

	// Create a valid label-safe repository identifier
	repoHash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.Repository.URL)))[:8]

	// Step 1: Create service account in existing namespace
	r.logger.WithField("namespace", req.ExistingNamespace).Info("Creating service account in existing namespace")
	serviceAccountName := "gitops"
	if err := r.k8s.CreateServiceAccount(ctx, req.ExistingNamespace, serviceAccountName); err != nil {
		registration.Status.Phase = "failed"
		registration.Status.Message = fmt.Sprintf("Failed to create service account: %v", err)
		return nil, fmt.Errorf("failed to create service account: %w", err)
	}

	// Step 2: Create role binding for the service account
	roleBindingName := "gitops-binding"
	if err := r.k8s.CreateRoleBinding(ctx, req.ExistingNamespace, roleBindingName, "gitops-role", serviceAccountName); err != nil {
		registration.Status.Phase = "failed"
		registration.Status.Message = fmt.Sprintf("Failed to create role binding: %v", err)
		return nil, fmt.Errorf("failed to create role binding: %w", err)
	}

	// Step 3: Add GitOps labels to existing namespace
	r.logger.WithField("namespace", req.ExistingNamespace).Info("Adding GitOps labels to existing namespace")
	namespaceLabels := map[string]string{
		"gitops.io/registration-id":    registrationID[:8], // Truncate to fit label limits
		"gitops.io/repository-hash":    repoHash,
		"gitops.io/managed-by":         "gitops-registration-service",
		"app.kubernetes.io/managed-by": "gitops-registration-service",
	}

	if err := r.k8s.UpdateNamespaceLabels(ctx, req.ExistingNamespace, namespaceLabels); err != nil {
		r.logger.WithError(err).WithField("namespace", req.ExistingNamespace).Warn("Failed to update namespace labels, continuing...")
		// Not a fatal error - we can continue without the labels
	}

	// Step 4: Create ArgoCD AppProject
	projectName := req.ExistingNamespace
	appProject := r.buildAppProject(projectName, req.ExistingNamespace, req.Repository.URL)

	if err := r.argocd.CreateAppProject(ctx, appProject); err != nil {
		registration.Status.Phase = "failed"
		registration.Status.Message = fmt.Sprintf("Failed to create ArgoCD AppProject: %v", err)
		return nil, fmt.Errorf("failed to create ArgoCD AppProject: %w", err)
	}

	// Step 5: Create ArgoCD Application
	appName := fmt.Sprintf("%s-app", req.ExistingNamespace)
	application := &types.Application{
		Name:    appName,
		Project: projectName,
		Source: types.ApplicationSource{
			RepoURL:        req.Repository.URL,
			TargetRevision: req.Repository.Branch,
			Path:           "manifests", // Default path where GitOps manifests are stored
		},
		Destination: types.ApplicationDestination{
			Server:    "https://kubernetes.default.svc",
			Namespace: req.ExistingNamespace,
		},
	}

	if err := r.argocd.CreateApplication(ctx, application); err != nil {
		registration.Status.Phase = "failed"
		registration.Status.Message = fmt.Sprintf("Failed to create ArgoCD Application: %v", err)
		return nil, fmt.Errorf("failed to create ArgoCD Application: %w", err)
	}

	// Update registration status
	registration.Status.Phase = "active"
	registration.Status.Message = "Existing namespace successfully converted to GitOps management"
	registration.Status.ArgoCDApplication = appName
	registration.Status.ArgoCDAppProject = projectName
	registration.Status.LastSyncTime = time.Now()
	registration.Status.NamespaceCreated = false // Existing namespace, not created by us
	registration.Status.AppProjectCreated = true
	registration.Status.ApplicationCreated = true
	registration.UpdatedAt = time.Now()

	r.logger.WithFields(logrus.Fields{
		"namespace":         req.ExistingNamespace,
		"registrationID":    registrationID,
		"argoCDApplication": appName,
		"argoCDAppProject":  projectName,
		"user":              userInfo.Username,
	}).Info("Successfully converted existing namespace to GitOps management")

	return registration, nil
}

func (r *registrationService) ValidateRegistration(ctx context.Context, req *types.RegistrationRequest) error {
	// Basic validation
	if req.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	if req.Repository.URL == "" {
		return fmt.Errorf("repository URL is required")
	}

	return nil
}

func (r *registrationService) ValidateExistingNamespaceRequest(ctx context.Context, req *types.ExistingNamespaceRequest) error {
	// Basic validation
	if req.ExistingNamespace == "" {
		return fmt.Errorf("existingNamespace is required")
	}
	if req.Repository.URL == "" {
		return fmt.Errorf("repository URL is required")
	}

	return nil
}

func (r *registrationService) buildAppProject(projectName, namespace, repoURL string) *types.AppProject {
	appProject := &types.AppProject{
		Name: projectName,
		Destinations: []types.AppProjectDestination{
			{
				Server:    "https://kubernetes.default.svc",
				Namespace: namespace,
			},
		},
		SourceRepos: []string{repoURL},
	}

	// Configure resource restrictions based on service-level configuration
	if len(r.cfg.Security.ResourceAllowList) > 0 {
		// If allowList is provided, use it as whitelist
		appProject.ClusterResourceWhitelist = r.convertServiceResourceRestrictions(r.cfg.Security.ResourceAllowList)
		appProject.NamespaceResourceWhitelist = r.convertServiceResourceRestrictions(r.cfg.Security.ResourceAllowList)
	} else if len(r.cfg.Security.ResourceDenyList) > 0 {
		// If denyList is provided, use it as blacklist
		appProject.ClusterResourceBlacklist = r.convertServiceResourceRestrictions(r.cfg.Security.ResourceDenyList)
		appProject.NamespaceResourceBlacklist = r.convertServiceResourceRestrictions(r.cfg.Security.ResourceDenyList)
	} else {
		// If no restrictions provided, allow all resources by not setting any whitelist
		// This is the default behavior - no restrictions
	}

	return appProject
}

// convertServiceResourceRestrictions converts service config resource restrictions to AppProject format
func (r *registrationService) convertServiceResourceRestrictions(restrictions []config.ServiceResourceRestriction) []types.AppProjectResource {
	result := make([]types.AppProjectResource, len(restrictions))
	for i, restriction := range restrictions {
		result[i] = types.AppProjectResource{
			Group: restriction.Group,
			Kind:  restriction.Kind,
		}
	}
	return result
}
