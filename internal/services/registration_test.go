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

			project := regService.buildAppProject(tt.projectName, tt.namespace, tt.repoURL)
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
	project := regService.buildAppProject("test-project", "restricted-namespace", "https://github.com/test/repo")

	require.NotNil(t, project)
	require.Len(t, project.Destinations, 1)

	destination := project.Destinations[0]
	assert.Equal(t, "https://kubernetes.default.svc", destination.Server)
	assert.Equal(t, "restricted-namespace", destination.Namespace)

	// Ensure the AppProject can only deploy to the specified namespace
	assert.Equal(t, "restricted-namespace", destination.Namespace)
}
