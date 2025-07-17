package types

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistration_JSONSerialization(t *testing.T) {
	registration := &Registration{
		ID:        "test-reg-123",
		Namespace: "test-namespace",
		Repository: Repository{
			URL:    "https://github.com/test/repo",
			Branch: "main",
			Credentials: Credentials{
				Type:      "token",
				SecretRef: "github-token",
			},
		},

		Status: RegistrationStatus{
			Phase:              "active",
			Message:            "Registration is active",
			ArgoCDApplication:  "test-app",
			ArgoCDAppProject:   "test-project",
			NamespaceCreated:   true,
			AppProjectCreated:  true,
			ApplicationCreated: true,
		},
		CreatedAt: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC),
		Labels: map[string]string{
			"app": "gitops-registration",
		},
	}

	// Test marshaling
	data, err := json.Marshal(registration)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled Registration
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Verify fields
	assert.Equal(t, registration.ID, unmarshaled.ID)
	assert.Equal(t, registration.Namespace, unmarshaled.Namespace)
	assert.Equal(t, registration.Repository.URL, unmarshaled.Repository.URL)
	assert.Equal(t, registration.Repository.Branch, unmarshaled.Repository.Branch)
	assert.Equal(t, registration.Status.Phase, unmarshaled.Status.Phase)
	assert.Equal(t, registration.CreatedAt.UTC(), unmarshaled.CreatedAt.UTC())
}

func TestRegistrationRequest_JSONSerialization(t *testing.T) {
	req := &RegistrationRequest{
		Repository: Repository{
			URL:    "https://github.com/test/repo",
			Branch: "main",
		},
		Namespace: "test-namespace",
	}

	// Test marshaling
	data, err := json.Marshal(req)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled RegistrationRequest
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, req.Repository.URL, unmarshaled.Repository.URL)
	assert.Equal(t, req.Namespace, unmarshaled.Namespace)
}

func TestExistingNamespaceRequest_JSONSerialization(t *testing.T) {
	req := &ExistingNamespaceRequest{
		Repository: Repository{
			URL:    "https://github.com/test/existing-repo",
			Branch: "main",
		},
		ExistingNamespace: "existing-namespace",
	}

	// Test marshaling
	data, err := json.Marshal(req)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled ExistingNamespaceRequest
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, req.Repository.URL, unmarshaled.Repository.URL)
	assert.Equal(t, req.ExistingNamespace, unmarshaled.ExistingNamespace)
}

func TestUserInfo_JSONSerialization(t *testing.T) {
	userInfo := &UserInfo{
		Username: "test-user",
		Email:    "test@example.com",
		Groups:   []string{"developers", "admins"},
		Extra: map[string]string{
			"department": "engineering",
			"team":       "platform",
		},
	}

	// Test marshaling
	data, err := json.Marshal(userInfo)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled UserInfo
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, userInfo.Username, unmarshaled.Username)
	assert.Equal(t, userInfo.Email, unmarshaled.Email)
	assert.Equal(t, userInfo.Groups, unmarshaled.Groups)
	assert.Equal(t, userInfo.Extra, unmarshaled.Extra)
}

func TestAppProject_JSONSerialization(t *testing.T) {
	project := &AppProject{
		Name:      "test-project",
		Namespace: "argocd",
		SourceRepos: []string{
			"https://github.com/test/repo1",
			"https://github.com/test/repo2",
		},
		Destinations: []AppProjectDestination{
			{
				Server:    "https://kubernetes.default.svc",
				Namespace: "test-namespace",
			},
		},
		Roles: []AppProjectRole{
			{
				Name:     "developer",
				Policies: []string{"p, proj:test-project:developer, applications, sync, test-project/*, allow"},
			},
		},
		ClusterResourceWhitelist: []AppProjectResource{
			{
				Group: "",
				Kind:  "Namespace",
			},
		},
		NamespaceResourceWhitelist: []AppProjectResource{
			{
				Group: "",
				Kind:  "Secret",
			},
			{
				Group: "batch",
				Kind:  "Job",
			},
		},
	}

	// Test marshaling
	data, err := json.Marshal(project)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled AppProject
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, project.Name, unmarshaled.Name)
	assert.Equal(t, project.Namespace, unmarshaled.Namespace)
	assert.Equal(t, project.SourceRepos, unmarshaled.SourceRepos)
	assert.Len(t, unmarshaled.Destinations, 1)
	assert.Equal(t, project.Destinations[0].Server, unmarshaled.Destinations[0].Server)
	assert.Len(t, unmarshaled.NamespaceResourceWhitelist, 2)
}

func TestApplication_JSONSerialization(t *testing.T) {
	app := &Application{
		Name:      "test-app",
		Namespace: "argocd",
		Project:   "test-project",
		Source: ApplicationSource{
			RepoURL:        "https://github.com/test/repo",
			Path:           "manifests",
			TargetRevision: "main",
		},
		Destination: ApplicationDestination{
			Server:    "https://kubernetes.default.svc",
			Namespace: "test-namespace",
		},
		SyncPolicy: ApplicationSyncPolicy{
			Automated: &ApplicationSyncPolicyAutomated{
				Prune:    true,
				SelfHeal: true,
			},
			SyncOptions: []string{"CreateNamespace=true"},
			Retry: &ApplicationSyncPolicyRetry{
				Limit: 3,
				Backoff: &ApplicationSyncPolicyRetryBackoff{
					Duration:    "5s",
					Factor:      2,
					MaxDuration: "3m",
				},
			},
		},
	}

	// Test marshaling
	data, err := json.Marshal(app)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled Application
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, app.Name, unmarshaled.Name)
	assert.Equal(t, app.Project, unmarshaled.Project)
	assert.Equal(t, app.Source.RepoURL, unmarshaled.Source.RepoURL)
	assert.Equal(t, app.Destination.Namespace, unmarshaled.Destination.Namespace)
	assert.NotNil(t, unmarshaled.SyncPolicy.Automated)
	assert.True(t, unmarshaled.SyncPolicy.Automated.Prune)
	assert.Equal(t, int64(3), unmarshaled.SyncPolicy.Retry.Limit)
}

func TestCapacityStatus_JSONSerialization(t *testing.T) {
	capacity := &CapacityStatus{
		Enabled: true,
		Current: CapacityCurrentUsage{
			Namespaces:         150,
			UtilizationPercent: 15.0,
		},
		Limits: CapacityLimits{
			MaxNamespaces:      1000,
			EmergencyThreshold: 0.95,
		},
		Status:                  "normal",
		Message:                 "System operating normally",
		AllowNewNamespaces:      true,
		AllowExistingNamespaces: true,
	}

	// Test marshaling
	data, err := json.Marshal(capacity)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled CapacityStatus
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, capacity.Enabled, unmarshaled.Enabled)
	assert.Equal(t, capacity.Current.Namespaces, unmarshaled.Current.Namespaces)
	assert.Equal(t, capacity.Current.UtilizationPercent, unmarshaled.Current.UtilizationPercent)
	assert.Equal(t, capacity.Limits.MaxNamespaces, unmarshaled.Limits.MaxNamespaces)
	assert.Equal(t, capacity.Status, unmarshaled.Status)
	assert.Equal(t, capacity.AllowNewNamespaces, unmarshaled.AllowNewNamespaces)
}

func TestErrorResponse_JSONSerialization(t *testing.T) {
	errorResp := &ErrorResponse{
		Error:   "VALIDATION_ERROR",
		Message: "Request validation failed",
		Details: map[string]interface{}{
			"field":  "namespace",
			"reason": "invalid format",
		},
		Code: 400,
	}

	// Test marshaling
	data, err := json.Marshal(errorResp)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled ErrorResponse
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, errorResp.Error, unmarshaled.Error)
	assert.Equal(t, errorResp.Message, unmarshaled.Message)
	assert.Equal(t, errorResp.Code, unmarshaled.Code)
	assert.Equal(t, "namespace", unmarshaled.Details["field"])
}

func TestHealthResponse_JSONSerialization(t *testing.T) {
	healthResp := &HealthResponse{
		Status:    "ready",
		Timestamp: "2023-01-01T12:00:00Z",
		Service:   "gitops-registration-service",
		Details: map[string]interface{}{
			"kubernetes": "connected",
			"argocd":     "connected",
		},
	}

	// Test marshaling
	data, err := json.Marshal(healthResp)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled HealthResponse
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, healthResp.Status, unmarshaled.Status)
	assert.Equal(t, healthResp.Service, unmarshaled.Service)
	assert.Equal(t, "connected", unmarshaled.Details["kubernetes"])
}

func TestRepository_DefaultValues(t *testing.T) {
	repo := Repository{
		URL: "https://github.com/test/repo",
		// Branch not set - should work with empty value
	}

	data, err := json.Marshal(repo)
	require.NoError(t, err)

	var unmarshaled Repository
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, "https://github.com/test/repo", unmarshaled.URL)
	assert.Equal(t, "", unmarshaled.Branch)
}

func TestCredentials_Types(t *testing.T) {
	testCases := []struct {
		name        string
		credentials Credentials
	}{
		{
			name: "token credentials",
			credentials: Credentials{
				Type:      "token",
				SecretRef: "github-token-secret",
			},
		},
		{
			name: "ssh credentials",
			credentials: Credentials{
				Type:      "ssh",
				SecretRef: "github-ssh-secret",
			},
		},
		{
			name: "github-app credentials",
			credentials: Credentials{
				Type:      "github-app",
				SecretRef: "github-app-secret",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.credentials)
			require.NoError(t, err)

			var unmarshaled Credentials
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			assert.Equal(t, tc.credentials.Type, unmarshaled.Type)
			assert.Equal(t, tc.credentials.SecretRef, unmarshaled.SecretRef)
		})
	}
}

func TestRegistrationStatus_Phases(t *testing.T) {
	phases := []string{"pending", "active", "failed", "deleting"}

	for _, phase := range phases {
		status := RegistrationStatus{
			Phase:   phase,
			Message: fmt.Sprintf("Registration is %s", phase),
		}

		data, err := json.Marshal(status)
		require.NoError(t, err)

		var unmarshaled RegistrationStatus
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, phase, unmarshaled.Phase)
		assert.Contains(t, unmarshaled.Message, phase)
	}
}

func TestCapacityStatus_StatusValues(t *testing.T) {
	statusValues := []string{"normal", "warning", "capacity_reached", "emergency"}

	for _, status := range statusValues {
		capacity := CapacityStatus{
			Status:  status,
			Message: fmt.Sprintf("System status: %s", status),
		}

		data, err := json.Marshal(capacity)
		require.NoError(t, err)

		var unmarshaled CapacityStatus
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, status, unmarshaled.Status)
		assert.Contains(t, unmarshaled.Message, status)
	}
}
