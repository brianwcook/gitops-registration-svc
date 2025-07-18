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

	"errors"

	"github.com/go-chi/chi/v5"
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
	args := m.Called(ctx, name, labels)
	return args.Error(0)
}

func (m *MockKubernetesService) CreateNamespaceWithMetadata(
	ctx context.Context, name string, labels, annotations map[string]string,
) error {
	args := m.Called(ctx, name, labels, annotations)
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

func (m *MockKubernetesService) UpdateNamespaceLabels(ctx context.Context,
	name string, labels map[string]string) error {
	args := m.Called(ctx, name, labels)
	return args.Error(0)
}

func (m *MockKubernetesService) UpdateNamespaceMetadata(
	ctx context.Context, name string, labels, annotations map[string]string,
) error {
	args := m.Called(ctx, name, labels, annotations)
	return args.Error(0)
}

func (m *MockKubernetesService) CreateRoleBinding(ctx context.Context,
	namespace, name, role, serviceAccount string) error {
	args := m.Called(ctx, namespace, name, role, serviceAccount)
	return args.Error(0)
}

func (m *MockKubernetesService) CheckAppProjectConflict(ctx context.Context, repositoryHash string) (bool, error) {
	args := m.Called(ctx, repositoryHash)
	return args.Bool(0), args.Error(1)
}

func (m *MockKubernetesService) CreateRoleBindingForServiceAccount(ctx context.Context,
	namespace, name, clusterRole, serviceAccountName string) error {
	args := m.Called(ctx, namespace, name, clusterRole, serviceAccountName)
	return args.Error(0)
}

func (m *MockKubernetesService) CreateServiceAccountWithGenerateName(ctx context.Context,
	namespace, baseName string) (string, error) {
	args := m.Called(ctx, namespace, baseName)
	return args.String(0), args.Error(1)
}

func (m *MockKubernetesService) ValidateClusterRole(ctx context.Context,
	name string) (*services.ClusterRoleValidation, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.ClusterRoleValidation), args.Error(1)
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
	args := m.Called(ctx, repositoryHash)
	return args.Bool(0), args.Error(1)
}

type MockRegistrationService struct {
	mock.Mock
}

func (m *MockRegistrationService) CreateRegistration(
	ctx context.Context,
	req *types.RegistrationRequest,
) (*types.Registration, error) {
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

func (m *MockRegistrationService) ListRegistrations(
	ctx context.Context,
	filters map[string]string,
) ([]*types.Registration, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]*types.Registration), args.Error(1)
}

func (m *MockRegistrationService) DeleteRegistration(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRegistrationService) RegisterExistingNamespace(
	ctx context.Context,
	req *types.ExistingNamespaceRequest,
	userInfo *types.UserInfo,
) (*types.Registration, error) {
	args := m.Called(ctx, req, userInfo)
	return args.Get(0).(*types.Registration), args.Error(1)
}

func (m *MockRegistrationService) ValidateRegistration(ctx context.Context, req *types.RegistrationRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *MockRegistrationService) ValidateExistingNamespaceRequest(
	ctx context.Context,
	req *types.ExistingNamespaceRequest,
) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

type MockRegistrationControlService struct {
	mock.Mock
}

func (m *MockRegistrationControlService) GetRegistrationStatus(
	ctx context.Context,
) (*types.ServiceRegistrationStatus, error) {
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

func (m *MockAuthorizationService) ValidateNamespaceAccess(
	ctx context.Context, userInfo *types.UserInfo, namespace string,
) error {
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

// TestMocks groups all mock services for easier test setup
type TestMocks struct {
	Kubernetes          *MockKubernetesService
	ArgoCD              *MockArgoCDService
	Registration        *MockRegistrationService
	RegistrationControl *MockRegistrationControlService
	Authorization       *MockAuthorizationService
}

func setupTestHandler() (*RegistrationHandler, *TestMocks) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Suppress logs during tests

	// Create mock services
	mocks := &TestMocks{
		Kubernetes:          &MockKubernetesService{},
		ArgoCD:              &MockArgoCDService{},
		Registration:        &MockRegistrationService{},
		RegistrationControl: &MockRegistrationControlService{},
		Authorization:       &MockAuthorizationService{},
	}

	// Create services struct with mocks
	mockServices := &services.Services{
		Kubernetes:          mocks.Kubernetes,
		ArgoCD:              mocks.ArgoCD,
		Registration:        mocks.Registration,
		RegistrationControl: mocks.RegistrationControl,
		Authorization:       mocks.Authorization,
	}

	handler := NewRegistrationHandler(mockServices, logger)
	return handler, mocks
}

func TestRegistrationHandler_CreateRegistration_Success(t *testing.T) {
	handler, mocks := setupTestHandler()

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

	mocks.Authorization.On("ExtractUserInfo", mock.Anything, "valid-token").Return(userInfo, nil)
	mocks.Registration.On("ValidateRegistration", mock.Anything,
		mock.AnythingOfType("*types.RegistrationRequest")).Return(nil)
	mocks.RegistrationControl.On("IsNewNamespaceAllowed", mock.Anything).Return(nil)
	mocks.Registration.On("CreateRegistration", mock.Anything,
		mock.AnythingOfType("*types.RegistrationRequest")).Return(registration, nil)

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

	mocks.Authorization.AssertExpectations(t)
	mocks.RegistrationControl.AssertExpectations(t)
	mocks.Registration.AssertExpectations(t)
}

func TestRegistrationHandler_CreateRegistration_InvalidJSON(t *testing.T) {
	handler, _ := setupTestHandler()

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
	handler, mocks := setupTestHandler()

	mocks.Authorization.On("ExtractUserInfo", mock.Anything, "").Return((*types.UserInfo)(nil), fmt.Errorf("no token"))
	mocks.Registration.On("ValidateRegistration", mock.Anything, mock.AnythingOfType("*types.RegistrationRequest")).Return(nil)
	mocks.RegistrationControl.On("IsNewNamespaceAllowed", mock.Anything).Return(nil)

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
	handler, mocks := setupTestHandler()

	userInfo := &types.UserInfo{
		Username: "test-user",
		Email:    "test@example.com",
	}

	mocks.Authorization.On("ExtractUserInfo", mock.Anything, "valid-token").Return(userInfo, nil)
	mocks.Registration.On("ValidateRegistration", mock.Anything,
		mock.AnythingOfType("*types.RegistrationRequest")).Return(nil)
	regControlErr := fmt.Errorf("new namespace registration is currently disabled")
	mocks.RegistrationControl.On("IsNewNamespaceAllowed", mock.Anything).Return(regControlErr)

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

	assert.Equal(t, http.StatusForbidden, w.Code)

	var response types.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "REGISTRATION_DISABLED", response.Error)
	assert.Contains(t, response.Message, "new namespace registration is currently disabled")

	mocks.Authorization.AssertExpectations(t)
	mocks.RegistrationControl.AssertExpectations(t)
}

func TestRegistrationHandler_RegisterExistingNamespace_Success(t *testing.T) {
	handler, mocks := setupTestHandler()

	userInfo := &types.UserInfo{Username: "test-user"}
	registration := &types.Registration{
		ID:        "test-existing-reg-123",
		Namespace: "existing-namespace",
		Status: types.RegistrationStatus{
			Phase: "active",
		},
	}

	mocks.Authorization.On("ExtractUserInfo", mock.Anything, "valid-token").Return(userInfo, nil)
	mocks.Registration.On("ValidateExistingNamespaceRequest", mock.Anything,
		mock.AnythingOfType("*types.ExistingNamespaceRequest")).Return(nil)
	mocks.Authorization.On("ValidateNamespaceAccess", mock.Anything, userInfo, "existing-namespace").Return(nil)
	mocks.Registration.On("RegisterExistingNamespace", mock.Anything,
		mock.AnythingOfType("*types.ExistingNamespaceRequest"), userInfo).Return(registration, nil)

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

	mocks.Authorization.AssertExpectations(t)
	mocks.Registration.AssertExpectations(t)
}

func TestRegistrationHandler_RegisterExistingNamespace_InsufficientPermissions(t *testing.T) {
	handler, mocks := setupTestHandler()

	userInfo := &types.UserInfo{Username: "test-user"}

	mocks.Authorization.On("ExtractUserInfo", mock.Anything, "valid-token").Return(userInfo, nil)
	mocks.Registration.On("ValidateExistingNamespaceRequest", mock.Anything,
		mock.AnythingOfType("*types.ExistingNamespaceRequest")).Return(nil)
	insufficientErr := fmt.Errorf("insufficient permissions")
	mocks.Authorization.On("ValidateNamespaceAccess", mock.Anything, userInfo, "existing-namespace").Return(insufficientErr)

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
	handler, mocks := setupTestHandler()

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

	mocks.Registration.On("ListRegistrations", mock.Anything,
		mock.AnythingOfType("map[string]string")).Return(registrations, nil)

	req := httptest.NewRequest("GET", "/api/v1/registrations", http.NoBody)
	w := httptest.NewRecorder()
	handler.ListRegistrations(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []*types.Registration
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Len(t, response, 2)
	assert.Equal(t, "reg-1", response[0].ID)

	mocks.Registration.AssertExpectations(t)
}

func TestRegistrationHandler_GetRegistration_Success(t *testing.T) {
	handler, mocks := setupTestHandler()

	registration := &types.Registration{
		ID:        "test-reg-123",
		Namespace: "test-namespace",
		CreatedAt: time.Now(),
	}

	mocks.Registration.On("GetRegistration", mock.Anything, "test-reg-123").Return(registration, nil)

	// Create request with chi context
	req := httptest.NewRequest("GET", "/api/v1/registrations/test-reg-123", http.NoBody)

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

	mocks.Registration.AssertExpectations(t)
}

func TestRegistrationHandler_GetRegistration_NotFound(t *testing.T) {
	handler, mocks := setupTestHandler()

	notFoundErr := fmt.Errorf("not found")
	mocks.Registration.On("GetRegistration", mock.Anything, "non-existent").Return((*types.Registration)(nil), notFoundErr)

	req := httptest.NewRequest("GET", "/api/v1/registrations/non-existent", http.NoBody)

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
	handler, mocks := setupTestHandler()

	mocks.Registration.On("DeleteRegistration", mock.Anything, "test-reg-123").Return(nil)

	req := httptest.NewRequest("DELETE", "/api/v1/registrations/test-reg-123", http.NoBody)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "test-reg-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.DeleteRegistration(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())

	mocks.Registration.AssertExpectations(t)
}

// Test helper functions
func TestExtractUserInfo_Success(t *testing.T) {
	handler, mocks := setupTestHandler()

	userInfo := &types.UserInfo{Username: "test-user"}
	mocks.Authorization.On("ExtractUserInfo", mock.Anything, "valid-token").Return(userInfo, nil)

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer valid-token")

	extractedUserInfo, err := handler.extractUserInfo(req)
	require.NoError(t, err)
	assert.Equal(t, "test-user", extractedUserInfo.Username)

	mocks.Authorization.AssertExpectations(t)
}

func TestExtractUserInfo_NoAuthHeader(t *testing.T) {
	handler, _ := setupTestHandler()

	req := httptest.NewRequest("GET", "/test", http.NoBody)

	_, err := handler.extractUserInfo(req)
	assert.Error(t, err)
}

func TestRegistrationHandler_ErrorPaths(t *testing.T) {
	handler, mocks := setupTestHandler()

	t.Run("isNamespaceConflictError returns true for NamespaceConflictError", func(t *testing.T) {
		err := &services.NamespaceConflictError{Namespace: "test"}
		result := isNamespaceConflictError(err)
		assert.True(t, result)
	})

	t.Run("isNamespaceConflictError returns false for other errors", func(t *testing.T) {
		err := errors.New("some other error")
		result := isNamespaceConflictError(err)
		assert.False(t, result)
	})

	t.Run("isRepositoryConflictError returns true for repository conflict", func(t *testing.T) {
		err := errors.New("repository https://github.com/test/repo is already registered")
		result := isRepositoryConflictError(err)
		assert.True(t, result)
	})

	t.Run("isRepositoryConflictError returns false for other errors", func(t *testing.T) {
		err := errors.New("some other error")
		result := isRepositoryConflictError(err)
		assert.False(t, result)
	})

	t.Run("GetRegistrationStatus endpoint", func(t *testing.T) {
		expectedRegistration := &types.Registration{
			ID:        "test-reg-123",
			Namespace: "test-namespace",
			Status: types.RegistrationStatus{
				Phase:   "active",
				Message: "Registration completed successfully",
			},
		}
		mocks.Registration.On("GetRegistration", mock.Anything, "test-reg-123").Return(expectedRegistration, nil)

		req := httptest.NewRequest("GET", "/api/v1/registrations/test-reg-123/status", http.NoBody)

		// Add chi URL parameters
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "test-reg-123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetRegistrationStatus(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.RegistrationStatus
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, expectedRegistration.Status.Phase, response.Phase)

		mocks.Registration.AssertExpectations(t)
	})

	t.Run("GetRegistrationStatus error", func(t *testing.T) {
		mocks.Registration.ExpectedCalls = nil
		mocks.Registration.On("GetRegistration", mock.Anything, "not-found").Return(
			(*types.Registration)(nil), errors.New("registration not found"))

		req := httptest.NewRequest("GET", "/api/v1/registrations/not-found/status", http.NoBody)

		// Add chi URL parameters
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "not-found")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetRegistrationStatus(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mocks.Registration.AssertExpectations(t)
	})

	t.Run("SyncRegistration endpoint", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/registrations/test-reg/sync", http.NoBody)

		// Add chi URL parameters
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "test-reg")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.SyncRegistration(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["message"], "Sync triggered successfully")
		assert.Equal(t, "test-reg", response["id"])
	})
}

func TestRegistrationHandler_CreateRegistration_ValidationErrors(t *testing.T) {
	handler, mocks := setupTestHandler()

	userInfo := &types.UserInfo{Username: "test-user"}

	t.Run("Validation error", func(t *testing.T) {
		mocks.Authorization.On("ExtractUserInfo", mock.Anything, "valid-token").Return(userInfo, nil)
		mocks.Registration.On("ValidateRegistration", mock.Anything,
			mock.AnythingOfType("*types.RegistrationRequest")).Return(errors.New("validation failed"))

		reqBody := types.RegistrationRequest{
			Namespace: "test-namespace",
			Repository: types.Repository{
				URL:    "https://github.com/test/repo",
				Branch: "main",
			},
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/v1/registrations", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.CreateRegistration(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mocks.Authorization.AssertExpectations(t)
		mocks.Registration.AssertExpectations(t)
	})

	t.Run("Repository conflict error", func(t *testing.T) {
		mocks.Authorization.ExpectedCalls = nil
		mocks.Registration.ExpectedCalls = nil
		mocks.RegistrationControl.ExpectedCalls = nil

		mocks.Authorization.On("ExtractUserInfo", mock.Anything, "valid-token").Return(userInfo, nil)
		mocks.Registration.On("ValidateRegistration", mock.Anything,
			mock.AnythingOfType("*types.RegistrationRequest")).Return(nil)
		mocks.RegistrationControl.On("IsNewNamespaceAllowed", mock.Anything).Return(nil)
		repoErr := errors.New("repository https://github.com/test/repo is already registered in another AppProject")
		mocks.Registration.On("CreateRegistration", mock.Anything,
			mock.AnythingOfType("*types.RegistrationRequest")).Return((*types.Registration)(nil), repoErr)

		reqBody := types.RegistrationRequest{
			Namespace: "test-namespace",
			Repository: types.Repository{
				URL:    "https://github.com/test/repo",
				Branch: "main",
			},
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/v1/registrations", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.CreateRegistration(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		var response types.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "REPOSITORY_CONFLICT", response.Error)

		mocks.Authorization.AssertExpectations(t)
		mocks.Registration.AssertExpectations(t)
		mocks.RegistrationControl.AssertExpectations(t)
	})

	t.Run("Namespace conflict error", func(t *testing.T) {
		mocks.Authorization.ExpectedCalls = nil
		mocks.Registration.ExpectedCalls = nil
		mocks.RegistrationControl.ExpectedCalls = nil

		mocks.Authorization.On("ExtractUserInfo", mock.Anything, "valid-token").Return(userInfo, nil)
		mocks.Registration.On("ValidateRegistration", mock.Anything,
			mock.AnythingOfType("*types.RegistrationRequest")).Return(nil)
		mocks.RegistrationControl.On("IsNewNamespaceAllowed", mock.Anything).Return(nil)
		namespaceErr := &services.NamespaceConflictError{Namespace: "existing-namespace"}
		mocks.Registration.On("CreateRegistration", mock.Anything,
			mock.AnythingOfType("*types.RegistrationRequest")).Return((*types.Registration)(nil), namespaceErr)

		reqBody := types.RegistrationRequest{
			Namespace: "existing-namespace",
			Repository: types.Repository{
				URL:    "https://github.com/test/repo",
				Branch: "main",
			},
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/v1/registrations", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.CreateRegistration(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		var response types.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "NAMESPACE_CONFLICT", response.Error)

		mocks.Authorization.AssertExpectations(t)
		mocks.Registration.AssertExpectations(t)
		mocks.RegistrationControl.AssertExpectations(t)
	})
}

func TestRegistrationHandler_RegisterExistingNamespace_ValidationErrors(t *testing.T) {
	handler, mocks := setupTestHandler()

	t.Run("Validation error", func(t *testing.T) {
		mocks.Registration.On("ValidateExistingNamespaceRequest", mock.Anything,
			mock.AnythingOfType("*types.ExistingNamespaceRequest")).Return(errors.New("validation failed"))

		reqBody := types.ExistingNamespaceRequest{
			ExistingNamespace: "existing-namespace",
			Repository: types.Repository{
				URL:    "https://github.com/test/repo",
				Branch: "main",
			},
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/v1/registrations/existing", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RegisterExistingNamespace(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		mocks.Registration.AssertExpectations(t)
	})

	t.Run("Authentication error", func(t *testing.T) {
		mocks.Registration.ExpectedCalls = nil
		mocks.Authorization.ExpectedCalls = nil

		mocks.Registration.On("ValidateExistingNamespaceRequest", mock.Anything,
			mock.AnythingOfType("*types.ExistingNamespaceRequest")).Return(nil)
		mocks.Authorization.On("ExtractUserInfo", mock.Anything, "invalid-token").Return(
			(*types.UserInfo)(nil), errors.New("invalid token"))

		reqBody := types.ExistingNamespaceRequest{
			ExistingNamespace: "existing-namespace",
			Repository: types.Repository{
				URL:    "https://github.com/test/repo",
				Branch: "main",
			},
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/v1/registrations/existing", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer invalid-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RegisterExistingNamespace(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		mocks.Registration.AssertExpectations(t)
		mocks.Authorization.AssertExpectations(t)
	})

	t.Run("Invalid JSON in request", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/registrations/existing", bytes.NewBufferString("invalid-json"))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RegisterExistingNamespace(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestRegistrationHandler_ComprehensiveEdgeCases(t *testing.T) {
	handler, mocks := setupTestHandler()

	t.Run("CreateRegistration - Missing ID in URL param", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/registrations//status", http.NoBody)

		// Add empty ID parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetRegistrationStatus(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response types.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_REQUEST", response.Error)
	})

	t.Run("SyncRegistration - Missing ID in URL param", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/registrations//sync", http.NoBody)

		// Add empty ID parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.SyncRegistration(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response types.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_REQUEST", response.Error)
	})

	t.Run("CreateRegistration - Authorization extraction failure", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/registrations", bytes.NewBufferString("{}"))
		// No Authorization header
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.CreateRegistration(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var response types.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "AUTHENTICATION_REQUIRED", response.Error)
	})

	t.Run("CreateRegistration - Invalid Bearer token format", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/registrations", bytes.NewBufferString("{}"))
		req.Header.Set("Authorization", "Invalid token-format")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.CreateRegistration(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("RegisterExistingNamespace - Insufficient permissions", func(t *testing.T) {
		userInfo := &types.UserInfo{Username: "test-user"}
		mocks.Registration.On("ValidateExistingNamespaceRequest", mock.Anything,
			mock.AnythingOfType("*types.ExistingNamespaceRequest")).Return(nil)
		mocks.Authorization.On("ExtractUserInfo", mock.Anything, "valid-token").Return(userInfo, nil)
		mocks.Authorization.On("ValidateNamespaceAccess", mock.Anything, userInfo, "restricted-namespace").Return(
			errors.New("insufficient permissions"))

		reqBody := types.ExistingNamespaceRequest{
			ExistingNamespace: "restricted-namespace",
			Repository: types.Repository{
				URL:    "https://github.com/test/repo",
				Branch: "main",
			},
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/v1/registrations/existing", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RegisterExistingNamespace(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		var response types.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "INSUFFICIENT_PERMISSIONS", response.Error)

		mocks.Registration.AssertExpectations(t)
		mocks.Authorization.AssertExpectations(t)
	})

	t.Run("RegisterExistingNamespace - Registration failure", func(t *testing.T) {
		mocks.Registration.ExpectedCalls = nil
		mocks.Authorization.ExpectedCalls = nil

		userInfo := &types.UserInfo{Username: "test-user"}
		mocks.Registration.On("ValidateExistingNamespaceRequest", mock.Anything,
			mock.AnythingOfType("*types.ExistingNamespaceRequest")).Return(nil)
		mocks.Authorization.On("ExtractUserInfo", mock.Anything, "valid-token").Return(userInfo, nil)
		mocks.Authorization.On("ValidateNamespaceAccess", mock.Anything, userInfo, "test-namespace").Return(nil)
		mocks.Registration.On("RegisterExistingNamespace", mock.Anything,
			mock.AnythingOfType("*types.ExistingNamespaceRequest"), userInfo).Return(
			(*types.Registration)(nil), errors.New("registration failed"))

		reqBody := types.ExistingNamespaceRequest{
			ExistingNamespace: "test-namespace",
			Repository: types.Repository{
				URL:    "https://github.com/test/repo",
				Branch: "main",
			},
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/v1/registrations/existing", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.RegisterExistingNamespace(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var response types.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "REGISTRATION_FAILED", response.Error)

		mocks.Registration.AssertExpectations(t)
		mocks.Authorization.AssertExpectations(t)
	})

	t.Run("DeleteRegistration - Missing ID parameter", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/registrations/", http.NoBody)

		// Add empty ID parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.DeleteRegistration(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GetRegistration - Missing ID parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/registrations/", http.NoBody)

		// Add empty ID parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		handler.GetRegistration(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ListRegistrations - Error case", func(t *testing.T) {
		mocks.Registration.ExpectedCalls = nil
		mocks.Registration.On("ListRegistrations", mock.Anything, mock.AnythingOfType("map[string]string")).Return(
			([]*types.Registration)(nil), errors.New("database error"))

		req := httptest.NewRequest("GET", "/api/v1/registrations", http.NoBody)
		w := httptest.NewRecorder()

		handler.ListRegistrations(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mocks.Registration.AssertExpectations(t)
	})

	t.Run("ExtractUserInfo - Invalid token format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("Authorization", "NotBearer token")

		userInfo, err := handler.extractUserInfo(req)

		assert.Error(t, err)
		assert.Nil(t, userInfo)
		assert.Equal(t, http.ErrNoCookie, err)
	})

	t.Run("ExtractUserInfo - Service error", func(t *testing.T) {
		mocks.Authorization.ExpectedCalls = nil
		mocks.Authorization.On("ExtractUserInfo", mock.Anything, "test-token").Return(
			(*types.UserInfo)(nil), errors.New("service error"))

		req := httptest.NewRequest("GET", "/test", http.NoBody)
		req.Header.Set("Authorization", "Bearer test-token")

		userInfo, err := handler.extractUserInfo(req)

		assert.Error(t, err)
		assert.Nil(t, userInfo)
		mocks.Authorization.AssertExpectations(t)
	})
}
