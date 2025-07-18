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
)

func TestServices_Structure(t *testing.T) {
	// Test Services struct creation and validation
	t.Run("Services with all components", func(t *testing.T) {
		services := &Services{
			Kubernetes:          &kubernetesServiceStub{},
			ArgoCD:              &argoCDServiceStub{},
			Registration:        &registrationServiceStub{},
			RegistrationControl: &registrationControlServiceStub{},
			Authorization:       &authorizationServiceStub{},
		}

		assert.NotNil(t, services.Kubernetes)
		assert.NotNil(t, services.ArgoCD)
		assert.NotNil(t, services.Registration)
		assert.NotNil(t, services.RegistrationControl)
		assert.NotNil(t, services.Authorization)
	})

	t.Run("Services with missing components", func(t *testing.T) {
		services := &Services{
			Kubernetes: &kubernetesServiceStub{},
			// Other services missing
		}

		assert.NotNil(t, services.Kubernetes)
		assert.Nil(t, services.ArgoCD)
		assert.Nil(t, services.Registration)
		assert.Nil(t, services.RegistrationControl)
		assert.Nil(t, services.Authorization)
	})
}

func TestServices_New_ErrorHandling(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	t.Run("New without Kubernetes environment", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{Port: 8080},
		}

		services, err := New(cfg, logger)

		// Should fail due to missing Kubernetes environment variables
		assert.Error(t, err)
		assert.Nil(t, services)
		assert.Contains(t, err.Error(), "kubernetes")
		assert.Contains(t, err.Error(), "in-cluster")
	})

	t.Run("New with nil config", func(t *testing.T) {
		services, err := New(nil, logger)

		// Should fail when trying to create services with nil config
		assert.Error(t, err)
		assert.Nil(t, services)
	})

	t.Run("New with nil logger", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{Port: 8080},
		}

		services, err := New(cfg, nil)

		// Should fail when trying to create services with nil logger
		assert.Error(t, err)
		assert.Nil(t, services)
	})
}

func TestErrorTypes_Structure(t *testing.T) {
	// Test custom error types
	t.Run("NamespaceConflictError", func(t *testing.T) {
		err := &NamespaceConflictError{
			Namespace: "test-namespace",
		}

		assert.Equal(t, "test-namespace", err.Namespace)
		assert.Contains(t, err.Error(), "namespace")
		assert.Contains(t, err.Error(), "test-namespace")
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestClusterRoleValidation_BusinessLogic(t *testing.T) {
	// Test ClusterRoleValidation business logic
	t.Run("Complete validation result", func(t *testing.T) {
		validation := &ClusterRoleValidation{
			Exists:               true,
			HasClusterAdmin:      false,
			HasNamespaceSpanning: true,
			HasClusterScoped:     false,
			Warnings: []string{
				"Role has broad permissions",
				"Consider using namespace-scoped roles",
			},
			ResourceTypes: []string{
				"configmaps",
				"secrets",
				"deployments",
				"services",
			},
		}

		// Test business logic conditions
		assert.True(t, validation.Exists)
		assert.False(t, validation.HasClusterAdmin)
		assert.True(t, validation.HasNamespaceSpanning)
		assert.False(t, validation.HasClusterScoped)
		assert.Len(t, validation.Warnings, 2)
		assert.Len(t, validation.ResourceTypes, 4)

		// Test warning content
		assert.Contains(t, validation.Warnings[0], "broad permissions")
		assert.Contains(t, validation.Warnings[1], "namespace-scoped")

		// Test resource types
		assert.Contains(t, validation.ResourceTypes, "configmaps")
		assert.Contains(t, validation.ResourceTypes, "secrets")
		assert.Contains(t, validation.ResourceTypes, "deployments")
		assert.Contains(t, validation.ResourceTypes, "services")
	})

	t.Run("High-risk validation result", func(t *testing.T) {
		validation := &ClusterRoleValidation{
			Exists:               true,
			HasClusterAdmin:      true,
			HasNamespaceSpanning: true,
			HasClusterScoped:     true,
			Warnings: []string{
				"Role has cluster-admin privileges",
				"Role can access cluster-scoped resources",
			},
			ResourceTypes: []string{"*"},
		}

		// Test high-risk scenarios
		assert.True(t, validation.Exists)
		assert.True(t, validation.HasClusterAdmin)
		assert.True(t, validation.HasNamespaceSpanning)
		assert.True(t, validation.HasClusterScoped)
		assert.Len(t, validation.Warnings, 2)
		assert.Equal(t, []string{"*"}, validation.ResourceTypes)

		// All high-risk flags should be true for dangerous roles
		highRiskCount := 0
		if validation.HasClusterAdmin {
			highRiskCount++
		}
		if validation.HasNamespaceSpanning {
			highRiskCount++
		}
		if validation.HasClusterScoped {
			highRiskCount++
		}
		assert.Equal(t, 3, highRiskCount)
	})
}

func TestServiceTypes_Integration(t *testing.T) {
	// Test service type integration and interface compliance
	ctx := context.Background()

	t.Run("Stub services implement interfaces", func(t *testing.T) {
		// Test that all stub services implement their respective interfaces
		var k8sService KubernetesService = &kubernetesServiceStub{}
		var argoCDService ArgoCDService = &argoCDServiceStub{}
		var regService RegistrationService = &registrationServiceStub{}
		var regControlService RegistrationControlService = &registrationControlServiceStub{}
		var authService AuthorizationService = &authorizationServiceStub{}

		assert.NotNil(t, k8sService)
		assert.NotNil(t, argoCDService)
		assert.NotNil(t, regService)
		assert.NotNil(t, regControlService)
		assert.NotNil(t, authService)

		// Test basic interface methods don't panic
		_ = k8sService.HealthCheck(ctx)

		// Call ArgoCD service health check
		_ = argoCDService.HealthCheck(ctx)
	})
}

func TestApplicationTypes_Validation(t *testing.T) {
	// Test Application type structure and validation
	t.Run("Complete Application structure", func(t *testing.T) {
		app := &types.Application{
			Name:      "test-app",
			Namespace: "argocd",
			Project:   "test-project",
			Source: types.ApplicationSource{
				RepoURL:        "https://github.com/test/repo",
				Path:           "manifests/",
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
				SyncOptions: []string{
					"CreateNamespace=true",
					"PrunePropagationPolicy=foreground",
				},
			},
		}

		// Test structure validation
		assert.Equal(t, "test-app", app.Name)
		assert.Equal(t, "argocd", app.Namespace)
		assert.Equal(t, "test-project", app.Project)
		assert.Equal(t, "https://github.com/test/repo", app.Source.RepoURL)
		assert.Equal(t, "manifests/", app.Source.Path)
		assert.Equal(t, "HEAD", app.Source.TargetRevision)
		assert.Equal(t, "https://kubernetes.default.svc", app.Destination.Server)
		assert.Equal(t, "test-namespace", app.Destination.Namespace)
		assert.NotNil(t, app.SyncPolicy.Automated)
		assert.True(t, app.SyncPolicy.Automated.Prune)
		assert.True(t, app.SyncPolicy.Automated.SelfHeal)
		assert.Len(t, app.SyncPolicy.SyncOptions, 2)
		assert.Contains(t, app.SyncPolicy.SyncOptions, "CreateNamespace=true")
	})
}

func TestNewWithFactories_Comprehensive(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	cfg := &config.Config{
		ArgoCD: config.ArgoCDConfig{
			Namespace: "argocd",
		},
	}

	t.Run("Successful creation with test factories", func(t *testing.T) {
		k8sFactory := NewTestKubernetesFactory()
		argoCDFactory := NewTestArgoCDFactory()

		services, err := NewWithFactories(cfg, logger, k8sFactory, argoCDFactory)
		assert.NoError(t, err)
		assert.NotNil(t, services)
		assert.NotNil(t, services.Kubernetes)
		assert.NotNil(t, services.ArgoCD)
	})

	t.Run("Error handling scenarios", func(t *testing.T) {
		// Test with error-returning Kubernetes factory
		errorK8sFactory := NewErrorKubernetesFactory(errors.New("k8s factory failed"))
		services, err := NewWithFactories(cfg, logger, errorK8sFactory, NewTestArgoCDFactory())
		assert.Error(t, err)
		assert.Nil(t, services)

		// Test with error-returning ArgoCD factory
		errorArgoCDFactory := NewErrorArgoCDFactory(errors.New("argocd factory failed"))
		services, err = NewWithFactories(cfg, logger, NewTestKubernetesFactory(), errorArgoCDFactory)
		assert.Error(t, err)
		assert.Nil(t, services)

		// Test that the function executes without panic with valid inputs
		services, err = NewWithFactories(cfg, logger, NewTestKubernetesFactory(), NewTestArgoCDFactory())
		assert.NoError(t, err)
		assert.NotNil(t, services)
	})

	t.Run("Various configuration scenarios", func(t *testing.T) {
		testConfigs := []*config.Config{
			{ArgoCD: config.ArgoCDConfig{Namespace: "argocd"}},
			{ArgoCD: config.ArgoCDConfig{Namespace: "custom-argocd"}},
			{}, // Minimal config
		}

		for i, testCfg := range testConfigs {
			t.Run(fmt.Sprintf("Config variation %d", i+1), func(t *testing.T) {
				k8sFactory := NewTestKubernetesFactory()
				argoCDFactory := NewTestArgoCDFactory()

				services, err := NewWithFactories(testCfg, logger, k8sFactory, argoCDFactory)
				assert.NoError(t, err)
				assert.NotNil(t, services)
			})
		}
	})
}
