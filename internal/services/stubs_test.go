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

func TestKubernetesServiceStub_HealthCheck(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	stub := &kubernetesServiceStub{logger: logger}

	ctx := context.Background()
	err := stub.HealthCheck(ctx)
	assert.NoError(t, err, "Health check should always succeed for stub")
}

func TestKubernetesServiceStub_CreateNamespace(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	stub := &kubernetesServiceStub{logger: logger}

	ctx := context.Background()
	labels := map[string]string{"app": "test"}

	err := stub.CreateNamespace(ctx, "test-namespace", labels)
	assert.NoError(t, err, "CreateNamespace should succeed for stub")
}

func TestKubernetesServiceStub_DeleteNamespace(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	stub := &kubernetesServiceStub{logger: logger}

	ctx := context.Background()
	err := stub.DeleteNamespace(ctx, "test-namespace")
	assert.NoError(t, err, "DeleteNamespace should succeed for stub")
}

func TestKubernetesServiceStub_NamespaceExists(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	stub := &kubernetesServiceStub{logger: logger}

	ctx := context.Background()
	exists, err := stub.NamespaceExists(ctx, "test-namespace")
	assert.NoError(t, err)
	assert.False(t, exists, "Stub should return false for namespace existence")
}

func TestKubernetesServiceStub_CountNamespaces(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	stub := &kubernetesServiceStub{logger: logger}

	ctx := context.Background()
	count, err := stub.CountNamespaces(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 5, count, "Stub should return fixed value of 5")
}

func TestArgoCDServiceStub_HealthCheck(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	stub := &argoCDServiceStub{logger: logger}

	ctx := context.Background()
	err := stub.HealthCheck(ctx)
	assert.NoError(t, err, "ArgoCD health check should always succeed for stub")
}

func TestArgoCDServiceStub_CreateAppProject(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	stub := &argoCDServiceStub{logger: logger}

	ctx := context.Background()
	project := &types.AppProject{
		Name:      "test-project",
		Namespace: "argocd",
	}

	err := stub.CreateAppProject(ctx, project)
	assert.NoError(t, err, "CreateAppProject should succeed for stub")
}

func TestArgoCDServiceStub_CreateApplication(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	stub := &argoCDServiceStub{logger: logger}

	ctx := context.Background()
	app := &types.Application{
		Name:      "test-app",
		Namespace: "argocd",
		Project:   "test-project",
	}

	err := stub.CreateApplication(ctx, app)
	assert.NoError(t, err, "CreateApplication should succeed for stub")
}

func TestArgoCDServiceStub_GetApplicationStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	stub := &argoCDServiceStub{logger: logger}

	ctx := context.Background()
	status, err := stub.GetApplicationStatus(ctx, "test-app")
	require.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "Synced", status.Phase)
	assert.Equal(t, "Healthy", status.Health)
	assert.Equal(t, "Synced", status.Sync)
	assert.Equal(t, "Application is healthy (stub)", status.Message)
}

func TestAuthorizationServiceStub_ValidateNamespaceAccess(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{
		Authorization: config.AuthorizationConfig{
			RequiredRole: "test-role",
		},
	}

	k8sStub := &kubernetesServiceStub{logger: logger}
	stub := &authorizationServiceStub{
		cfg:    cfg,
		k8s:    k8sStub,
		logger: logger,
	}

	ctx := context.Background()
	userInfo := &types.UserInfo{
		Username: "test-user",
		Email:    "test@example.com",
	}

	err := stub.ValidateNamespaceAccess(ctx, userInfo, "test-namespace")
	assert.NoError(t, err, "ValidateNamespaceAccess should succeed for stub")
}

func TestAuthorizationServiceStub_ExtractUserInfo(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{}
	k8sStub := &kubernetesServiceStub{logger: logger}
	stub := &authorizationServiceStub{
		cfg:    cfg,
		k8s:    k8sStub,
		logger: logger,
	}

	ctx := context.Background()
	userInfo, err := stub.ExtractUserInfo(ctx, "test-token")
	require.NoError(t, err)
	assert.NotNil(t, userInfo)
	assert.Equal(t, "stub-user", userInfo.Username)
	assert.Equal(t, "stub@example.com", userInfo.Email)
	assert.Contains(t, userInfo.Groups, "stub-group")
}

func TestAuthorizationServiceStub_IsAdminUser(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{}
	k8sStub := &kubernetesServiceStub{logger: logger}
	stub := &authorizationServiceStub{
		cfg:    cfg,
		k8s:    k8sStub,
		logger: logger,
	}

	userInfo := &types.UserInfo{Username: "test-user"}
	isAdmin := stub.IsAdminUser(userInfo)
	assert.False(t, isAdmin, "Stub should return false for admin user check")
}

/*
func TestCapacityServiceStub_GetCurrentCapacity(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{
		Capacity: config.CapacityConfig{
			Enabled: true,
			Limits: config.CapacityLimits{
				MaxNamespaces:      500,
				EmergencyThreshold: 0.9,
			},
		},
	}

	stub := &capacityServiceStub{
		cfg:    cfg,
		logger: logger,
	}

	ctx := context.Background()
	capacity, err := stub.GetCurrentCapacity(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, capacity)
	assert.Equal(t, true, capacity.Enabled)
	assert.Equal(t, "normal", capacity.Status)
	assert.Equal(t, 100, capacity.Current.Namespaces)
	assert.Equal(t, 0.2, capacity.Current.UtilizationPercent)
}

func TestCapacityServiceStub_CheckCapacityForNewNamespace_Normal(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{
		Capacity: config.CapacityConfig{
			Enabled: true,
			Limits: config.CapacityLimits{
				MaxNamespaces:      500,
				EmergencyThreshold: 0.9,
			},
		},
	}

	stub := &capacityServiceStub{
		cfg:    cfg,
		logger: logger,
	}

	ctx := context.Background()
	err := stub.CheckCapacityForNewNamespace(ctx, &types.UserInfo{Username: "test-user"})
	assert.NoError(t, err)
}

func TestCapacityServiceStub_CheckCapacityForNewNamespace_AtLimit(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{
		Capacity: config.CapacityConfig{
			Enabled: true,
			Limits: config.CapacityLimits{
				MaxNamespaces:      100, // Set low limit
				EmergencyThreshold: 0.9,
			},
		},
	}

	stub := &capacityServiceStub{
		cfg:    cfg,
		logger: logger,
	}

	ctx := context.Background()
	err := stub.CheckCapacityForNewNamespace(ctx, &types.UserInfo{Username: "test-user"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "capacity threshold exceeded")
}
*/

func TestRegistrationServiceStub_CreateRegistration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{}
	k8sStub := &kubernetesServiceStub{logger: logger}
	argoCDStub := &argoCDServiceStub{logger: logger}

	stub := &registrationServiceStub{
		cfg:    cfg,
		k8s:    k8sStub,
		argocd: argoCDStub,
		logger: logger,
	}

	ctx := context.Background()
	req := &types.RegistrationRequest{
		Repository: types.Repository{
			URL:    "https://github.com/test/repo",
			Branch: "main",
		},
		Namespace: "test-namespace",
	}

	registration, err := stub.CreateRegistration(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, registration)
	assert.Equal(t, "stub-reg-123", registration.ID)
	assert.Equal(t, "test-namespace", registration.Namespace)
	assert.Equal(t, "pending", registration.Status.Phase)
}

func TestRegistrationServiceStub_RegisterExistingNamespace(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{}
	k8sStub := &kubernetesServiceStub{logger: logger}
	argoCDStub := &argoCDServiceStub{logger: logger}

	stub := &registrationServiceStub{
		cfg:    cfg,
		k8s:    k8sStub,
		argocd: argoCDStub,
		logger: logger,
	}

	ctx := context.Background()
	req := &types.ExistingNamespaceRequest{
		Repository: types.Repository{
			URL:    "https://github.com/test/existing-repo",
			Branch: "main",
		},
		ExistingNamespace: "existing-namespace",
	}

	userInfo := &types.UserInfo{Username: "test-user"}

	registration, err := stub.RegisterExistingNamespace(ctx, req, userInfo)
	require.NoError(t, err)
	assert.NotNil(t, registration)
	assert.Equal(t, "stub-existing-reg-123", registration.ID)
	assert.Equal(t, "existing-namespace", registration.Namespace)
	assert.Equal(t, "active", registration.Status.Phase)
	assert.False(t, registration.Status.NamespaceCreated, "Existing namespace should not be marked as created")
}

func TestRegistrationServiceStub_GetRegistration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{}
	k8sStub := &kubernetesServiceStub{logger: logger}
	argoCDStub := &argoCDServiceStub{logger: logger}

	stub := &registrationServiceStub{
		cfg:    cfg,
		k8s:    k8sStub,
		argocd: argoCDStub,
		logger: logger,
	}

	ctx := context.Background()
	registration, err := stub.GetRegistration(ctx, "non-existent-id")
	assert.Error(t, err, "Should return error for non-existent registration")
	assert.Nil(t, registration)
	assert.Contains(t, err.Error(), "registration not found")
}

func TestRegistrationServiceStub_ListRegistrations(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{}
	k8sStub := &kubernetesServiceStub{logger: logger}
	argoCDStub := &argoCDServiceStub{logger: logger}

	stub := &registrationServiceStub{
		cfg:    cfg,
		k8s:    k8sStub,
		argocd: argoCDStub,
		logger: logger,
	}

	ctx := context.Background()
	filters := map[string]string{"namespace": "test"}

	registrations, err := stub.ListRegistrations(ctx, filters)
	require.NoError(t, err)
	assert.NotNil(t, registrations)
	assert.Len(t, registrations, 0, "Stub should return empty list")
}

func TestRegistrationServiceStub_DeleteRegistration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{}
	k8sStub := &kubernetesServiceStub{logger: logger}
	argoCDStub := &argoCDServiceStub{logger: logger}

	stub := &registrationServiceStub{
		cfg:    cfg,
		k8s:    k8sStub,
		argocd: argoCDStub,
		logger: logger,
	}

	ctx := context.Background()
	err := stub.DeleteRegistration(ctx, "test-id")
	assert.NoError(t, err, "DeleteRegistration should succeed for stub")
}

func TestRegistrationControlServiceStub_IsNewNamespaceAllowed_Enabled(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{
		Registration: config.RegistrationConfig{
			AllowNewNamespaces: true,
		},
	}

	stub := &registrationControlServiceStub{
		cfg:    cfg,
		logger: logger,
	}

	ctx := context.Background()
	err := stub.IsNewNamespaceAllowed(ctx)
	assert.NoError(t, err, "IsNewNamespaceAllowed should succeed when enabled")
}

func TestRegistrationControlServiceStub_IsNewNamespaceAllowed_Disabled(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{
		Registration: config.RegistrationConfig{
			AllowNewNamespaces: false,
		},
	}

	stub := &registrationControlServiceStub{
		cfg:    cfg,
		logger: logger,
	}

	ctx := context.Background()
	err := stub.IsNewNamespaceAllowed(ctx)
	assert.Error(t, err, "IsNewNamespaceAllowed should fail when disabled")
	assert.Contains(t, err.Error(), "new namespace registration is currently disabled")
}

func TestRegistrationControlServiceStub_GetRegistrationStatus_Enabled(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{
		Registration: config.RegistrationConfig{
			AllowNewNamespaces: true,
		},
	}

	stub := &registrationControlServiceStub{
		cfg:    cfg,
		logger: logger,
	}

	ctx := context.Background()
	status, err := stub.GetRegistrationStatus(ctx)
	require.NoError(t, err)
	assert.True(t, status.AllowNewNamespaces)
	assert.Equal(t, "Registration status based on configuration", status.Message)
}

func TestRegistrationControlServiceStub_GetRegistrationStatus_Disabled(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{
		Registration: config.RegistrationConfig{
			AllowNewNamespaces: false,
		},
	}

	stub := &registrationControlServiceStub{
		cfg:    cfg,
		logger: logger,
	}

	ctx := context.Background()
	status, err := stub.GetRegistrationStatus(ctx)
	require.NoError(t, err)
	assert.False(t, status.AllowNewNamespaces)
	assert.Equal(t, "Registration status based on configuration", status.Message)
}
