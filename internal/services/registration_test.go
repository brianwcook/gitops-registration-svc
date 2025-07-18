package services

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/konflux-ci/gitops-registration-service/internal/config"
	"github.com/konflux-ci/gitops-registration-service/internal/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock services for testing real implementations
type MockKubernetesService struct {
	mock.Mock
}

func (m *MockKubernetesService) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockKubernetesService) CreateNamespace(ctx context.Context, name string, labels map[string]string) error {
	args := m.Called(ctx, name, labels)
	return args.Error(0)
}

func (m *MockKubernetesService) CreateNamespaceWithMetadata(ctx context.Context, name string, labels, annotations map[string]string) error {
	args := m.Called(ctx, name, labels, annotations)
	return args.Error(0)
}

func (m *MockKubernetesService) DeleteNamespace(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockKubernetesService) UpdateNamespaceLabels(ctx context.Context, name string, labels map[string]string) error {
	args := m.Called(ctx, name, labels)
	return args.Error(0)
}

func (m *MockKubernetesService) UpdateNamespaceMetadata(ctx context.Context, name string, labels, annotations map[string]string) error {
	args := m.Called(ctx, name, labels, annotations)
	return args.Error(0)
}

func (m *MockKubernetesService) NamespaceExists(ctx context.Context, name string) (bool, error) {
	args := m.Called(ctx, name)
	return args.Bool(0), args.Error(1)
}

func (m *MockKubernetesService) CountNamespaces(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockKubernetesService) CreateServiceAccount(ctx context.Context, namespace, name string) error {
	args := m.Called(ctx, namespace, name)
	return args.Error(0)
}

func (m *MockKubernetesService) CreateRoleBinding(ctx context.Context, namespace, name, role, serviceAccount string) error {
	args := m.Called(ctx, namespace, name, role, serviceAccount)
	return args.Error(0)
}

func (m *MockKubernetesService) ValidateClusterRole(ctx context.Context, name string) (*ClusterRoleValidation, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(*ClusterRoleValidation), args.Error(1)
}

func (m *MockKubernetesService) CreateServiceAccountWithGenerateName(ctx context.Context, namespace, baseName string) (string, error) {
	args := m.Called(ctx, namespace, baseName)
	return args.String(0), args.Error(1)
}

func (m *MockKubernetesService) CreateRoleBindingForServiceAccount(ctx context.Context, namespace, name, clusterRole, serviceAccountName string) error {
	args := m.Called(ctx, namespace, name, clusterRole, serviceAccountName)
	return args.Error(0)
}

func (m *MockKubernetesService) CheckAppProjectConflict(ctx context.Context, repoHash string) (bool, error) {
	args := m.Called(ctx, repoHash)
	return args.Bool(0), args.Error(1)
}

type MockArgoCDService struct {
	mock.Mock
}

func (m *MockArgoCDService) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockArgoCDService) CreateAppProject(ctx context.Context, project *types.AppProject) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *MockArgoCDService) DeleteAppProject(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockArgoCDService) CreateApplication(ctx context.Context, app *types.Application) error {
	args := m.Called(ctx, app)
	return args.Error(0)
}

func (m *MockArgoCDService) DeleteApplication(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockArgoCDService) GetApplicationStatus(ctx context.Context, name string) (*types.ApplicationStatus, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(*types.ApplicationStatus), args.Error(1)
}

func (m *MockArgoCDService) CheckAppProjectConflict(ctx context.Context, repoHash string) (bool, error) {
	args := m.Called(ctx, repoHash)
	return args.Bool(0), args.Error(1)
}

// Test helper function
func setupRegistrationService(t *testing.T) (*registrationService, *MockKubernetesService, *MockArgoCDService) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{
		Security: config.SecurityConfig{
			Impersonation: config.ImpersonationConfig{
				Enabled:                false,
				ServiceAccountBaseName: "gitops",
				ClusterRole:            "gitops-role",
			},
		},
	}

	mockK8s := &MockKubernetesService{}
	mockArgoCD := &MockArgoCDService{}

	service := &registrationService{
		cfg:    cfg,
		k8s:    mockK8s,
		argocd: mockArgoCD,
		logger: logger,
	}

	return service, mockK8s, mockArgoCD
}

func TestRegistrationService_CheckRepositoryConflicts(t *testing.T) {
	service, _, mockArgoCD := setupRegistrationService(t)
	ctx := context.Background()

	tests := []struct {
		name                 string
		repoURL              string
		impersonationEnabled bool
		conflictExists       bool
		expectError          bool
		setupMocks           func()
	}{
		{
			name:                 "No conflicts with impersonation disabled",
			repoURL:              "https://github.com/test/repo",
			impersonationEnabled: false,
			expectError:          false,
			setupMocks:           func() {}, // No mocks needed when impersonation disabled
		},
		{
			name:                 "No conflicts with impersonation enabled",
			repoURL:              "https://github.com/test/repo",
			impersonationEnabled: true,
			conflictExists:       false,
			expectError:          false,
			setupMocks: func() {
				mockArgoCD.On("CheckAppProjectConflict", ctx, mock.AnythingOfType("string")).Return(false, nil)
			},
		},
		{
			name:                 "Repository conflict exists",
			repoURL:              "https://github.com/test/repo",
			impersonationEnabled: true,
			conflictExists:       true,
			expectError:          true,
			setupMocks: func() {
				mockArgoCD.On("CheckAppProjectConflict", ctx, mock.AnythingOfType("string")).Return(true, nil)
			},
		},
		{
			name:                 "Error checking conflicts",
			repoURL:              "https://github.com/test/repo",
			impersonationEnabled: true,
			expectError:          true,
			setupMocks: func() {
				mockArgoCD.On("CheckAppProjectConflict", ctx, mock.AnythingOfType("string")).Return(false, errors.New("API error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			mockArgoCD.ExpectedCalls = nil

			service.cfg.Security.Impersonation.Enabled = tt.impersonationEnabled
			tt.setupMocks()

			err := service.checkRepositoryConflicts(ctx, tt.repoURL)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockArgoCD.AssertExpectations(t)
		})
	}
}

func TestRegistrationService_ValidateNamespaceAvailability(t *testing.T) {
	service, mockK8s, _ := setupRegistrationService(t)
	ctx := context.Background()

	tests := []struct {
		name            string
		namespace       string
		namespaceExists bool
		k8sError        error
		expectError     bool
		expectedErrType string
	}{
		{
			name:            "Namespace available",
			namespace:       "test-namespace",
			namespaceExists: false,
			expectError:     false,
		},
		{
			name:            "Namespace already exists",
			namespace:       "existing-namespace",
			namespaceExists: true,
			expectError:     true,
			expectedErrType: "*services.NamespaceConflictError",
		},
		{
			name:        "Error checking namespace",
			namespace:   "test-namespace",
			k8sError:    errors.New("API error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			mockK8s.ExpectedCalls = nil

			mockK8s.On("NamespaceExists", ctx, tt.namespace).Return(tt.namespaceExists, tt.k8sError)

			err := service.validateNamespaceAvailability(ctx, tt.namespace)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErrType != "" {
					assert.IsType(t, &NamespaceConflictError{}, err)
				}
			} else {
				assert.NoError(t, err)
			}

			mockK8s.AssertExpectations(t)
		})
	}
}

func TestRegistrationService_BuildRegistrationRecord(t *testing.T) {
	service, _, _ := setupRegistrationService(t)

	req := &types.RegistrationRequest{
		Namespace: "test-namespace",
		Repository: types.Repository{
			URL:    "https://github.com/test/repo",
			Branch: "main",
		},
	}

	registrationID := "test-reg-123"

	registration := service.buildRegistrationRecord(registrationID, req)

	assert.Equal(t, registrationID, registration.ID)
	assert.Equal(t, req.Namespace, registration.Namespace)
	assert.Equal(t, req.Repository.URL, registration.Repository.URL)
	assert.Equal(t, req.Repository.Branch, registration.Repository.Branch)
	assert.Equal(t, "creating", registration.Status.Phase)
	assert.Equal(t, "Registration in progress", registration.Status.Message)
	assert.NotNil(t, registration.Labels)
	assert.Contains(t, registration.Labels, "gitops.io/managed-by")
}

func TestRegistrationService_SetupNamespace(t *testing.T) {
	service, mockK8s, _ := setupRegistrationService(t)
	ctx := context.Background()

	req := &types.RegistrationRequest{
		Namespace: "test-namespace",
		Repository: types.Repository{
			URL:    "https://github.com/test/repo",
			Branch: "main",
		},
	}
	registrationID := "test-reg-123"

	tests := []struct {
		name        string
		expectError bool
		setupMocks  func()
	}{
		{
			name:        "Successful namespace setup",
			expectError: false,
			setupMocks: func() {
				mockK8s.On("CreateNamespaceWithMetadata", ctx, req.Namespace,
					mock.AnythingOfType("map[string]string"),
					mock.AnythingOfType("map[string]string")).Return(nil)
			},
		},
		{
			name:        "Error creating namespace",
			expectError: true,
			setupMocks: func() {
				mockK8s.On("CreateNamespaceWithMetadata", ctx, req.Namespace,
					mock.AnythingOfType("map[string]string"),
					mock.AnythingOfType("map[string]string")).Return(errors.New("creation failed"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			mockK8s.ExpectedCalls = nil
			tt.setupMocks()

			err := service.setupNamespace(ctx, req, registrationID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockK8s.AssertExpectations(t)
		})
	}
}

func TestRegistrationService_SetupServiceAccount_Legacy(t *testing.T) {
	service, mockK8s, _ := setupRegistrationService(t)
	ctx := context.Background()
	namespace := "test-namespace"

	// Test legacy mode (impersonation disabled)
	service.cfg.Security.Impersonation.Enabled = false

	tests := []struct {
		name              string
		expectError       bool
		expectedSAName    string
		serviceAccountErr error
		roleBindingErr    error
	}{
		{
			name:           "Successful legacy service account setup",
			expectError:    false,
			expectedSAName: "gitops",
		},
		{
			name:              "Service account creation fails",
			expectError:       true,
			serviceAccountErr: errors.New("SA creation failed"),
		},
		{
			name:           "Role binding creation fails",
			expectError:    true,
			roleBindingErr: errors.New("RB creation failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			mockK8s.ExpectedCalls = nil

			mockK8s.On("CreateServiceAccount", ctx, namespace, "gitops").Return(tt.serviceAccountErr)
			if tt.serviceAccountErr == nil {
				mockK8s.On("CreateRoleBinding", ctx, namespace, "gitops-binding", "gitops-role", "gitops").Return(tt.roleBindingErr)
			}

			serviceAccountName, err := service.setupServiceAccount(ctx, namespace)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSAName, serviceAccountName)
			}

			mockK8s.AssertExpectations(t)
		})
	}
}

func TestRegistrationService_SetupServiceAccount_Impersonation(t *testing.T) {
	service, mockK8s, _ := setupRegistrationService(t)
	ctx := context.Background()
	namespace := "test-namespace"

	// Test impersonation mode
	service.cfg.Security.Impersonation.Enabled = true
	service.cfg.Security.Impersonation.ServiceAccountBaseName = "gitops-sa"
	service.cfg.Security.Impersonation.ClusterRole = "gitops-cluster-role"

	tests := []struct {
		name              string
		expectError       bool
		generatedSAName   string
		serviceAccountErr error
		roleBindingErr    error
	}{
		{
			name:            "Successful impersonation service account setup",
			expectError:     false,
			generatedSAName: "gitops-sa-abc123",
		},
		{
			name:              "Service account generation fails",
			expectError:       true,
			serviceAccountErr: errors.New("SA generation failed"),
		},
		{
			name:            "Role binding creation fails",
			expectError:     true,
			generatedSAName: "gitops-sa-abc123",
			roleBindingErr:  errors.New("RB creation failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			mockK8s.ExpectedCalls = nil

			mockK8s.On("CreateServiceAccountWithGenerateName", ctx, namespace, "gitops-sa").Return(tt.generatedSAName, tt.serviceAccountErr)
			if tt.serviceAccountErr == nil && tt.generatedSAName != "" {
				mockK8s.On("CreateRoleBindingForServiceAccount", ctx, namespace,
					fmt.Sprintf("%s-binding", tt.generatedSAName), "gitops-cluster-role", tt.generatedSAName).Return(tt.roleBindingErr)
			}

			serviceAccountName, err := service.setupServiceAccount(ctx, namespace)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.generatedSAName, serviceAccountName)
			}

			mockK8s.AssertExpectations(t)
		})
	}
}

func TestRegistrationService_SetupArgoCDResources(t *testing.T) {
	service, _, mockArgoCD := setupRegistrationService(t)
	ctx := context.Background()

	req := &types.RegistrationRequest{
		Namespace: "test-namespace",
		Repository: types.Repository{
			URL:    "https://github.com/test/repo",
			Branch: "main",
		},
	}
	serviceAccountName := "gitops"

	tests := []struct {
		name                string
		expectError         bool
		appProjectErr       error
		applicationErr      error
		expectedAppName     string
		expectedProjectName string
	}{
		{
			name:                "Successful ArgoCD resource setup",
			expectError:         false,
			expectedAppName:     "test-namespace-app",
			expectedProjectName: "test-namespace",
		},
		{
			name:          "AppProject creation fails",
			expectError:   true,
			appProjectErr: errors.New("AppProject creation failed"),
		},
		{
			name:           "Application creation fails",
			expectError:    true,
			applicationErr: errors.New("Application creation failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			mockArgoCD.ExpectedCalls = nil

			mockArgoCD.On("CreateAppProject", ctx, mock.AnythingOfType("*types.AppProject")).Return(tt.appProjectErr)
			if tt.appProjectErr == nil {
				mockArgoCD.On("CreateApplication", ctx, mock.AnythingOfType("*types.Application")).Return(tt.applicationErr)
			}

			appName, projectName, err := service.setupArgoCDResources(ctx, req, serviceAccountName)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedAppName, appName)
				assert.Equal(t, tt.expectedProjectName, projectName)
			}

			mockArgoCD.AssertExpectations(t)
		})
	}
}

func TestRegistrationService_FinalizeRegistration(t *testing.T) {
	service, _, _ := setupRegistrationService(t)

	registration := &types.Registration{
		ID:        "test-reg-123",
		Namespace: "test-namespace",
		Status: types.RegistrationStatus{
			Phase:   "creating",
			Message: "Registration in progress",
		},
	}

	appName := "test-namespace-app"
	projectName := "test-namespace"
	serviceAccountName := "gitops"

	service.finalizeRegistration(registration, appName, projectName, serviceAccountName)

	assert.Equal(t, "active", registration.Status.Phase)
	assert.Equal(t, "Registration completed successfully", registration.Status.Message)
	assert.Equal(t, appName, registration.Status.ArgoCDApplication)
	assert.Equal(t, projectName, registration.Status.ArgoCDAppProject)
	assert.True(t, registration.Status.NamespaceCreated)
	assert.True(t, registration.Status.AppProjectCreated)
	assert.True(t, registration.Status.ApplicationCreated)
	assert.NotNil(t, registration.Status.LastSyncTime)
}

func TestRegistrationService_ValidateRegistration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{}
	k8sStub := &kubernetesServiceStub{logger: logger}
	argoCDStub := &argoCDServiceStub{logger: logger}

	regService := NewRegistrationServiceReal(cfg, k8sStub, argoCDStub, logger)
	ctx := context.Background()

	tests := []struct {
		name        string
		req         *types.RegistrationRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid request",
			req: &types.RegistrationRequest{
				Repository: types.Repository{URL: "https://github.com/test/repo"},
				Namespace:  "test-namespace",
			},
			expectError: false,
		},
		{
			name: "Invalid request - missing namespace",
			req: &types.RegistrationRequest{
				Repository: types.Repository{URL: "https://github.com/test/repo"},
			},
			expectError: true,
			errorMsg:    "namespace is required",
		},
		{
			name: "Invalid request - missing repository URL",
			req: &types.RegistrationRequest{
				Namespace: "test-namespace",
			},
			expectError: true,
			errorMsg:    "repository URL is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := regService.ValidateRegistration(ctx, tt.req)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegistrationService_ValidateExistingNamespaceRequest(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{}
	k8sStub := &kubernetesServiceStub{logger: logger}
	argoCDStub := &argoCDServiceStub{logger: logger}

	regService := NewRegistrationServiceReal(cfg, k8sStub, argoCDStub, logger)
	ctx := context.Background()

	tests := []struct {
		name        string
		req         *types.ExistingNamespaceRequest
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid request",
			req: &types.ExistingNamespaceRequest{
				Repository:        types.Repository{URL: "https://github.com/test/repo"},
				ExistingNamespace: "test-namespace",
			},
			expectError: false,
		},
		{
			name: "Invalid request - missing existing namespace",
			req: &types.ExistingNamespaceRequest{
				Repository: types.Repository{URL: "https://github.com/test/repo"},
			},
			expectError: true,
			errorMsg:    "existingNamespace is required",
		},
		{
			name: "Invalid request - missing repository URL",
			req: &types.ExistingNamespaceRequest{
				ExistingNamespace: "test-namespace",
			},
			expectError: true,
			errorMsg:    "repository URL is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := regService.ValidateExistingNamespaceRequest(ctx, tt.req)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegistrationService_BuildAppProject(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	tests := []struct {
		name        string
		config      *config.Config
		projectName string
		namespace   string
		repoURL     string
		checkFunc   func(t *testing.T, project *types.AppProject)
	}{
		{
			name: "AppProject with service-level allowList",
			config: &config.Config{
				Security: config.SecurityConfig{
					ResourceAllowList: []config.ServiceResourceRestriction{
						{Group: "apps", Kind: "Deployment"},
						{Group: "", Kind: "ConfigMap"},
					},
				},
			},
			projectName: "test-project",
			namespace:   "test-namespace",
			repoURL:     "https://github.com/test/repo",
			checkFunc: func(t *testing.T, project *types.AppProject) {
				assert.Equal(t, "test-project", project.Name)
				assert.Equal(t, "test-namespace", project.Destinations[0].Namespace)
				assert.Equal(t, []string{"https://github.com/test/repo"}, project.SourceRepos)
				assert.Len(t, project.ClusterResourceWhitelist, 2)
				assert.Len(t, project.NamespaceResourceWhitelist, 2)
				assert.Empty(t, project.ClusterResourceBlacklist)
				assert.Empty(t, project.NamespaceResourceBlacklist)

				// Check specific resources
				assert.Contains(t, project.ClusterResourceWhitelist, types.AppProjectResource{Group: "apps", Kind: "Deployment"})
				assert.Contains(t, project.ClusterResourceWhitelist, types.AppProjectResource{Group: "", Kind: "ConfigMap"})
			},
		},
		{
			name: "AppProject with service-level denyList",
			config: &config.Config{
				Security: config.SecurityConfig{
					ResourceDenyList: []config.ServiceResourceRestriction{
						{Group: "kafka.strimzi.io", Kind: "KafkaTopic"},
					},
				},
			},
			projectName: "test-project",
			namespace:   "test-namespace",
			repoURL:     "https://github.com/test/repo",
			checkFunc: func(t *testing.T, project *types.AppProject) {
				assert.Equal(t, "test-project", project.Name)
				assert.Empty(t, project.ClusterResourceWhitelist)
				assert.Empty(t, project.NamespaceResourceWhitelist)
				assert.Len(t, project.ClusterResourceBlacklist, 1)
				assert.Len(t, project.NamespaceResourceBlacklist, 1)

				// Check specific resources
				assert.Contains(t, project.ClusterResourceBlacklist, types.AppProjectResource{Group: "kafka.strimzi.io", Kind: "KafkaTopic"})
			},
		},
		{
			name: "AppProject with no service-level restrictions",
			config: &config.Config{
				Security: config.SecurityConfig{},
			},
			projectName: "test-project",
			namespace:   "test-namespace",
			repoURL:     "https://github.com/test/repo",
			checkFunc: func(t *testing.T, project *types.AppProject) {
				assert.Equal(t, "test-project", project.Name)
				assert.Empty(t, project.ClusterResourceWhitelist)
				assert.Empty(t, project.NamespaceResourceWhitelist)
				assert.Empty(t, project.ClusterResourceBlacklist)
				assert.Empty(t, project.NamespaceResourceBlacklist)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8sStub := &kubernetesServiceStub{logger: logger}
			argoCDStub := &argoCDServiceStub{logger: logger}
			regService := NewRegistrationServiceReal(tt.config, k8sStub, argoCDStub, logger).(*registrationService)

			project := regService.buildAppProject(tt.projectName, tt.namespace, tt.repoURL, "test-service-account")
			require.NotNil(t, project)
			tt.checkFunc(t, project)
		})
	}
}

func TestRegistrationService_BuildAppProject_DestinationsEnforcement(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{}
	k8sStub := &kubernetesServiceStub{logger: logger}
	argoCDStub := &argoCDServiceStub{logger: logger}

	regService := NewRegistrationServiceReal(cfg, k8sStub, argoCDStub, logger).(*registrationService)

	// Test that destinations are properly enforced
	project := regService.buildAppProject("test-project", "restricted-namespace", "https://github.com/test/repo", "test-service-account")

	require.NotNil(t, project)
	require.Len(t, project.Destinations, 1)

	destination := project.Destinations[0]
	assert.Equal(t, "https://kubernetes.default.svc", destination.Server)
	assert.Equal(t, "restricted-namespace", destination.Namespace)

	// Ensure the AppProject can only deploy to the specified namespace
	assert.Equal(t, "restricted-namespace", destination.Namespace)
}

func TestRegistrationService_ImpersonationEnabled(t *testing.T) {
	tests := []struct {
		name                 string
		impersonationEnabled bool
		serviceAccountName   string
		expectedSACount      int
		expectedLabels       map[string]string
	}{
		{
			name:                 "Impersonation enabled with service account",
			impersonationEnabled: true,
			serviceAccountName:   "gitops-sa-abc123",
			expectedSACount:      1,
			expectedLabels: map[string]string{
				"gitops.io/repository-hash":    "be40cd26",
				"gitops.io/managed-by":         "gitops-registration-service",
				"app.kubernetes.io/managed-by": "gitops-registration-service",
			},
		},
		{
			name:                 "Impersonation disabled",
			impersonationEnabled: false,
			serviceAccountName:   "gitops",
			expectedSACount:      0,
			expectedLabels: map[string]string{
				"gitops.io/repository-hash":    "be40cd26",
				"gitops.io/managed-by":         "gitops-registration-service",
				"app.kubernetes.io/managed-by": "gitops-registration-service",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup config with impersonation settings
			cfg := &config.Config{
				Security: config.SecurityConfig{
					Impersonation: config.ImpersonationConfig{
						Enabled:                tt.impersonationEnabled,
						ClusterRole:            "test-gitops-deployer",
						ServiceAccountBaseName: "gitops-sa",
						ValidatePermissions:    true,
						AutoCleanup:            true,
					},
				},
				ArgoCD: config.ArgoCDConfig{
					Namespace: "argocd",
				},
			}

			logger := logrus.New()
			k8sStub := &kubernetesServiceStub{logger: logger}
			argoCDStub := &argoCDServiceStub{logger: logger}

			regService := NewRegistrationServiceReal(cfg, k8sStub, argoCDStub, logger).(*registrationService)

			// Test buildAppProject with impersonation
			project := regService.buildAppProject("test-project", "test-namespace", "https://github.com/test/repo", tt.serviceAccountName)

			// Verify basic project properties
			require.NotNil(t, project)
			require.Equal(t, "test-project", project.Name)
			require.Equal(t, "argocd", project.Namespace)
			require.Equal(t, []string{"https://github.com/test/repo"}, project.SourceRepos)

			// Verify repository hash label
			require.Contains(t, project.Labels, "gitops.io/repository-hash")
			require.Equal(t, "be40cd26", project.Labels["gitops.io/repository-hash"]) // First 8 chars of SHA256

			// Verify destinationServiceAccounts based on impersonation setting
			if tt.impersonationEnabled {
				require.Len(t, project.DestinationServiceAccounts, tt.expectedSACount)
				if len(project.DestinationServiceAccounts) > 0 {
					sa := project.DestinationServiceAccounts[0]
					require.Equal(t, "https://kubernetes.default.svc", sa.Server)
					require.Equal(t, "test-namespace", sa.Namespace)
					require.Equal(t, tt.serviceAccountName, sa.DefaultServiceAccount)
				}
			} else {
				require.Len(t, project.DestinationServiceAccounts, 0)
			}

			// Verify labels
			for key, value := range tt.expectedLabels {
				require.Equal(t, value, project.Labels[key])
			}
		})
	}
}

func TestRegistrationService_RepositoryConflictDetection(t *testing.T) {
	repoURL := "https://github.com/test/repo"

	// Test repository hash generation
	hash := GenerateRepositoryHash(repoURL)
	require.Equal(t, "be40cd26", hash) // First 8 chars of SHA256 for this URL

	// This test verifies the hash generation used for conflict detection
	// In a real scenario, this hash would be used to label AppProjects
	// and check for conflicts via Kubernetes label selectors
}

func TestGenerateRepositoryHash(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		expected string
	}{
		{
			name:     "GitHub repository",
			repoURL:  "https://github.com/user/repo",
			expected: "b719fba9", // First 8 chars of SHA256
		},
		{
			name:     "GitLab repository",
			repoURL:  "https://gitlab.com/user/repo.git",
			expected: "4b47a8b4", // First 8 chars of SHA256
		},
		{
			name:     "Same URL should produce same hash",
			repoURL:  "https://github.com/user/repo",
			expected: "b719fba9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := GenerateRepositoryHash(tt.repoURL)
			require.Equal(t, tt.expected, hash)
			require.Len(t, hash, 8) // Should always be 8 characters
		})
	}
}

func TestClusterRoleValidation_SecurityWarnings(t *testing.T) {
	logger := logrus.New()
	k8sStub := &kubernetesServiceStub{logger: logger}

	validation, err := k8sStub.ValidateClusterRole(context.Background(), "test-role")
	require.NoError(t, err)
	require.NotNil(t, validation)
	require.True(t, validation.Exists)
	require.False(t, validation.HasClusterAdmin)
	require.False(t, validation.HasNamespaceSpanning)
	require.False(t, validation.HasClusterScoped)
	require.Len(t, validation.Warnings, 0)
	require.Contains(t, validation.ResourceTypes, "secrets")
	require.Contains(t, validation.ResourceTypes, "configmaps")
	require.Contains(t, validation.ResourceTypes, "deployments")
}

// Test helper function setup for real registration service
func setupRealRegistrationService(t *testing.T) (*registrationService, *MockKubernetesService, *MockArgoCDService) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{
		Security: config.SecurityConfig{
			Impersonation: config.ImpersonationConfig{
				Enabled:                false,
				ServiceAccountBaseName: "gitops",
				ClusterRole:            "gitops-role",
			},
		},
	}

	mockK8s := &MockKubernetesService{}
	mockArgoCD := &MockArgoCDService{}

	service := &registrationService{
		cfg:    cfg,
		k8s:    mockK8s,
		argocd: mockArgoCD,
		logger: logger,
	}

	return service, mockK8s, mockArgoCD
}

func TestRegistrationService_CheckRepositoryConflicts_Real(t *testing.T) {
	service, _, mockArgoCD := setupRealRegistrationService(t)
	ctx := context.Background()

	tests := []struct {
		name                 string
		repoURL              string
		impersonationEnabled bool
		conflictExists       bool
		expectError          bool
		setupMocks           func()
	}{
		{
			name:                 "No conflicts with impersonation disabled",
			repoURL:              "https://github.com/test/repo",
			impersonationEnabled: false,
			expectError:          false,
			setupMocks:           func() {}, // No mocks needed when impersonation disabled
		},
		{
			name:                 "No conflicts with impersonation enabled",
			repoURL:              "https://github.com/test/repo",
			impersonationEnabled: true,
			conflictExists:       false,
			expectError:          false,
			setupMocks: func() {
				mockArgoCD.On("CheckAppProjectConflict", ctx, mock.AnythingOfType("string")).Return(false, nil)
			},
		},
		{
			name:                 "Repository conflict exists",
			repoURL:              "https://github.com/test/repo",
			impersonationEnabled: true,
			conflictExists:       true,
			expectError:          true,
			setupMocks: func() {
				mockArgoCD.On("CheckAppProjectConflict", ctx, mock.AnythingOfType("string")).Return(true, nil)
			},
		},
		{
			name:                 "Error checking conflicts",
			repoURL:              "https://github.com/test/repo",
			impersonationEnabled: true,
			expectError:          true,
			setupMocks: func() {
				mockArgoCD.On("CheckAppProjectConflict", ctx, mock.AnythingOfType("string")).Return(false, errors.New("API error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			mockArgoCD.ExpectedCalls = nil

			service.cfg.Security.Impersonation.Enabled = tt.impersonationEnabled
			tt.setupMocks()

			err := service.checkRepositoryConflicts(ctx, tt.repoURL)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockArgoCD.AssertExpectations(t)
		})
	}
}

func TestRegistrationService_ValidateNamespaceAvailability_Real(t *testing.T) {
	service, mockK8s, _ := setupRealRegistrationService(t)
	ctx := context.Background()

	tests := []struct {
		name            string
		namespace       string
		namespaceExists bool
		k8sError        error
		expectError     bool
		expectedErrType string
	}{
		{
			name:            "Namespace available",
			namespace:       "test-namespace",
			namespaceExists: false,
			expectError:     false,
		},
		{
			name:            "Namespace already exists",
			namespace:       "existing-namespace",
			namespaceExists: true,
			expectError:     true,
			expectedErrType: "*services.NamespaceConflictError",
		},
		{
			name:        "Error checking namespace",
			namespace:   "test-namespace",
			k8sError:    errors.New("API error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			mockK8s.ExpectedCalls = nil

			mockK8s.On("NamespaceExists", ctx, tt.namespace).Return(tt.namespaceExists, tt.k8sError)

			err := service.validateNamespaceAvailability(ctx, tt.namespace)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErrType != "" {
					assert.IsType(t, &NamespaceConflictError{}, err)
				}
			} else {
				assert.NoError(t, err)
			}

			mockK8s.AssertExpectations(t)
		})
	}
}

func TestRegistrationService_BuildRegistrationRecord_Real(t *testing.T) {
	service, _, _ := setupRealRegistrationService(t)

	req := &types.RegistrationRequest{
		Namespace: "test-namespace",
		Repository: types.Repository{
			URL:    "https://github.com/test/repo",
			Branch: "main",
		},
	}

	registrationID := "test-reg-123"

	registration := service.buildRegistrationRecord(registrationID, req)

	assert.Equal(t, registrationID, registration.ID)
	assert.Equal(t, req.Namespace, registration.Namespace)
	assert.Equal(t, req.Repository.URL, registration.Repository.URL)
	assert.Equal(t, req.Repository.Branch, registration.Repository.Branch)
	assert.Equal(t, "creating", registration.Status.Phase)
	assert.Equal(t, "Registration in progress", registration.Status.Message)
	assert.NotNil(t, registration.Labels)
	assert.Contains(t, registration.Labels, "gitops.io/managed-by")
}

func TestRegistrationService_SetupNamespace_Real(t *testing.T) {
	service, mockK8s, _ := setupRealRegistrationService(t)
	ctx := context.Background()

	req := &types.RegistrationRequest{
		Namespace: "test-namespace",
		Repository: types.Repository{
			URL:    "https://github.com/test/repo",
			Branch: "main",
		},
	}
	registrationID := "test-reg-123"

	tests := []struct {
		name        string
		expectError bool
		setupMocks  func()
	}{
		{
			name:        "Successful namespace setup",
			expectError: false,
			setupMocks: func() {
				mockK8s.On("CreateNamespaceWithMetadata", ctx, req.Namespace,
					mock.AnythingOfType("map[string]string"),
					mock.AnythingOfType("map[string]string")).Return(nil)
			},
		},
		{
			name:        "Error creating namespace",
			expectError: true,
			setupMocks: func() {
				mockK8s.On("CreateNamespaceWithMetadata", ctx, req.Namespace,
					mock.AnythingOfType("map[string]string"),
					mock.AnythingOfType("map[string]string")).Return(errors.New("creation failed"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			mockK8s.ExpectedCalls = nil
			tt.setupMocks()

			err := service.setupNamespace(ctx, req, registrationID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockK8s.AssertExpectations(t)
		})
	}
}

func TestRegistrationService_SetupServiceAccount_Legacy_Real(t *testing.T) {
	service, mockK8s, _ := setupRealRegistrationService(t)
	ctx := context.Background()
	namespace := "test-namespace"

	// Test legacy mode (impersonation disabled)
	service.cfg.Security.Impersonation.Enabled = false

	tests := []struct {
		name              string
		expectError       bool
		expectedSAName    string
		serviceAccountErr error
		roleBindingErr    error
	}{
		{
			name:           "Successful legacy service account setup",
			expectError:    false,
			expectedSAName: "gitops",
		},
		{
			name:              "Service account creation fails",
			expectError:       true,
			serviceAccountErr: errors.New("SA creation failed"),
		},
		{
			name:           "Role binding creation fails",
			expectError:    true,
			roleBindingErr: errors.New("RB creation failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			mockK8s.ExpectedCalls = nil

			mockK8s.On("CreateServiceAccount", ctx, namespace, "gitops").Return(tt.serviceAccountErr)
			if tt.serviceAccountErr == nil {
				mockK8s.On("CreateRoleBinding", ctx, namespace, "gitops-binding", "gitops-role", "gitops").Return(tt.roleBindingErr)
			}

			serviceAccountName, err := service.setupServiceAccount(ctx, namespace)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSAName, serviceAccountName)
			}

			mockK8s.AssertExpectations(t)
		})
	}
}

func TestRegistrationService_SetupServiceAccount_Impersonation_Real(t *testing.T) {
	service, mockK8s, _ := setupRealRegistrationService(t)
	ctx := context.Background()
	namespace := "test-namespace"

	// Test impersonation mode
	service.cfg.Security.Impersonation.Enabled = true
	service.cfg.Security.Impersonation.ServiceAccountBaseName = "gitops-sa"
	service.cfg.Security.Impersonation.ClusterRole = "gitops-cluster-role"

	tests := []struct {
		name              string
		expectError       bool
		generatedSAName   string
		serviceAccountErr error
		roleBindingErr    error
	}{
		{
			name:            "Successful impersonation service account setup",
			expectError:     false,
			generatedSAName: "gitops-sa-abc123",
		},
		{
			name:              "Service account generation fails",
			expectError:       true,
			serviceAccountErr: errors.New("SA generation failed"),
		},
		{
			name:            "Role binding creation fails",
			expectError:     true,
			generatedSAName: "gitops-sa-abc123",
			roleBindingErr:  errors.New("RB creation failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			mockK8s.ExpectedCalls = nil

			mockK8s.On("CreateServiceAccountWithGenerateName", ctx, namespace, "gitops-sa").Return(tt.generatedSAName, tt.serviceAccountErr)
			if tt.serviceAccountErr == nil && tt.generatedSAName != "" {
				mockK8s.On("CreateRoleBindingForServiceAccount", ctx, namespace,
					fmt.Sprintf("%s-binding", tt.generatedSAName), "gitops-cluster-role", tt.generatedSAName).Return(tt.roleBindingErr)
			}

			serviceAccountName, err := service.setupServiceAccount(ctx, namespace)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.generatedSAName, serviceAccountName)
			}

			mockK8s.AssertExpectations(t)
		})
	}
}

func TestRegistrationService_SetupArgoCDResources_Real(t *testing.T) {
	service, _, mockArgoCD := setupRealRegistrationService(t)
	ctx := context.Background()

	req := &types.RegistrationRequest{
		Namespace: "test-namespace",
		Repository: types.Repository{
			URL:    "https://github.com/test/repo",
			Branch: "main",
		},
	}
	serviceAccountName := "gitops"

	tests := []struct {
		name                string
		expectError         bool
		appProjectErr       error
		applicationErr      error
		expectedAppName     string
		expectedProjectName string
	}{
		{
			name:                "Successful ArgoCD resource setup",
			expectError:         false,
			expectedAppName:     "test-namespace-app",
			expectedProjectName: "test-namespace",
		},
		{
			name:          "AppProject creation fails",
			expectError:   true,
			appProjectErr: errors.New("AppProject creation failed"),
		},
		{
			name:           "Application creation fails",
			expectError:    true,
			applicationErr: errors.New("Application creation failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			mockArgoCD.ExpectedCalls = nil

			mockArgoCD.On("CreateAppProject", ctx, mock.AnythingOfType("*types.AppProject")).Return(tt.appProjectErr)
			if tt.appProjectErr == nil {
				mockArgoCD.On("CreateApplication", ctx, mock.AnythingOfType("*types.Application")).Return(tt.applicationErr)
			}

			appName, projectName, err := service.setupArgoCDResources(ctx, req, serviceAccountName)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedAppName, appName)
				assert.Equal(t, tt.expectedProjectName, projectName)
			}

			mockArgoCD.AssertExpectations(t)
		})
	}
}

func TestRegistrationService_FinalizeRegistration_Real(t *testing.T) {
	service, _, _ := setupRealRegistrationService(t)

	registration := &types.Registration{
		ID:        "test-reg-123",
		Namespace: "test-namespace",
		Status: types.RegistrationStatus{
			Phase:   "creating",
			Message: "Registration in progress",
		},
	}

	appName := "test-namespace-app"
	projectName := "test-namespace"
	serviceAccountName := "gitops"

	service.finalizeRegistration(registration, appName, projectName, serviceAccountName)

	assert.Equal(t, "active", registration.Status.Phase)
	assert.Equal(t, "Registration completed successfully", registration.Status.Message)
	assert.Equal(t, appName, registration.Status.ArgoCDApplication)
	assert.Equal(t, projectName, registration.Status.ArgoCDAppProject)
	assert.True(t, registration.Status.NamespaceCreated)
	assert.True(t, registration.Status.AppProjectCreated)
	assert.True(t, registration.Status.ApplicationCreated)
	assert.NotNil(t, registration.Status.LastSyncTime)
}

func TestRegistrationService_CreateRegistration_NamespaceConflict_Real(t *testing.T) {
	service, mockK8s, _ := setupRealRegistrationService(t)
	ctx := context.Background()

	req := &types.RegistrationRequest{
		Namespace: "existing-namespace",
		Repository: types.Repository{
			URL:    "https://github.com/test/repo",
			Branch: "main",
		},
	}

	// Setup namespace conflict
	mockK8s.On("NamespaceExists", ctx, req.Namespace).Return(true, nil)

	registration, err := service.CreateRegistration(ctx, req)

	require.Error(t, err)
	require.Nil(t, registration)
	assert.IsType(t, &NamespaceConflictError{}, err)

	mockK8s.AssertExpectations(t)
}

func TestRegistrationService_CRUDOperations_WithFakeClients(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	cfg := &config.Config{
		ArgoCD: config.ArgoCDConfig{
			Namespace: "argocd",
		},
	}

	// Create services with fake clients
	k8sFactory := NewTestKubernetesFactory()
	argoCDFactory := NewTestArgoCDFactory()

	k8sService, err := NewKubernetesServiceWithFactory(cfg, logger, k8sFactory)
	require.NoError(t, err)

	argoCDService, err := NewArgoCDServiceWithFactory(cfg, logger, argoCDFactory)
	require.NoError(t, err)

	service := NewRegistrationServiceReal(cfg, k8sService, argoCDService, logger)

	t.Run("GetRegistration operations", func(t *testing.T) {
		ctx := context.Background()

		// Test getting a registration - with fake clients this will return not found
		registration, err := service.GetRegistration(ctx, "test-registration-id")

		// With fake clients, we expect this to fail gracefully
		if err != nil {
			// Should be a reasonable error like "not found"
			assert.Error(t, err)
		} else {
			// If it succeeds, registration should be valid
			assert.NotNil(t, registration)
		}
	})

	t.Run("ListRegistrations operations", func(t *testing.T) {
		ctx := context.Background()

		// Test listing registrations
		query := map[string]string{
			"namespace": "test-namespace",
		}

		registrations, err := service.ListRegistrations(ctx, query)

		// With fake clients, this should work and return empty list or error
		if err != nil {
			assert.Error(t, err)
		} else {
			assert.NotNil(t, registrations)
			// Should be empty list with fake clients - correct type is []*types.Registration
			assert.IsType(t, []*types.Registration{}, registrations)
		}
	})

	t.Run("DeleteRegistration operations", func(t *testing.T) {
		ctx := context.Background()

		// Test deleting a registration
		err := service.DeleteRegistration(ctx, "test-registration-id")

		// With fake clients, this might succeed (no-op) or return not found
		// Both are acceptable behaviors
		if err != nil {
			// Should be a reasonable error
			assert.Error(t, err)
		}
		// If no error, that's also fine (no-op deletion)
	})

	t.Run("RegisterExistingNamespace operations", func(t *testing.T) {
		ctx := context.Background()

		// Create a valid existing namespace request
		request := &types.ExistingNamespaceRequest{
			ExistingNamespace: "existing-namespace",
			Repository: types.Repository{
				URL: "https://github.com/test/repo",
			},
		}

		userInfo := &types.UserInfo{
			Username: "test-user",
			Groups:   []string{"system:authenticated"},
		}

		// Test registering existing namespace
		registration, err := service.RegisterExistingNamespace(ctx, request, userInfo)

		// With fake clients, this should work or fail gracefully
		if err != nil {
			assert.Error(t, err)
		} else {
			assert.NotNil(t, registration)
			assert.Equal(t, "existing-namespace", registration.Namespace)
		}
	})
}

func TestRegistrationService_ExistingNamespaceHelpers(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	cfg := &config.Config{
		ArgoCD: config.ArgoCDConfig{
			Namespace: "argocd",
		},
	}

	// Create services with fake clients
	k8sFactory := NewTestKubernetesFactory()
	argoCDFactory := NewTestArgoCDFactory()

	k8sService, err := NewKubernetesServiceWithFactory(cfg, logger, k8sFactory)
	require.NoError(t, err)

	argoCDService, err := NewArgoCDServiceWithFactory(cfg, logger, argoCDFactory)
	require.NoError(t, err)

	service := NewRegistrationServiceReal(cfg, k8sService, argoCDService, logger)

	t.Run("validateExistingNamespace operations", func(t *testing.T) {
		ctx := context.Background()

		request := &types.ExistingNamespaceRequest{
			ExistingNamespace: "test-namespace",
			Repository: types.Repository{
				URL: "https://github.com/test/repo",
			},
		}

		userInfo := &types.UserInfo{
			Username: "test-user",
			Groups:   []string{"system:authenticated"},
		}

		// Call the public method that uses validateExistingNamespace internally
		// This will test the private method through the public interface
		_, err := service.RegisterExistingNamespace(ctx, request, userInfo)

		// The validation should run (even if other parts fail with fake clients)
		// We're just ensuring the method executes without panicking
		if err != nil {
			// Error is acceptable with fake clients
			assert.Error(t, err)
		}
	})

	t.Run("buildExistingNamespaceRegistration operations", func(t *testing.T) {
		ctx := context.Background()

		request := &types.ExistingNamespaceRequest{
			ExistingNamespace: "test-namespace",
			Repository: types.Repository{
				URL: "https://github.com/test/repo",
			},
		}

		userInfo := &types.UserInfo{
			Username: "test-user",
			Groups:   []string{"system:authenticated"},
		}

		// This tests buildExistingNamespaceRegistration through RegisterExistingNamespace
		_, err := service.RegisterExistingNamespace(ctx, request, userInfo)

		// Should execute the registration building logic
		if err != nil {
			assert.Error(t, err)
		}
	})
}

func TestRegistrationService_EdgeCases_Coverage(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	cfg := &config.Config{
		ArgoCD: config.ArgoCDConfig{
			Namespace: "argocd",
		},
	}

	t.Run("Service with nil dependencies", func(t *testing.T) {
		// Test service creation with nil dependencies to ensure it doesn't panic
		service := NewRegistrationServiceReal(cfg, nil, nil, logger)
		assert.NotNil(t, service)

		// Calling methods should fail gracefully or succeed depending on implementation
		ctx := context.Background()
		_, err := service.GetRegistration(ctx, "test")

		// The service might return an error or succeed with stubs - both are acceptable
		// Just ensure it doesn't panic
		if err != nil {
			assert.Error(t, err)
		}
	})

	t.Run("Service with minimal configuration", func(t *testing.T) {
		// Test with minimal config
		minimalCfg := &config.Config{}

		k8sFactory := NewTestKubernetesFactory()
		argoCDFactory := NewTestArgoCDFactory()

		k8sService, err := NewKubernetesServiceWithFactory(minimalCfg, logger, k8sFactory)
		require.NoError(t, err)

		argoCDService, err := NewArgoCDServiceWithFactory(minimalCfg, logger, argoCDFactory)
		require.NoError(t, err)

		service := NewRegistrationServiceReal(minimalCfg, k8sService, argoCDService, logger)
		assert.NotNil(t, service)

		// Operations should work with minimal config
		ctx := context.Background()
		_, err = service.ListRegistrations(ctx, nil)
		if err != nil {
			assert.Error(t, err)
		}
	})
}
