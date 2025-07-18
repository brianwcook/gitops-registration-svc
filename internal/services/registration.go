package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/konflux-ci/gitops-registration-service/internal/config"
	"github.com/konflux-ci/gitops-registration-service/internal/types"
	"github.com/sirupsen/logrus"
)

// Constants for commonly used strings
const (
	StatusFailed = "failed"
)

// NamespaceConflictError represents a namespace already exists error
type NamespaceConflictError struct {
	Namespace string
}

func (e *NamespaceConflictError) Error() string {
	return fmt.Sprintf("namespace %s already exists", e.Namespace)
}

// extractRepositoryDomain extracts a label-safe domain from a repository URL
func extractRepositoryDomain(repoURL string) string {
	parsed, err := url.Parse(repoURL)
	if err != nil {
		// If URL parsing fails, create a safe fallback
		return strings.ReplaceAll(strings.ReplaceAll(repoURL, ":", "-"), "/", "-")
	}

	domain := parsed.Host
	if domain == "" {
		domain = "unknown"
	}

	// Make domain label-safe: only alphanumeric, hyphens, dots, and underscores
	domain = strings.ReplaceAll(domain, ":", "-")

	// Truncate if too long (Kubernetes labels must be 63 chars or less)
	if len(domain) > 63 {
		domain = domain[:63]
	}

	return domain
}

// registrationService is the real implementation of RegistrationService
type registrationService struct {
	cfg    *config.Config
	k8s    KubernetesService
	argocd ArgoCDService
	logger *logrus.Logger
}

// NewRegistrationServiceReal creates a new real RegistrationService implementation
func NewRegistrationServiceReal(
	cfg *config.Config, k8s KubernetesService, argocd ArgoCDService, logger *logrus.Logger,
) RegistrationService {
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

	// Step 1: Check for repository conflicts
	if err := r.checkRepositoryConflicts(ctx, req.Repository.URL); err != nil {
		return nil, err
	}

	// Step 2: Validate namespace availability
	if err := r.validateNamespaceAvailability(ctx, req.Namespace); err != nil {
		return nil, err
	}

	// Step 3: Create registration record
	registration := r.buildRegistrationRecord(registrationID, req)

	// Step 4: Setup namespace with metadata
	if err := r.setupNamespace(ctx, req, registrationID); err != nil {
		registration.Status.Phase = StatusFailed
		registration.Status.Message = fmt.Sprintf("Failed to create namespace: %v", err)
		return nil, fmt.Errorf("failed to create namespace: %w", err)
	}

	// Step 5: Setup service account and role binding
	serviceAccountName, err := r.setupServiceAccount(ctx, req.Namespace)
	if err != nil {
		registration.Status.Phase = StatusFailed
		registration.Status.Message = fmt.Sprintf("Failed to setup service account: %v", err)
		if deleteErr := r.k8s.DeleteNamespace(ctx, req.Namespace); deleteErr != nil {
			r.logger.WithError(deleteErr).Error("Failed to cleanup namespace")
		}
		return nil, fmt.Errorf("failed to setup service account: %w", err)
	}

	// Step 6: Setup ArgoCD resources
	appName, projectName, err := r.setupArgoCDResources(ctx, req, serviceAccountName)
	if err != nil {
		registration.Status.Phase = StatusFailed
		registration.Status.Message = fmt.Sprintf("Failed to setup ArgoCD resources: %v", err)
		if deleteErr := r.k8s.DeleteNamespace(ctx, req.Namespace); deleteErr != nil {
			r.logger.WithError(deleteErr).Error("Failed to cleanup namespace")
		}
		return nil, fmt.Errorf("failed to setup ArgoCD resources: %w", err)
	}

	// Step 7: Finalize registration
	r.finalizeRegistration(registration, appName, projectName, serviceAccountName)

	r.logger.WithFields(logrus.Fields{
		"namespace":         req.Namespace,
		"registrationID":    registrationID,
		"argoCDApplication": appName,
		"argoCDAppProject":  projectName,
		"serviceAccount":    serviceAccountName,
		"impersonation":     r.cfg.Security.Impersonation.Enabled,
	}).Info("Successfully completed registration")

	return registration, nil
}

// checkRepositoryConflicts validates repository availability if impersonation is enabled
func (r *registrationService) checkRepositoryConflicts(ctx context.Context, repoURL string) error {
	if !r.cfg.Security.Impersonation.Enabled {
		return nil
	}

	repoHash := GenerateRepositoryHash(repoURL)
	conflictExists, err := r.argocd.CheckAppProjectConflict(ctx, repoHash)
	if err != nil {
		return fmt.Errorf("failed to check repository conflict: %w", err)
	}
	if conflictExists {
		return fmt.Errorf("repository %s is already registered in another AppProject", repoURL)
	}
	return nil
}

// validateNamespaceAvailability checks if the namespace already exists
func (r *registrationService) validateNamespaceAvailability(ctx context.Context, namespace string) error {
	exists, err := r.k8s.NamespaceExists(ctx, namespace)
	if err != nil {
		return fmt.Errorf("failed to check namespace existence: %w", err)
	}
	if exists {
		return &NamespaceConflictError{Namespace: namespace}
	}
	return nil
}

// buildRegistrationRecord creates the initial registration record
func (r *registrationService) buildRegistrationRecord(registrationID string, req *types.RegistrationRequest) *types.Registration {
	return &types.Registration{
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
}

// setupNamespace creates the namespace with proper metadata
func (r *registrationService) setupNamespace(ctx context.Context, req *types.RegistrationRequest, registrationID string) error {
	r.logger.WithField("namespace", req.Namespace).Info("Creating namespace")

	repoHash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.Repository.URL)))[:8]
	repoDomain := extractRepositoryDomain(req.Repository.URL)

	namespaceLabels := map[string]string{
		"gitops.io/registration-id":    registrationID[:8],
		"gitops.io/repository-hash":    repoHash,
		"gitops.io/repository-domain":  repoDomain,
		"gitops.io/managed-by":         "gitops-registration-service",
		"app.kubernetes.io/managed-by": "gitops-registration-service",
	}

	namespaceAnnotations := map[string]string{
		"gitops.io/repository-url":    req.Repository.URL,
		"gitops.io/repository-branch": req.Repository.Branch,
		"gitops.io/registration-id":   registrationID,
	}

	return r.k8s.CreateNamespaceWithMetadata(ctx, req.Namespace, namespaceLabels, namespaceAnnotations)
}

// setupServiceAccount creates service account and role binding with or without impersonation
func (r *registrationService) setupServiceAccount(ctx context.Context, namespace string) (string, error) {
	if r.cfg.Security.Impersonation.Enabled {
		return r.setupServiceAccountWithImpersonation(ctx, namespace)
	}
	return r.setupLegacyServiceAccount(ctx, namespace)
}

// setupServiceAccountWithImpersonation creates service account with impersonation support
func (r *registrationService) setupServiceAccountWithImpersonation(ctx context.Context, namespace string) (string, error) {
	r.logger.WithField("namespace", namespace).Info("Creating service account with impersonation")

	baseName := r.cfg.Security.Impersonation.ServiceAccountBaseName
	generatedName, err := r.k8s.CreateServiceAccountWithGenerateName(ctx, namespace, baseName)
	if err != nil {
		return "", fmt.Errorf("failed to create service account: %w", err)
	}

	roleBindingName := fmt.Sprintf("%s-binding", generatedName)
	clusterRole := r.cfg.Security.Impersonation.ClusterRole
	if err := r.k8s.CreateRoleBindingForServiceAccount(ctx, namespace, roleBindingName, clusterRole, generatedName); err != nil {
		return "", fmt.Errorf("failed to create role binding: %w", err)
	}

	return generatedName, nil
}

// setupLegacyServiceAccount creates service account with legacy behavior
func (r *registrationService) setupLegacyServiceAccount(ctx context.Context, namespace string) (string, error) {
	serviceAccountName := "gitops"
	if err := r.k8s.CreateServiceAccount(ctx, namespace, serviceAccountName); err != nil {
		return "", fmt.Errorf("failed to create service account: %w", err)
	}

	roleBindingName := "gitops-binding"
	if err := r.k8s.CreateRoleBinding(ctx, namespace, roleBindingName, "gitops-role", serviceAccountName); err != nil {
		return "", fmt.Errorf("failed to create role binding: %w", err)
	}

	return serviceAccountName, nil
}

// setupArgoCDResources creates ArgoCD AppProject and Application
func (r *registrationService) setupArgoCDResources(ctx context.Context, req *types.RegistrationRequest, serviceAccountName string) (appName, projectName string, err error) {
	projectName = req.Namespace
	appProject := r.buildAppProject(projectName, req.Namespace, req.Repository.URL, serviceAccountName)

	if err := r.argocd.CreateAppProject(ctx, appProject); err != nil {
		return "", "", fmt.Errorf("failed to create ArgoCD AppProject: %w", err)
	}

	appName = fmt.Sprintf("%s-app", req.Namespace)
	application := &types.Application{
		Name:    appName,
		Project: projectName,
		Source: types.ApplicationSource{
			RepoURL:        req.Repository.URL,
			TargetRevision: req.Repository.Branch,
			Path:           "manifests",
		},
		Destination: types.ApplicationDestination{
			Server:    "https://kubernetes.default.svc",
			Namespace: req.Namespace,
		},
	}

	if err := r.argocd.CreateApplication(ctx, application); err != nil {
		return "", "", fmt.Errorf("failed to create ArgoCD Application: %w", err)
	}

	return appName, projectName, nil
}

// finalizeRegistration updates the registration record with success status
func (r *registrationService) finalizeRegistration(registration *types.Registration, appName, projectName, serviceAccountName string) {
	registration.Status.Phase = "active"
	registration.Status.Message = "Registration completed successfully"
	registration.Status.ArgoCDApplication = appName
	registration.Status.ArgoCDAppProject = projectName
	registration.Status.LastSyncTime = time.Now()
	registration.Status.NamespaceCreated = true
	registration.Status.AppProjectCreated = true
	registration.Status.ApplicationCreated = true
	registration.UpdatedAt = time.Now()
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

func (r *registrationService) ListRegistrations(
	ctx context.Context, filters map[string]string,
) ([]*types.Registration, error) {
	// For now, return a simple stub - in a real implementation this would query a database
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

	// Step 1: Validate namespace exists
	if err := r.validateExistingNamespace(ctx, req.ExistingNamespace); err != nil {
		return nil, err
	}

	// Step 2: Create registration record
	registration := r.buildExistingNamespaceRegistration(registrationID, req)

	// Step 3: Setup service account in existing namespace
	if err := r.setupServiceAccountInExistingNamespace(ctx, req.ExistingNamespace); err != nil {
		registration.Status.Phase = StatusFailed
		registration.Status.Message = fmt.Sprintf("Failed to setup service account: %v", err)
		return nil, fmt.Errorf("failed to setup service account: %w", err)
	}

	// Step 4: Update namespace metadata
	r.updateExistingNamespaceMetadata(ctx, req, registrationID)

	// Step 5: Setup ArgoCD resources
	appName, projectName, err := r.setupArgoCDResourcesForExistingNamespace(ctx, req)
	if err != nil {
		registration.Status.Phase = StatusFailed
		registration.Status.Message = fmt.Sprintf("Failed to setup ArgoCD resources: %v", err)
		if deleteErr := r.k8s.DeleteNamespace(ctx, req.ExistingNamespace); deleteErr != nil {
			r.logger.WithError(deleteErr).Error("Failed to cleanup namespace")
		}
		return nil, fmt.Errorf("failed to setup ArgoCD resources: %w", err)
	}

	// Step 6: Finalize registration for existing namespace
	r.finalizeExistingNamespaceRegistration(registration, appName, projectName, userInfo)

	r.logger.WithFields(logrus.Fields{
		"namespace":         req.ExistingNamespace,
		"registrationID":    registrationID,
		"argoCDApplication": appName,
		"argoCDAppProject":  projectName,
		"user":              userInfo.Username,
	}).Info("Successfully converted existing namespace to GitOps management")

	return registration, nil
}

// validateExistingNamespace checks if the namespace exists
func (r *registrationService) validateExistingNamespace(ctx context.Context, namespace string) error {
	exists, err := r.k8s.NamespaceExists(ctx, namespace)
	if err != nil {
		return fmt.Errorf("failed to check namespace existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("namespace %s does not exist", namespace)
	}
	return nil
}

// buildExistingNamespaceRegistration creates the registration record for existing namespace
func (r *registrationService) buildExistingNamespaceRegistration(registrationID string, req *types.ExistingNamespaceRequest) *types.Registration {
	return &types.Registration{
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
}

// setupServiceAccountInExistingNamespace creates service account and role binding
func (r *registrationService) setupServiceAccountInExistingNamespace(ctx context.Context, namespace string) error {
	r.logger.WithField("namespace", namespace).Info("Creating service account in existing namespace")

	serviceAccountName := "gitops"
	if err := r.k8s.CreateServiceAccount(ctx, namespace, serviceAccountName); err != nil {
		return fmt.Errorf("failed to create service account: %w", err)
	}

	roleBindingName := "gitops-binding"
	if err := r.k8s.CreateRoleBinding(ctx, namespace, roleBindingName, "gitops-role", serviceAccountName); err != nil {
		return fmt.Errorf("failed to create role binding: %w", err)
	}

	return nil
}

// updateExistingNamespaceMetadata adds GitOps metadata to the existing namespace
func (r *registrationService) updateExistingNamespaceMetadata(ctx context.Context, req *types.ExistingNamespaceRequest, registrationID string) {
	r.logger.WithField("namespace", req.ExistingNamespace).Info("Adding GitOps metadata to existing namespace")

	repoHash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.Repository.URL)))[:8]
	repoDomain := extractRepositoryDomain(req.Repository.URL)

	namespaceLabels := map[string]string{
		"gitops.io/registration-id":    registrationID[:8],
		"gitops.io/repository-hash":    repoHash,
		"gitops.io/repository-domain":  repoDomain,
		"gitops.io/managed-by":         "gitops-registration-service",
		"app.kubernetes.io/managed-by": "gitops-registration-service",
	}

	namespaceAnnotations := map[string]string{
		"gitops.io/repository-url":    req.Repository.URL,
		"gitops.io/repository-branch": req.Repository.Branch,
		"gitops.io/registration-id":   registrationID,
	}

	err := r.k8s.UpdateNamespaceMetadata(ctx, req.ExistingNamespace, namespaceLabels, namespaceAnnotations)
	if err != nil {
		r.logger.WithError(err).WithField("namespace", req.ExistingNamespace).Warn("Failed to update namespace metadata, continuing...")
	}
}

// setupArgoCDResourcesForExistingNamespace creates ArgoCD AppProject and Application for existing namespace
func (r *registrationService) setupArgoCDResourcesForExistingNamespace(ctx context.Context, req *types.ExistingNamespaceRequest) (appName, projectName string, err error) {
	projectName = req.ExistingNamespace
	appProject := r.buildAppProject(projectName, req.ExistingNamespace, req.Repository.URL, "gitops")

	if err := r.argocd.CreateAppProject(ctx, appProject); err != nil {
		return "", "", fmt.Errorf("failed to create ArgoCD AppProject: %w", err)
	}

	appName = fmt.Sprintf("%s-app", req.ExistingNamespace)
	application := &types.Application{
		Name:    appName,
		Project: projectName,
		Source: types.ApplicationSource{
			RepoURL:        req.Repository.URL,
			TargetRevision: req.Repository.Branch,
			Path:           "manifests",
		},
		Destination: types.ApplicationDestination{
			Server:    "https://kubernetes.default.svc",
			Namespace: req.ExistingNamespace,
		},
	}

	if err := r.argocd.CreateApplication(ctx, application); err != nil {
		return "", "", fmt.Errorf("failed to create ArgoCD Application: %w", err)
	}

	return appName, projectName, nil
}

// finalizeExistingNamespaceRegistration updates the registration record with success status
func (r *registrationService) finalizeExistingNamespaceRegistration(registration *types.Registration, appName, projectName string, userInfo *types.UserInfo) {
	registration.Status.Phase = "active"
	registration.Status.Message = "Existing namespace successfully converted to GitOps management"
	registration.Status.ArgoCDApplication = appName
	registration.Status.ArgoCDAppProject = projectName
	registration.Status.LastSyncTime = time.Now()
	registration.Status.NamespaceCreated = false // Existing namespace, not created by us
	registration.Status.AppProjectCreated = true
	registration.Status.ApplicationCreated = true
	registration.UpdatedAt = time.Now()
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

func (r *registrationService) ValidateExistingNamespaceRequest(
	ctx context.Context, req *types.ExistingNamespaceRequest,
) error {
	// Basic validation
	if req.ExistingNamespace == "" {
		return fmt.Errorf("existingNamespace is required")
	}
	if req.Repository.URL == "" {
		return fmt.Errorf("repository URL is required")
	}

	return nil
}

func (r *registrationService) buildAppProject(
	projectName, namespace, repoURL, serviceAccountName string,
) *types.AppProject {
	// Generate repository hash for labeling
	repoHash := GenerateRepositoryHash(repoURL)

	appProject := &types.AppProject{
		Name:      projectName,
		Namespace: r.cfg.ArgoCD.Namespace, // AppProjects live in ArgoCD namespace
		Labels: map[string]string{
			RepositoryHashLabel:            repoHash,
			"gitops.io/managed-by":         "gitops-registration-service",
			"app.kubernetes.io/managed-by": "gitops-registration-service",
		},
		Destinations: []types.AppProjectDestination{
			{
				Server:    "https://kubernetes.default.svc",
				Namespace: namespace,
			},
		},
		SourceRepos: []string{repoURL},
	}

	// Add impersonation support if enabled
	if r.cfg.Security.Impersonation.Enabled {
		appProject.DestinationServiceAccounts = []types.AppProjectDestinationServiceAccount{
			{
				Server:                "https://kubernetes.default.svc",
				Namespace:             namespace,
				DefaultServiceAccount: serviceAccountName,
			},
		}
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
	}
	// If no restrictions provided, allow all resources by not setting any whitelist
	// This is the default behavior - no restrictions

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
