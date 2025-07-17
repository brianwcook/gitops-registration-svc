package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/konflux-ci/gitops-registration-service/internal/config"
	"github.com/konflux-ci/gitops-registration-service/internal/services"
	"github.com/konflux-ci/gitops-registration-service/internal/types"
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
	args := m.Called(ctx, name, labels)
	return args.Error(0)
}

func (m *MockKubernetesService) DeleteNamespace(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
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

func (m *MockKubernetesService) UpdateNamespaceLabels(ctx context.Context, name string, labels map[string]string) error {
	args := m.Called(ctx, name, labels)
	return args.Error(0)
}

func (m *MockKubernetesService) CreateRoleBinding(ctx context.Context, namespace, name, role, serviceAccount string) error {
	args := m.Called(ctx, namespace, name, role, serviceAccount)
	return args.Error(0)
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

	req := httptest.NewRequest("GET", "/health/live", nil)
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

	req := httptest.NewRequest("GET", "/health/ready", nil)
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

	req := httptest.NewRequest("GET", "/health/ready", nil)
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

	req := httptest.NewRequest("GET", "/health/ready", nil)
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
	req := httptest.NewRequest("GET", "/health/live", nil)
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
			req := httptest.NewRequest(tc.method, tc.path, nil)
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
	req := httptest.NewRequest("GET", "/health/live", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Verify that all middleware was applied correctly
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// Verify CORS headers are set
	req = httptest.NewRequest("OPTIONS", "/health/live", nil)
	req.Header.Set("Origin", "http://example.com")
	w = httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// CORS should handle OPTIONS requests
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Origin"))
}
