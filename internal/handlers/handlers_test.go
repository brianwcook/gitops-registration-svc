package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

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

// setupTestHandler creates a handler with mocked services
func setupTestHandler() (*RegistrationHandler, *MockKubernetesService, *MockArgoCDService, *MockRegistrationService, *MockRegistrationControlService, *MockAuthorizationService) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Suppress logs during tests

	// Create mock services
	mockK8s := &MockKubernetesService{}
	mockArgoCD := &MockArgoCDService{}
	mockRegistration := &MockRegistrationService{}
	mockRegistrationControl := &MockRegistrationControlService{}
	mockAuth := &MockAuthorizationService{}

	// Create services struct with mocks
	mockServices := &services.Services{
		Kubernetes:          mockK8s,
		ArgoCD:              mockArgoCD,
		Registration:        mockRegistration,
		RegistrationControl: mockRegistrationControl,
		Authorization:       mockAuth,
	}

	handler := NewRegistrationHandler(mockServices, logger)
	return handler, mockK8s, mockArgoCD, mockRegistration, mockRegistrationControl, mockAuth
}

func TestRegistrationHandler_CreateRegistration_Success(t *testing.T) {
	handler, _, _, mockRegistration, mockRegControl, mockAuth := setupTestHandler()

	// Setup mocks
	userInfo := &types.UserInfo{
		Username: "test-user",
		Email:    "test@example.com",
	}
	registration := &types.Registration{
		ID:        "test-reg-123",
		Namespace: "test-namespace",
		Status: types.RegistrationStatus{
			Phase: "pending",
		},
	}

	mockAuth.On("ExtractUserInfo", mock.Anything, "valid-token").Return(userInfo, nil)
	mockRegistration.On("ValidateRegistration", mock.Anything, mock.AnythingOfType("*types.RegistrationRequest")).Return(nil)
	mockRegControl.On("IsNewNamespaceAllowed", mock.Anything).Return(nil)
	mockRegistration.On("CreateRegistration", mock.Anything, mock.AnythingOfType("*types.RegistrationRequest")).Return(registration, nil)

	// Create request
	reqBody := types.RegistrationRequest{
		Repository: types.Repository{
			URL:    "https://github.com/test/repo",
			Branch: "main",
		},
		Namespace: "test-namespace",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/registrations", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")

	w := httptest.NewRecorder()
	handler.CreateRegistration(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response types.Registration
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "test-reg-123", response.ID)

	mockAuth.AssertExpectations(t)
	mockRegControl.AssertExpectations(t)
	mockRegistration.AssertExpectations(t)
}

func TestRegistrationHandler_CreateRegistration_InvalidJSON(t *testing.T) {
	handler, _, _, _, _, _ := setupTestHandler()

	req := httptest.NewRequest("POST", "/api/v1/registrations", bytes.NewBufferString("invalid-json"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.CreateRegistration(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response types.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_REQUEST", response.Error)
}

func TestRegistrationHandler_CreateRegistration_NoAuth(t *testing.T) {
	handler, _, _, mockRegistration, mockRegControl, mockAuth := setupTestHandler()

	mockAuth.On("ExtractUserInfo", mock.Anything, "").Return((*types.UserInfo)(nil), fmt.Errorf("no token"))
	mockRegistration.On("ValidateRegistration", mock.Anything, mock.AnythingOfType("*types.RegistrationRequest")).Return(nil)
	mockRegControl.On("IsNewNamespaceAllowed", mock.Anything).Return(nil)

	reqBody := types.RegistrationRequest{
		Repository: types.Repository{URL: "https://github.com/test/repo"},
		Namespace:  "test-namespace",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/registrations", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.CreateRegistration(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRegistrationHandler_CreateRegistration_RegistrationDisabled(t *testing.T) {
	handler, _, _, mockRegistration, mockRegControl, mockAuth := setupTestHandler()

	userInfo := &types.UserInfo{
		Username: "test-user",
		Email:    "test@example.com",
	}

	mockAuth.On("ExtractUserInfo", mock.Anything, "valid-token").Return(userInfo, nil)
	mockRegistration.On("ValidateRegistration", mock.Anything, mock.AnythingOfType("*types.RegistrationRequest")).Return(nil)
	mockRegControl.On("IsNewNamespaceAllowed", mock.Anything).Return(fmt.Errorf("new namespace registration is currently disabled"))

	// Create request
	reqBody := types.RegistrationRequest{
		Repository: types.Repository{
			URL:    "https://github.com/test/repo",
			Branch: "main",
		},
		Namespace: "test-namespace",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/registrations", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")

	w := httptest.NewRecorder()
	handler.CreateRegistration(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response types.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "REGISTRATION_DISABLED", response.Error)
	assert.Contains(t, response.Message, "new namespace registration is currently disabled")

	mockAuth.AssertExpectations(t)
	mockRegControl.AssertExpectations(t)
}

func TestRegistrationHandler_RegisterExistingNamespace_Success(t *testing.T) {
	handler, _, _, mockRegistration, _, mockAuth := setupTestHandler()

	userInfo := &types.UserInfo{Username: "test-user"}
	registration := &types.Registration{
		ID:        "test-existing-reg-123",
		Namespace: "existing-namespace",
		Status: types.RegistrationStatus{
			Phase: "active",
		},
	}

	mockAuth.On("ExtractUserInfo", mock.Anything, "valid-token").Return(userInfo, nil)
	mockRegistration.On("ValidateExistingNamespaceRequest", mock.Anything, mock.AnythingOfType("*types.ExistingNamespaceRequest")).Return(nil)
	mockAuth.On("ValidateNamespaceAccess", mock.Anything, userInfo, "existing-namespace").Return(nil)
	mockRegistration.On("RegisterExistingNamespace", mock.Anything, mock.AnythingOfType("*types.ExistingNamespaceRequest"), userInfo).Return(registration, nil)

	reqBody := types.ExistingNamespaceRequest{
		Repository: types.Repository{
			URL:    "https://github.com/test/existing-repo",
			Branch: "main",
		},
		ExistingNamespace: "existing-namespace",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/registrations/existing", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")

	w := httptest.NewRecorder()
	handler.RegisterExistingNamespace(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response types.Registration
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "test-existing-reg-123", response.ID)

	mockAuth.AssertExpectations(t)
	mockRegistration.AssertExpectations(t)
}

func TestRegistrationHandler_RegisterExistingNamespace_InsufficientPermissions(t *testing.T) {
	handler, _, _, mockRegistration, _, mockAuth := setupTestHandler()

	userInfo := &types.UserInfo{Username: "test-user"}

	mockAuth.On("ExtractUserInfo", mock.Anything, "valid-token").Return(userInfo, nil)
	mockRegistration.On("ValidateExistingNamespaceRequest", mock.Anything, mock.AnythingOfType("*types.ExistingNamespaceRequest")).Return(nil)
	mockAuth.On("ValidateNamespaceAccess", mock.Anything, userInfo, "existing-namespace").Return(fmt.Errorf("insufficient permissions"))

	reqBody := types.ExistingNamespaceRequest{
		Repository:        types.Repository{URL: "https://github.com/test/repo"},
		ExistingNamespace: "existing-namespace",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/registrations/existing", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")

	w := httptest.NewRecorder()
	handler.RegisterExistingNamespace(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var response types.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "INSUFFICIENT_PERMISSIONS", response.Error)
}

func TestRegistrationHandler_ListRegistrations_Success(t *testing.T) {
	handler, _, _, mockRegistration, _, _ := setupTestHandler()

	registrations := []*types.Registration{
		{
			ID:        "reg-1",
			Namespace: "namespace-1",
		},
		{
			ID:        "reg-2",
			Namespace: "namespace-2",
		},
	}

	mockRegistration.On("ListRegistrations", mock.Anything, mock.AnythingOfType("map[string]string")).Return(registrations, nil)

	req := httptest.NewRequest("GET", "/api/v1/registrations", nil)
	w := httptest.NewRecorder()
	handler.ListRegistrations(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []*types.Registration
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Len(t, response, 2)
	assert.Equal(t, "reg-1", response[0].ID)

	mockRegistration.AssertExpectations(t)
}

func TestRegistrationHandler_GetRegistration_Success(t *testing.T) {
	handler, _, _, mockRegistration, _, _ := setupTestHandler()

	registration := &types.Registration{
		ID:        "test-reg-123",
		Namespace: "test-namespace",
		CreatedAt: time.Now(),
	}

	mockRegistration.On("GetRegistration", mock.Anything, "test-reg-123").Return(registration, nil)

	// Create request with chi context
	req := httptest.NewRequest("GET", "/api/v1/registrations/test-reg-123", nil)

	// Add chi URL parameters
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "test-reg-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.GetRegistration(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response types.Registration
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "test-reg-123", response.ID)

	mockRegistration.AssertExpectations(t)
}

func TestRegistrationHandler_GetRegistration_NotFound(t *testing.T) {
	handler, _, _, mockRegistration, _, _ := setupTestHandler()

	mockRegistration.On("GetRegistration", mock.Anything, "non-existent").Return((*types.Registration)(nil), fmt.Errorf("not found"))

	req := httptest.NewRequest("GET", "/api/v1/registrations/non-existent", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "non-existent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.GetRegistration(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response types.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "NOT_FOUND", response.Error)
}

func TestRegistrationHandler_DeleteRegistration_Success(t *testing.T) {
	handler, _, _, mockRegistration, _, _ := setupTestHandler()

	mockRegistration.On("DeleteRegistration", mock.Anything, "test-reg-123").Return(nil)

	req := httptest.NewRequest("DELETE", "/api/v1/registrations/test-reg-123", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "test-reg-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.DeleteRegistration(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())

	mockRegistration.AssertExpectations(t)
}

// Test helper functions
func TestExtractUserInfo_Success(t *testing.T) {
	handler, _, _, _, _, mockAuth := setupTestHandler()

	userInfo := &types.UserInfo{Username: "test-user"}
	mockAuth.On("ExtractUserInfo", mock.Anything, "valid-token").Return(userInfo, nil)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")

	extractedUserInfo, err := handler.extractUserInfo(req)
	require.NoError(t, err)
	assert.Equal(t, "test-user", extractedUserInfo.Username)

	mockAuth.AssertExpectations(t)
}

func TestExtractUserInfo_NoAuthHeader(t *testing.T) {
	handler, _, _, _, _, _ := setupTestHandler()

	req := httptest.NewRequest("GET", "/test", nil)

	_, err := handler.extractUserInfo(req)
	assert.Error(t, err)
}
