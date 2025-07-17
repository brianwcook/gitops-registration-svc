package services

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/konflux-ci/gitops-registration-service/internal/config"
)

func TestNewKubernetesService_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{
		Kubernetes: config.KubernetesConfig{
			Namespace: "test-namespace",
		},
	}

	k8sService, err := NewKubernetesService(cfg, logger)
	require.NoError(t, err)
	assert.NotNil(t, k8sService)

	// Test that it implements the interface
	var _ KubernetesService = k8sService
}

func TestNewArgoCDService_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{
		ArgoCD: config.ArgoCDConfig{
			Server:    "argocd-server.example.com",
			Namespace: "argocd",
			GRPC:      true,
		},
	}

	argoCDService, err := NewArgoCDService(cfg, logger)
	require.NoError(t, err)
	assert.NotNil(t, argoCDService)

	// Test that it implements the interface
	var _ ArgoCDService = argoCDService
}

func TestNewAuthorizationService_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{
		Authorization: config.AuthorizationConfig{
			RequiredRole:              "test-role",
			EnableSubjectAccessReview: true,
		},
	}

	// Create a mock Kubernetes service for testing
	k8sService, _ := NewKubernetesService(cfg, logger)

	authService := NewAuthorizationService(cfg, k8sService, logger)
	assert.NotNil(t, authService)

	// Test that it implements the interface
	var _ AuthorizationService = authService
}

func TestNewRegistrationService_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{
		ArgoCD: config.ArgoCDConfig{
			Server:    "argocd-server.example.com",
			Namespace: "argocd",
		},
	}

	k8sService, _ := NewKubernetesService(cfg, logger)
	argoCDService, _ := NewArgoCDService(cfg, logger)

	registrationService := NewRegistrationService(cfg, k8sService, argoCDService, logger)
	assert.NotNil(t, registrationService)

	// Test that it implements the interface
	var _ RegistrationService = registrationService
}
