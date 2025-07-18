package services

import (
	"errors"
	"testing"

	"github.com/konflux-ci/gitops-registration-service/internal/config"
	"github.com/konflux-ci/gitops-registration-service/internal/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
)

func TestConvertResourceListToInterface(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	// Use the real argoCDService for testing utility functions
	service := &argoCDService{
		logger: logger,
	}

	tests := []struct {
		name      string
		resources []types.AppProjectResource
		expected  []interface{}
	}{
		{
			name:      "Empty resource list",
			resources: []types.AppProjectResource{},
			expected:  []interface{}{},
		},
		{
			name: "Single resource with group",
			resources: []types.AppProjectResource{
				{Group: "apps", Kind: "Deployment"},
			},
			expected: []interface{}{
				map[string]interface{}{
					"group": "apps",
					"kind":  "Deployment",
				},
			},
		},
		{
			name: "Single resource without group (core resource)",
			resources: []types.AppProjectResource{
				{Group: "", Kind: "ConfigMap"},
			},
			expected: []interface{}{
				map[string]interface{}{
					"group": "",
					"kind":  "ConfigMap",
				},
			},
		},
		{
			name: "Multiple resources",
			resources: []types.AppProjectResource{
				{Group: "apps", Kind: "Deployment"},
				{Group: "", Kind: "ConfigMap"},
				{Group: "kafka.strimzi.io", Kind: "KafkaTopic"},
			},
			expected: []interface{}{
				map[string]interface{}{
					"group": "apps",
					"kind":  "Deployment",
				},
				map[string]interface{}{
					"group": "",
					"kind":  "ConfigMap",
				},
				map[string]interface{}{
					"group": "kafka.strimzi.io",
					"kind":  "KafkaTopic",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.convertResourceListToInterface(tt.resources)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildProjectSpec_BasicStructure(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	service := &argoCDService{
		logger: logger,
	}

	project := &types.AppProject{
		Name: "test-project",
		SourceRepos: []string{
			"https://github.com/test/repo",
		},
		Destinations: []types.AppProjectDestination{
			{
				Namespace: "test-namespace",
				Server:    "https://kubernetes.default.svc",
			},
		},
	}

	spec := service.buildProjectSpec(project)

	// Test basic structure
	assert.Equal(t, project.SourceRepos, spec["sourceRepos"])

	// Check destinations structure
	destinations := spec["destinations"].([]interface{})
	assert.Len(t, destinations, 1)

	firstDest := destinations[0].(map[string]interface{})
	assert.Equal(t, "test-namespace", firstDest["namespace"])
	assert.Equal(t, "https://kubernetes.default.svc", firstDest["server"])

	// Check roles structure
	roles := spec["roles"].([]interface{})
	assert.Len(t, roles, 1)

	role := roles[0].(map[string]interface{})
	assert.Equal(t, "tenant-role", role["name"])
	assert.NotEmpty(t, role["policies"])
}

func TestAddResourceRestrictions_WithWhitelist(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	service := &argoCDService{
		logger: logger,
	}

	project := &types.AppProject{
		Name: "test-project",
		ClusterResourceWhitelist: []types.AppProjectResource{
			{Group: "", Kind: "Namespace"},
		},
		NamespaceResourceWhitelist: []types.AppProjectResource{
			{Group: "", Kind: "ConfigMap"},
			{Group: "apps", Kind: "Deployment"},
		},
	}

	spec := map[string]interface{}{}
	service.addResourceRestrictions(spec, project)

	// Should have whitelist fields
	clusterWhitelist := spec["clusterResourceWhitelist"].([]interface{})
	assert.Len(t, clusterWhitelist, 1)

	namespaceWhitelist := spec["namespaceResourceWhitelist"].([]interface{})
	assert.Len(t, namespaceWhitelist, 2)

	// Should not have blacklist fields
	assert.NotContains(t, spec, "clusterResourceBlacklist")
	assert.NotContains(t, spec, "namespaceResourceBlacklist")
}

func TestAddResourceRestrictions_WithBlacklist(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	service := &argoCDService{
		logger: logger,
	}

	project := &types.AppProject{
		Name: "test-project",
		ClusterResourceBlacklist: []types.AppProjectResource{
			{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
		},
		NamespaceResourceBlacklist: []types.AppProjectResource{
			{Group: "database.example.com", Kind: "MySQLDatabase"},
		},
	}

	spec := map[string]interface{}{}
	service.addResourceRestrictions(spec, project)

	// Should have blacklist fields
	clusterBlacklist := spec["clusterResourceBlacklist"].([]interface{})
	assert.Len(t, clusterBlacklist, 1)

	namespaceBlacklist := spec["namespaceResourceBlacklist"].([]interface{})
	assert.Len(t, namespaceBlacklist, 1)

	// Should not have whitelist fields
	assert.NotContains(t, spec, "clusterResourceWhitelist")
	assert.NotContains(t, spec, "namespaceResourceWhitelist")
}

func TestAddResourceRestrictions_NoRestrictions(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	service := &argoCDService{
		logger: logger,
	}

	project := &types.AppProject{
		Name: "test-project",
		// No resource restrictions
	}

	spec := map[string]interface{}{}
	service.addResourceRestrictions(spec, project)

	// Should have default whitelist
	assert.Contains(t, spec, "namespaceResourceWhitelist")

	namespaceWhitelist := spec["namespaceResourceWhitelist"].([]interface{})
	assert.NotEmpty(t, namespaceWhitelist)
}

func TestArgoCDService_DataStructures(t *testing.T) {
	// Test the ArgoCD service can be created with basic fields
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	service := &argoCDService{
		logger:    logger,
		namespace: "argocd",
	}

	assert.NotNil(t, service.logger)
	assert.Equal(t, "argocd", service.namespace)
}

func TestArgoCDService_UtilityFunctions_Coverage(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	service := &argoCDService{
		logger: logger,
	}

	// Note: Real client-dependent functions like deleteResource, HealthCheck, and
	// CheckAppProjectConflict are tested indirectly through integration tests
	// Here we focus on utility functions that can be tested in isolation

	// Test that the service structure is correct
	assert.NotNil(t, service.logger)

	// Test convertResourceListToInterface is already covered above
	// Test buildProjectSpec is already covered above
	// Test addResourceRestrictions is already covered above
}

func TestTypes_AppProject_Structure(t *testing.T) {
	// Test AppProject structure and field access
	project := &types.AppProject{
		Name:      "test-project",
		Namespace: "argocd",
		SourceRepos: []string{
			"https://github.com/test/repo",
		},
		Destinations: []types.AppProjectDestination{
			{
				Server:    "https://kubernetes.default.svc",
				Namespace: "test-namespace",
			},
		},
		ClusterResourceWhitelist: []types.AppProjectResource{
			{Group: "", Kind: "Namespace"},
		},
		NamespaceResourceWhitelist: []types.AppProjectResource{
			{Group: "", Kind: "ConfigMap"},
			{Group: "apps", Kind: "Deployment"},
		},
	}

	assert.Equal(t, "test-project", project.Name)
	assert.Equal(t, "argocd", project.Namespace)
	assert.Len(t, project.SourceRepos, 1)
	assert.Len(t, project.Destinations, 1)
	assert.Len(t, project.ClusterResourceWhitelist, 1)
	assert.Len(t, project.NamespaceResourceWhitelist, 2)

	assert.Equal(t, "test-namespace", project.Destinations[0].Namespace)
	assert.Equal(t, "ConfigMap", project.NamespaceResourceWhitelist[0].Kind)
	assert.Equal(t, "apps", project.NamespaceResourceWhitelist[1].Group)
}

func TestTypes_Application_Structure(t *testing.T) {
	// Test Application structure
	app := &types.Application{
		Name:      "test-app",
		Namespace: "argocd",
		Project:   "test-project",
		Source: types.ApplicationSource{
			RepoURL:        "https://github.com/test/repo",
			Path:           ".",
			TargetRevision: "HEAD",
		},
		Destination: types.ApplicationDestination{
			Server:    "https://kubernetes.default.svc",
			Namespace: "test-namespace",
		},
		SyncPolicy: types.ApplicationSyncPolicy{
			Automated: &types.ApplicationSyncPolicyAutomated{
				Prune:    true,
				SelfHeal: true,
			},
		},
	}

	assert.Equal(t, "test-app", app.Name)
	assert.Equal(t, "test-project", app.Project)
	assert.Equal(t, "https://github.com/test/repo", app.Source.RepoURL)
	assert.NotNil(t, app.SyncPolicy.Automated)
	assert.True(t, app.SyncPolicy.Automated.Prune)
	assert.True(t, app.SyncPolicy.Automated.SelfHeal)
}

func TestBuildAppProjectResource(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	service := &argoCDService{
		logger:    logger,
		namespace: "argocd",
	}

	project := &types.AppProject{
		Name:      "test-project",
		Namespace: "argocd",
		SourceRepos: []string{
			"https://github.com/test/repo",
		},
		Destinations: []types.AppProjectDestination{
			{
				Namespace: "test-namespace",
				Server:    "https://kubernetes.default.svc",
			},
		},
	}

	spec := map[string]interface{}{
		"sourceRepos": project.SourceRepos,
		"destinations": []interface{}{
			map[string]interface{}{
				"namespace": "test-namespace",
				"server":    "https://kubernetes.default.svc",
			},
		},
	}

	resource := service.buildAppProjectResource(project, spec)

	// Test basic structure
	assert.Equal(t, "AppProject", resource.Object["kind"])
	assert.Equal(t, "argoproj.io/v1alpha1", resource.Object["apiVersion"])

	// Test metadata
	metadata := resource.Object["metadata"].(map[string]interface{})
	assert.Equal(t, project.Name, metadata["name"])
	assert.Equal(t, service.namespace, metadata["namespace"])

	// Test labels
	labels := metadata["labels"].(map[string]interface{})
	assert.Equal(t, "gitops-registration-service", labels["gitops.io/managed-by"])
	assert.Equal(t, "gitops-registration-service", labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, project.Destinations[0].Namespace, labels["gitops.io/tenant"])

	// Test spec is correctly embedded
	embeddedSpec := resource.Object["spec"].(map[string]interface{})
	assert.Equal(t, spec["sourceRepos"], embeddedSpec["sourceRepos"])
	assert.Equal(t, spec["destinations"], embeddedSpec["destinations"])
}

func TestNewArgoCDServiceReal_Constructor(t *testing.T) {
	logger := logrus.New()
	cfg := &config.Config{}

	t.Run("Constructor fails outside Kubernetes cluster", func(t *testing.T) {
		// When running unit tests outside a Kubernetes cluster,
		// InClusterConfig() should fail
		service, err := NewArgoCDServiceReal(cfg, logger)

		// Expect error since we're not in a Kubernetes cluster
		assert.Error(t, err)
		assert.Nil(t, service)
		assert.Contains(t, err.Error(), "failed to create config")
	})

	t.Run("Constructor with nil config", func(t *testing.T) {
		// Test behavior with nil config - should still fail on InClusterConfig
		service, err := NewArgoCDServiceReal(nil, logger)

		assert.Error(t, err)
		assert.Nil(t, service)
		assert.Contains(t, err.Error(), "failed to create config")
	})

	t.Run("Constructor with nil logger", func(t *testing.T) {
		// Test behavior with nil logger - should still fail on InClusterConfig
		service, err := NewArgoCDServiceReal(cfg, nil)

		assert.Error(t, err)
		assert.Nil(t, service)
		assert.Contains(t, err.Error(), "failed to create config")
	})
}

func TestArgoCDService_StructureValidation(t *testing.T) {
	t.Run("argoCDService struct implements ArgoCDService interface", func(t *testing.T) {
		// Compile-time check that argoCDService implements ArgoCDService
		var _ ArgoCDService = (*argoCDService)(nil)

		// Test struct field types
		service := &argoCDService{}
		assert.NotNil(t, service)

		// Ensure struct can be instantiated
		service.namespace = "test-namespace"
		assert.Equal(t, "test-namespace", service.namespace)
	})
}

func TestArgoCDService_Constants(t *testing.T) {
	t.Run("ArgoCD API constants are defined", func(t *testing.T) {
		// Verify the GVR constants exist and are properly formed
		assert.Equal(t, "argoproj.io", appProjectGVR.Group)
		assert.Equal(t, "v1alpha1", appProjectGVR.Version)
		assert.Equal(t, "appprojects", appProjectGVR.Resource)

		assert.Equal(t, "argoproj.io", applicationGVR.Group)
		assert.Equal(t, "v1alpha1", applicationGVR.Version)
		assert.Equal(t, "applications", applicationGVR.Resource)
	})
}

func TestNewArgoCDServiceWithFactory(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	cfg := &config.Config{
		ArgoCD: config.ArgoCDConfig{
			Namespace: "argocd",
		},
	}

	t.Run("Successful service creation with test factory", func(t *testing.T) {
		factory := NewTestArgoCDFactory()

		service, err := NewArgoCDServiceWithFactory(cfg, logger, factory)

		assert.NoError(t, err)
		assert.NotNil(t, service)

		// Verify service implements interface
		var _ ArgoCDService = service
	})

	t.Run("Factory config creation fails", func(t *testing.T) {
		testError := errors.New("argocd config creation failed")
		factory := NewErrorArgoCDFactory(testError)

		service, err := NewArgoCDServiceWithFactory(cfg, logger, factory)

		assert.Error(t, err)
		assert.Nil(t, service)
		assert.Contains(t, err.Error(), "failed to create config")
		assert.Contains(t, err.Error(), testError.Error())
	})

	t.Run("Factory dynamic client creation fails", func(t *testing.T) {
		// Create a factory that succeeds config creation but fails client creation
		factory := &TestArgoCDFactory{
			Config: &rest.Config{Host: "https://test-argocd"},
		}

		// Create a separate call to test the dynamic client creation failure path
		// First test config creation succeeds
		config, err := factory.CreateConfig()
		require.NoError(t, err)
		assert.NotNil(t, config)

		// Now set error for dynamic client creation and test that path
		factory.Error = errors.New("dynamic client creation failed")
		client, err := factory.CreateDynamicClient(config)
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "dynamic client creation failed")
	})

	t.Run("Service created with custom fake client", func(t *testing.T) {
		// Create a custom fake dynamic client
		scheme := runtime.NewScheme()
		fakeClient := fakedynamic.NewSimpleDynamicClient(scheme)
		factory := &TestArgoCDFactory{
			Client: fakeClient,
			Config: &rest.Config{Host: "https://custom-argocd"},
		}

		service, err := NewArgoCDServiceWithFactory(cfg, logger, factory)

		assert.NoError(t, err)
		assert.NotNil(t, service)

		// Note: Skip health check test since fake client needs ArgoCD CRDs registered
		// We can test the service was created successfully without calling methods that require real CRDs
	})

	t.Run("Service configuration validation", func(t *testing.T) {
		factory := NewTestArgoCDFactory()

		// Test with nil config
		service, err := NewArgoCDServiceWithFactory(nil, logger, factory)
		assert.NoError(t, err) // Should work since config is passed to service
		assert.NotNil(t, service)

		// Test with nil logger
		service, err = NewArgoCDServiceWithFactory(cfg, nil, factory)
		assert.NoError(t, err) // Should work since logger is passed to service
		assert.NotNil(t, service)
	})

	t.Run("Error handling scenarios", func(t *testing.T) {
		// Test with error-returning factory
		factory := NewErrorArgoCDFactory(errors.New("connection failed"))
		service, err := NewArgoCDServiceWithFactory(cfg, logger, factory)
		assert.Error(t, err)
		assert.Nil(t, service)
	})
}

func TestArgoCDServiceWithFakeClient_Operations(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	cfg := &config.Config{
		ArgoCD: config.ArgoCDConfig{
			Namespace: "argocd",
		},
	}

	t.Run("Basic service operations", func(t *testing.T) {
		// Create service with fake client - skip operations that require CRD registration
		factory := NewTestArgoCDFactory()
		service, err := NewArgoCDServiceWithFactory(cfg, logger, factory)
		require.NoError(t, err)

		// Note: We can test service creation but skip health check and CRD operations
		// since they require ArgoCD CRDs to be registered in the fake client scheme
		assert.NotNil(t, service)
	})

	t.Run("Service with custom ArgoCD namespace", func(t *testing.T) {
		customCfg := &config.Config{
			ArgoCD: config.ArgoCDConfig{
				Namespace: "custom-argocd",
			},
		}

		factory := NewTestArgoCDFactory()
		service, err := NewArgoCDServiceWithFactory(customCfg, logger, factory)
		require.NoError(t, err)

		// Verify the service was created successfully
		assert.NotNil(t, service)
	})

	t.Run("Error handling scenarios", func(t *testing.T) {
		// Test with error-returning factory
		factory := NewErrorArgoCDFactory(errors.New("connection failed"))
		service, err := NewArgoCDServiceWithFactory(cfg, logger, factory)
		assert.Error(t, err)
		assert.Nil(t, service)
	})
}
