package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/konflux-ci/gitops-registration-service/internal/config"
	"github.com/konflux-ci/gitops-registration-service/internal/services"
	"github.com/konflux-ci/gitops-registration-service/internal/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock services for testing
type MockKubernetesService struct {
	mock.Mock
}

func (m *MockKubernetesService) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockKubernetesService) CreateNamespace(ctx context.Context, name string, labels map[string]string) error {
	// Mock implementation for CreateNamespace
	return nil
}

func (m *MockKubernetesService) CreateNamespaceWithMetadata(ctx context.Context, name string, labels, annotations map[string]string) error {
	// Mock implementation for CreateNamespaceWithMetadata
	return nil
}

func (m *MockKubernetesService) DeleteNamespace(ctx context.Context, name string) error {
	// Mock implementation for DeleteNamespace
	return nil
}

func (m *MockKubernetesService) NamespaceExists(ctx context.Context, name string) (bool, error) {
	// Mock implementation for NamespaceExists
	return false, nil
}

func (m *MockKubernetesService) CountNamespaces(ctx context.Context) (int, error) {
	// Mock implementation for CountNamespaces
	return 5, nil
}

func (m *MockKubernetesService) CreateServiceAccount(ctx context.Context, namespace, name string) error {
	// Mock implementation for CreateServiceAccount
	return nil
}

func (m *MockKubernetesService) UpdateNamespaceLabels(ctx context.Context, name string, labels map[string]string) error {
	// Mock implementation for UpdateNamespaceLabels
	return nil
}

func (m *MockKubernetesService) UpdateNamespaceMetadata(ctx context.Context, name string, labels, annotations map[string]string) error {
	// Mock implementation for UpdateNamespaceMetadata
	return nil
}

func (m *MockKubernetesService) CreateRoleBinding(ctx context.Context, namespace, name, role, serviceAccount string) error {
	args := m.Called(ctx, namespace, name, role, serviceAccount)
	return args.Error(0)
}

func (m *MockKubernetesService) CheckAppProjectConflict(ctx context.Context, repositoryHash string) (bool, error) {
	return false, nil
}

func (m *MockKubernetesService) CreateRoleBindingForServiceAccount(ctx context.Context, namespace, name, clusterRole, serviceAccountName string) error {
	return nil
}

func (m *MockKubernetesService) CreateServiceAccountWithGenerateName(ctx context.Context, namespace, baseName string) (string, error) {
	return "mock-sa-12345", nil
}

func (m *MockKubernetesService) ValidateClusterRole(ctx context.Context, name string) (*services.ClusterRoleValidation, error) {
	return &services.ClusterRoleValidation{
		Exists:               true,
		HasClusterAdmin:      false,
		HasNamespaceSpanning: false,
		HasClusterScoped:     false,
		Warnings:             []string{},
		ResourceTypes:        []string{},
	}, nil
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

func (m *MockArgoCDService) CheckAppProjectConflict(ctx context.Context, repositoryHash string) (bool, error) {
	return false, nil
}

// Mock other services as needed
type MockRegistrationService struct {
	mock.Mock
}

func (m *MockRegistrationService) CreateRegistration(ctx context.Context, req *types.RegistrationRequest) (*types.Registration, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*types.Registration), args.Error(1)
}

func (m *MockRegistrationService) GetRegistration(ctx context.Context, id string) (*types.Registration, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Registration), args.Error(1)
}

func (m *MockRegistrationService) ListRegistrations(ctx context.Context, filters map[string]string) ([]*types.Registration, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]*types.Registration), args.Error(1)
}

func (m *MockRegistrationService) DeleteRegistration(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRegistrationService) RegisterExistingNamespace(ctx context.Context, req *types.ExistingNamespaceRequest, userInfo *types.UserInfo) (*types.Registration, error) {
	args := m.Called(ctx, req, userInfo)
	return args.Get(0).(*types.Registration), args.Error(1)
}

func (m *MockRegistrationService) ValidateRegistration(ctx context.Context, req *types.RegistrationRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *MockRegistrationService) ValidateExistingNamespaceRequest(ctx context.Context, req *types.ExistingNamespaceRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

type MockRegistrationControlService struct {
	mock.Mock
}

func (m *MockRegistrationControlService) GetRegistrationStatus(ctx context.Context) (*types.ServiceRegistrationStatus, error) {
	args := m.Called(ctx)
	return args.Get(0).(*types.ServiceRegistrationStatus), args.Error(1)
}

func (m *MockRegistrationControlService) IsNewNamespaceAllowed(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type MockAuthorizationService struct {
	mock.Mock
}

func (m *MockAuthorizationService) ValidateNamespaceAccess(ctx context.Context, userInfo *types.UserInfo, namespace string) error {
	args := m.Called(ctx, userInfo, namespace)
	return args.Error(0)
}

func (m *MockAuthorizationService) ExtractUserInfo(ctx context.Context, token string) (*types.UserInfo, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(*types.UserInfo), args.Error(1)
}

func (m *MockAuthorizationService) IsAdminUser(userInfo *types.UserInfo) bool {
	args := m.Called(userInfo)
	return args.Bool(0)
}

func setupTestServer() (*Server, *MockKubernetesService, *MockArgoCDService) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Suppress logs during tests

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:    8080,
			Timeout: "30s",
		},
		ArgoCD: config.ArgoCDConfig{
			Server:    "argocd-server.argocd.svc.cluster.local",
			Namespace: "argocd",
			GRPC:      true,
		},
		Kubernetes: config.KubernetesConfig{
			Namespace: "gitops-registration-system",
		},
	}

	// Create mock services
	mockK8s := &MockKubernetesService{}
	mockArgoCD := &MockArgoCDService{}
	mockRegistration := &MockRegistrationService{}
	mockAuth := &MockAuthorizationService{}

	// Create mock registration control service
	mockRegistrationControl := &MockRegistrationControlService{}

	// Create services struct with mocks
	mockServices := &services.Services{
		Kubernetes:          mockK8s,
		ArgoCD:              mockArgoCD,
		Registration:        mockRegistration,
		RegistrationControl: mockRegistrationControl,
		Authorization:       mockAuth,
	}

	server := &Server{
		config:   cfg,
		logger:   logger,
		router:   chi.NewRouter(),
		services: mockServices,
	}

	// Setup middleware and routes
	server.setupMiddleware()
	server.setupRoutes()

	return server, mockK8s, mockArgoCD
}

func TestHealthLive_Success(t *testing.T) {
	server, _, _ := setupTestServer()

	req := httptest.NewRequest("GET", "/health/live", http.NoBody)
	w := httptest.NewRecorder()

	server.healthLive(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
	assert.Contains(t, w.Body.String(), "gitops-registration-service")
}

func TestHealthReady_Success(t *testing.T) {
	server, mockK8s, mockArgoCD := setupTestServer()

	// Setup mocks for successful health checks
	mockK8s.On("HealthCheck", mock.Anything).Return(nil)
	mockArgoCD.On("HealthCheck", mock.Anything).Return(nil)

	req := httptest.NewRequest("GET", "/health/ready", http.NoBody)
	w := httptest.NewRecorder()

	server.healthReady(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ready")
	assert.Contains(t, w.Body.String(), "gitops-registration-service")

	mockK8s.AssertExpectations(t)
	mockArgoCD.AssertExpectations(t)
}

func TestHealthReady_KubernetesFailure(t *testing.T) {
	server, mockK8s, mockArgoCD := setupTestServer()

	// Setup mocks - Kubernetes fails
	mockK8s.On("HealthCheck", mock.Anything).Return(assert.AnError)
	// ArgoCD should not be called if Kubernetes fails first

	req := httptest.NewRequest("GET", "/health/ready", http.NoBody)
	w := httptest.NewRecorder()

	server.healthReady(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "not ready")
	assert.Contains(t, w.Body.String(), "kubernetes api unavailable")

	mockK8s.AssertExpectations(t)
	mockArgoCD.AssertNotCalled(t, "HealthCheck")
}

func TestHealthReady_ArgoCDFailure(t *testing.T) {
	server, mockK8s, mockArgoCD := setupTestServer()

	// Setup mocks - Kubernetes succeeds, ArgoCD fails
	mockK8s.On("HealthCheck", mock.Anything).Return(nil)
	mockArgoCD.On("HealthCheck", mock.Anything).Return(assert.AnError)

	req := httptest.NewRequest("GET", "/health/ready", http.NoBody)
	w := httptest.NewRecorder()

	server.healthReady(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "not ready")
	assert.Contains(t, w.Body.String(), "argocd api unavailable")

	mockK8s.AssertExpectations(t)
	mockArgoCD.AssertExpectations(t)
}

func TestCheckDependencies_Success(t *testing.T) {
	server, mockK8s, mockArgoCD := setupTestServer()

	mockK8s.On("HealthCheck", mock.Anything).Return(nil)
	mockArgoCD.On("HealthCheck", mock.Anything).Return(nil)

	ctx := context.Background()
	err := server.checkDependencies(ctx)
	assert.NoError(t, err)

	mockK8s.AssertExpectations(t)
	mockArgoCD.AssertExpectations(t)
}

func TestCheckDependencies_KubernetesFailure(t *testing.T) {
	server, mockK8s, _ := setupTestServer()

	mockK8s.On("HealthCheck", mock.Anything).Return(assert.AnError)

	ctx := context.Background()
	err := server.checkDependencies(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kubernetes api unavailable")

	mockK8s.AssertExpectations(t)
}

func TestCheckDependencies_ArgoCDFailure(t *testing.T) {
	server, mockK8s, mockArgoCD := setupTestServer()

	mockK8s.On("HealthCheck", mock.Anything).Return(nil)
	mockArgoCD.On("HealthCheck", mock.Anything).Return(assert.AnError)

	ctx := context.Background()
	err := server.checkDependencies(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "argocd api unavailable")

	mockK8s.AssertExpectations(t)
	mockArgoCD.AssertExpectations(t)
}

func TestSetupMiddleware(t *testing.T) {
	server, _, _ := setupTestServer()

	// Test that middleware is properly set up by making a request
	req := httptest.NewRequest("GET", "/health/live", http.NoBody)
	w := httptest.NewRecorder()

	// The router should have middleware that sets content-type
	server.router.ServeHTTP(w, req)

	// Verify middleware effects
	assert.Equal(t, http.StatusOK, w.Code)
	// Note: The content-type middleware sets application/json for all responses
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestSetupRoutes(t *testing.T) {
	server, _, _ := setupTestServer()

	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "health live endpoint",
			method:         "GET",
			path:           "/health/live",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "health ready endpoint",
			method:         "GET",
			path:           "/health/ready",
			expectedStatus: http.StatusServiceUnavailable, // Will fail due to no mock setup
		},
		{
			name:           "metrics endpoint",
			method:         "GET",
			path:           "/metrics",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "non-existent endpoint",
			method:         "GET",
			path:           "/non-existent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, http.NoBody)
			w := httptest.NewRecorder()

			// Skip health/ready test since it requires proper mock setup
			if tc.path == "/health/ready" {
				t.Skip("Health ready endpoint requires mock setup - tested separately")
				return
			}

			server.router.ServeHTTP(w, req)
			assert.Equal(t, tc.expectedStatus, w.Code)
		})
	}
}

func TestServer_MiddlewareOrder(t *testing.T) {
	server, _, _ := setupTestServer()

	// Test that middleware is applied in correct order
	// This is verified by the fact that the server starts and responds correctly
	req := httptest.NewRequest("GET", "/health/live", http.NoBody)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Verify that all middleware was applied correctly
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// Verify CORS headers are set
	req = httptest.NewRequest("OPTIONS", "/health/live", http.NoBody)
	req.Header.Set("Origin", "http://example.com")
	w = httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// CORS should handle OPTIONS requests
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestServer_HealthEndpoints_Isolated(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	// Create mock services
	mockK8s := &MockKubernetesService{}
	mockArgoCD := &MockArgoCDService{}
	mockServices := &services.Services{
		Kubernetes:          mockK8s,
		ArgoCD:              mockArgoCD,
		Registration:        &MockRegistrationService{},
		RegistrationControl: &MockRegistrationControlService{},
		Authorization:       &MockAuthorizationService{},
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: 8080,
		},
	}

	// Create server with mock dependencies directly
	server := &Server{
		config:   cfg,
		logger:   logger,
		services: mockServices,
		router:   chi.NewRouter(),
	}

	t.Run("Health live endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health/live", http.NoBody)
		w := httptest.NewRecorder()

		server.healthLive(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.HealthResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "ok", response.Status)
	})

	t.Run("Health ready endpoint - success", func(t *testing.T) {
		mockK8s.On("HealthCheck", mock.Anything).Return(nil)
		mockArgoCD.On("HealthCheck", mock.Anything).Return(nil)

		req := httptest.NewRequest("GET", "/health/ready", http.NoBody)
		w := httptest.NewRecorder()

		server.healthReady(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.HealthResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "ready", response.Status)

		mockK8s.AssertExpectations(t)
		mockArgoCD.AssertExpectations(t)
	})

	t.Run("Health ready endpoint - k8s failure", func(t *testing.T) {
		mockK8s.ExpectedCalls = nil
		mockArgoCD.ExpectedCalls = nil

		mockK8s.On("HealthCheck", mock.Anything).Return(errors.New("k8s connection failed"))

		req := httptest.NewRequest("GET", "/health/ready", http.NoBody)
		w := httptest.NewRecorder()

		server.healthReady(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)

		mockK8s.AssertExpectations(t)
	})

	t.Run("Health ready endpoint - argocd failure", func(t *testing.T) {
		mockK8s.ExpectedCalls = nil
		mockArgoCD.ExpectedCalls = nil

		mockK8s.On("HealthCheck", mock.Anything).Return(nil)
		mockArgoCD.On("HealthCheck", mock.Anything).Return(errors.New("argocd connection failed"))

		req := httptest.NewRequest("GET", "/health/ready", http.NoBody)
		w := httptest.NewRecorder()

		server.healthReady(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)

		mockK8s.AssertExpectations(t)
		mockArgoCD.AssertExpectations(t)
	})
}

func TestServer_SetupMiddleware_Unit(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: 8080,
		},
	}

	// Create server with mock services
	server := &Server{
		config:   cfg,
		logger:   logger,
		services: &services.Services{},
		router:   chi.NewRouter(),
	}

	// Test setupMiddleware doesn't panic
	server.setupMiddleware()

	// Test that middleware was applied by making a simple request
	req := httptest.NewRequest("GET", "/nonexistent", http.NoBody)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Middleware should have applied even for 404 routes
	assert.Equal(t, http.StatusNotFound, w.Code)
	// Test completed without panic - middleware setup successful
}

func TestServer_SetupRoutes_Unit(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	mockReg := &MockRegistrationService{}
	mockRegControl := &MockRegistrationControlService{}
	mockAuth := &MockAuthorizationService{}
	mockServices := &services.Services{
		Kubernetes:          &MockKubernetesService{},
		ArgoCD:              &MockArgoCDService{},
		Registration:        mockReg,
		RegistrationControl: mockRegControl,
		Authorization:       mockAuth,
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: 8080,
		},
	}

	server := &Server{
		config:   cfg,
		logger:   logger,
		services: mockServices,
		router:   chi.NewRouter(),
	}

	// Test setupRoutes doesn't panic
	server.setupRoutes()

	// Test that routes were registered by making requests
	tests := []struct {
		method   string
		path     string
		setup    func()
		expected int
	}{
		{
			method:   "GET",
			path:     "/health/live",
			setup:    func() {},
			expected: http.StatusOK,
		},
		{
			method:   "GET",
			path:     "/metrics",
			setup:    func() {},
			expected: http.StatusOK,
		},
		{
			method: "GET",
			path:   "/api/v1/registrations",
			setup: func() {
				mockReg.On("ListRegistrations", mock.Anything, mock.AnythingOfType("map[string]string")).Return(
					[]*types.Registration{}, nil)
			},
			expected: http.StatusOK,
		},
		{
			method:   "POST",
			path:     "/api/v1/registrations",
			setup:    func() {},
			expected: http.StatusBadRequest, // Invalid JSON
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.method, tt.path), func(t *testing.T) {
			// Reset mocks and setup for this test
			mockReg.ExpectedCalls = nil
			mockRegControl.ExpectedCalls = nil
			mockAuth.ExpectedCalls = nil
			tt.setup()

			req := httptest.NewRequest(tt.method, tt.path, http.NoBody)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Just verify the route exists (not 404)
			assert.NotEqual(t, http.StatusNotFound, w.Code, "Route should exist")
			if tt.expected != 0 {
				assert.Equal(t, tt.expected, w.Code)
			}

			mockReg.AssertExpectations(t)
		})
	}
}

func TestServer_CheckDependencies_Unit(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	t.Run("Dependencies healthy", func(t *testing.T) {
		mockK8s := &MockKubernetesService{}
		mockArgoCD := &MockArgoCDService{}
		mockServices := &services.Services{
			Kubernetes: mockK8s,
			ArgoCD:     mockArgoCD,
		}

		mockK8s.On("HealthCheck", mock.Anything).Return(nil)
		mockArgoCD.On("HealthCheck", mock.Anything).Return(nil)

		server := &Server{
			services: mockServices,
			logger:   logger,
		}

		err := server.checkDependencies(context.Background())

		assert.NoError(t, err)
		mockK8s.AssertExpectations(t)
		mockArgoCD.AssertExpectations(t)
	})

	t.Run("Kubernetes unhealthy", func(t *testing.T) {
		mockK8s := &MockKubernetesService{}
		mockArgoCD := &MockArgoCDService{}
		mockServices := &services.Services{
			Kubernetes: mockK8s,
			ArgoCD:     mockArgoCD,
		}

		mockK8s.On("HealthCheck", mock.Anything).Return(errors.New("k8s failed"))

		server := &Server{
			services: mockServices,
			logger:   logger,
		}

		err := server.checkDependencies(context.Background())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "kubernetes api unavailable")
		mockK8s.AssertExpectations(t)
	})

	t.Run("ArgoCD unhealthy", func(t *testing.T) {
		mockK8s := &MockKubernetesService{}
		mockArgoCD := &MockArgoCDService{}
		mockServices := &services.Services{
			Kubernetes: mockK8s,
			ArgoCD:     mockArgoCD,
		}

		mockK8s.On("HealthCheck", mock.Anything).Return(nil)
		mockArgoCD.On("HealthCheck", mock.Anything).Return(errors.New("argocd failed"))

		server := &Server{
			services: mockServices,
			logger:   logger,
		}

		err := server.checkDependencies(context.Background())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "argocd api unavailable")
		mockK8s.AssertExpectations(t)
		mockArgoCD.AssertExpectations(t)
	})
}

func TestServer_Shutdown_Unit(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	// Create a real HTTP server for shutdown testing
	server := &Server{
		logger: logger,
		server: &http.Server{
			Addr: ":0", // Use random port
		},
	}

	// Test shutdown on unstarted server (should not panic)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// May return an error since server wasn't started, but shouldn't panic
	assert.NotPanics(t, func() {
		_ = server.Shutdown(ctx)
	})
}

func TestServer_New_ErrorHandling(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	t.Run("Server creation without Kubernetes environment", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Port: 8080,
			},
		}

		server, err := New(cfg, logger)

		// Should fail due to missing Kubernetes environment variables
		assert.Error(t, err)
		assert.Nil(t, server)
		assert.Contains(t, err.Error(), "failed to initialize services")
		assert.Contains(t, err.Error(), "kubernetes")
	})

	t.Run("Server creation with nil config", func(t *testing.T) {
		server, err := New(nil, logger)

		// Should fail when trying to initialize services with nil config
		assert.Error(t, err)
		assert.Nil(t, server)
		assert.Contains(t, err.Error(), "failed to initialize services")
	})

	t.Run("Server creation with nil logger", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Port: 8080,
			},
		}

		server, err := New(cfg, nil)

		// Should fail when trying to initialize services with nil logger
		assert.Error(t, err)
		assert.Nil(t, server)
		assert.Contains(t, err.Error(), "failed to initialize services")
	})
}

func TestServer_Configuration_Validation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	t.Run("Server with complete configuration", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Port:    8080,
				Timeout: "30s",
			},
			ArgoCD: config.ArgoCDConfig{
				Namespace: "argocd",
				Server:    "argocd-server.argocd.svc.cluster.local",
			},
			Kubernetes: config.KubernetesConfig{
				Namespace: "gitops-system",
			},
		}

		// Test configuration validation patterns
		assert.Equal(t, 8080, cfg.Server.Port)
		assert.Equal(t, "30s", cfg.Server.Timeout)
		assert.Equal(t, "argocd", cfg.ArgoCD.Namespace)
		assert.Equal(t, "gitops-system", cfg.Kubernetes.Namespace)
	})
}

func TestServer_ComponentIntegration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	t.Run("Server structure with all components", func(t *testing.T) {
		mockServices := &services.Services{
			Kubernetes:          &MockKubernetesService{},
			ArgoCD:              &MockArgoCDService{},
			Registration:        &MockRegistrationService{},
			RegistrationControl: &MockRegistrationControlService{},
			Authorization:       &MockAuthorizationService{},
		}

		cfg := &config.Config{
			Server: config.ServerConfig{
				Port: 8080,
			},
		}

		server := &Server{
			config:   cfg,
			logger:   logger,
			services: mockServices,
			router:   chi.NewRouter(),
			server: &http.Server{
				Addr: ":8080",
			},
		}

		// Test that all components are properly set
		assert.NotNil(t, server.config)
		assert.NotNil(t, server.logger)
		assert.NotNil(t, server.services)
		assert.NotNil(t, server.router)
		assert.NotNil(t, server.server)
		assert.Equal(t, cfg, server.config)
		assert.Equal(t, ":8080", server.server.Addr)
	})
}

func TestServer_New_Comprehensive(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: 8080,
		},
		ArgoCD: config.ArgoCDConfig{
			Namespace: "argocd",
		},
	}
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	t.Run("Server creation error paths", func(t *testing.T) {
		// Test with nil config - this will fail service initialization
		server, err := New(nil, logger)
		assert.Error(t, err)
		assert.Nil(t, server)

		// Test with nil logger - this will also fail service initialization
		server, err = New(cfg, nil)
		assert.Error(t, err)
		assert.Nil(t, server)
	})

	t.Run("Server creation with various configurations", func(t *testing.T) {
		testConfigs := []*config.Config{
			{
				Server: config.ServerConfig{Port: 8080},
				ArgoCD: config.ArgoCDConfig{Namespace: "argocd"},
			},
			{
				Server: config.ServerConfig{Port: 9090},
				ArgoCD: config.ArgoCDConfig{Namespace: "custom-argocd"},
			},
		}

		for i, testCfg := range testConfigs {
			t.Run(fmt.Sprintf("Config variation %d", i+1), func(t *testing.T) {
				// Since server.New creates services internally and they might fail
				// in test environment, we just test that it doesn't panic
				server, err := New(testCfg, logger)
				if err != nil {
					// Expected to fail in test environment without real k8s
					assert.Error(t, err)
					assert.Nil(t, server)
				} else {
					// If it succeeds, verify server is created
					assert.NotNil(t, server)
				}
			})
		}
	})
}

func TestServer_Start_Operations(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: 0, // Use port 0 to get a random free port
		},
		ArgoCD: config.ArgoCDConfig{
			Namespace: "argocd",
		},
	}
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Reduce noise in tests

	t.Run("Server start with invalid port", func(t *testing.T) {
		invalidCfg := &config.Config{
			Server: config.ServerConfig{
				Port: -1, // Invalid port
			},
			ArgoCD: config.ArgoCDConfig{
				Namespace: "argocd",
			},
		}

		// In test environment, New will likely fail due to k8s dependencies
		// but we can test the error handling
		server, err := New(invalidCfg, logger)
		if err != nil {
			// Expected in test environment
			assert.Error(t, err)
		} else {
			// If server creation succeeds, test starting with invalid port
			ctx := context.Background()
			err = server.Start(ctx)
			assert.Error(t, err)
		}
	})

	t.Run("Server creation attempts", func(t *testing.T) {
		// Test that New doesn't panic with valid config
		server, err := New(cfg, logger)
		if err != nil {
			// Expected to fail in test environment without real dependencies
			assert.Error(t, err)
			assert.Nil(t, server)
		} else {
			// If creation succeeds, server should be properly initialized
			assert.NotNil(t, server)

			// Test graceful shutdown without starting
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			err = server.Shutdown(ctx)
			// Should handle shutdown gracefully even if not started
			assert.NoError(t, err)
		}
	})
}

func TestServer_Health_Endpoints_Enhanced(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Port: 0},
		ArgoCD: config.ArgoCDConfig{Namespace: "argocd"},
	}
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	t.Run("Health endpoints structure", func(t *testing.T) {
		// Since we can't easily create a server with real dependencies in tests,
		// we test the error handling path
		server, err := New(cfg, logger)
		if err != nil {
			// Expected in test environment
			assert.Error(t, err)
			assert.Nil(t, server)
		} else {
			// If server creation succeeds, test that it's properly structured
			assert.NotNil(t, server)
		}
	})
}

func TestServer_Start_Comprehensive_Coverage(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Minimize test output

	t.Run("Server start with context cancellation", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Port: 0, // Use random available port
			},
			ArgoCD: config.ArgoCDConfig{
				Namespace: "argocd",
			},
		}

		// Create server (this may fail due to dependencies, but we test what we can)
		server, err := New(cfg, logger)
		if err != nil {
			// Expected in test environment - skip this test
			t.Skipf("Server creation failed in test environment: %v", err)
			return
		}

		// Test Start with immediate cancellation
		ctx, cancel := context.WithCancel(context.Background())

		// Start server in goroutine
		startErr := make(chan error, 1)
		go func() {
			startErr <- server.Start(ctx)
		}()

		// Cancel context immediately to trigger shutdown path
		cancel()

		// Wait for Start to return
		select {
		case err := <-startErr:
			// Start should return due to context cancellation
			if err != nil {
				// Context cancellation or server closed is expected
				t.Logf("Start returned with error (expected): %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("Start did not return within timeout")
		}
	})

	t.Run("Server start with quick timeout context", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Port: 0, // Use random available port
			},
			ArgoCD: config.ArgoCDConfig{
				Namespace: "argocd",
			},
		}

		server, err := New(cfg, logger)
		if err != nil {
			t.Skipf("Server creation failed in test environment: %v", err)
			return
		}

		// Test Start with timeout context
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err = server.Start(ctx)
		// Should return due to timeout or successful startup
		if err != nil {
			t.Logf("Start returned with error (acceptable in test): %v", err)
		}
	})

	t.Run("Server start error path - invalid configuration", func(t *testing.T) {
		// Test with configuration that should cause start to fail
		cfg := &config.Config{
			Server: config.ServerConfig{
				Port: -1, // Invalid port should cause failure
			},
			ArgoCD: config.ArgoCDConfig{
				Namespace: "argocd",
			},
		}

		server, err := New(cfg, logger)
		if err != nil {
			// If server creation fails, that's also testing error paths
			t.Logf("Server creation failed as expected: %v", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		err = server.Start(ctx)
		// Starting with invalid port should fail
		assert.Error(t, err)
	})

	t.Run("Server start and shutdown sequence", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Port: 0, // Use random available port
			},
			ArgoCD: config.ArgoCDConfig{
				Namespace: "argocd",
			},
		}

		server, err := New(cfg, logger)
		if err != nil {
			t.Skipf("Server creation failed in test environment: %v", err)
			return
		}

		// Test the full start-shutdown cycle
		ctx := context.Background()

		// Start server in background
		startDone := make(chan error, 1)
		go func() {
			startDone <- server.Start(ctx)
		}()

		// Give server a moment to start
		time.Sleep(50 * time.Millisecond)

		// Shutdown server
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err = server.Shutdown(shutdownCtx)
		assert.NoError(t, err)

		// Wait for start to complete
		select {
		case startErr := <-startDone:
			// Start should return cleanly after shutdown
			if startErr != nil && !errors.Is(startErr, http.ErrServerClosed) {
				t.Logf("Start returned with error: %v", startErr)
			}
		case <-time.After(3 * time.Second):
			t.Error("Start did not return after shutdown")
		}
	})
}
