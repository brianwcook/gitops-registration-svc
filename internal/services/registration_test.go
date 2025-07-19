package services

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/konflux-ci/gitops-registration-service/internal/config"
	"github.com/konflux-ci/gitops-registration-service/internal/types"
)

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
