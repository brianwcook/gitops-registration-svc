package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear environment variables
	clearEnvVars()

	cfg, err := Load()
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify defaults
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "30s", cfg.Server.Timeout)
	assert.Equal(t, "argocd-server.argocd.svc.cluster.local", cfg.ArgoCD.Server)
	assert.Equal(t, "argocd", cfg.ArgoCD.Namespace)
	assert.True(t, cfg.ArgoCD.GRPC)
	assert.Equal(t, "gitops-registration-system", cfg.Kubernetes.Namespace)

	// Security defaults
	assert.Equal(t, []string{"jobs", "cronjobs", "secrets", "rolebindings"}, cfg.Security.AllowedResourceTypes)
	assert.True(t, cfg.Security.RequireAppProjectPerTenant)
	assert.True(t, cfg.Security.EnableServiceAccountImpersonation)

	// Registration defaults
	assert.True(t, cfg.Registration.AllowNewNamespaces)

	// Authorization defaults
	assert.Equal(t, "konflux-admin-user-actions", cfg.Authorization.RequiredRole)
	assert.True(t, cfg.Authorization.EnableSubjectAccessReview)
	assert.True(t, cfg.Authorization.AuditFailedAttempts)
}

func TestLoad_EnvironmentVariables(t *testing.T) {
	clearEnvVars()

	// Set environment variables
	envVars := map[string]string{
		"PORT":                        "9090",
		"SERVER_TIMEOUT":              "45s",
		"ARGOCD_SERVER":               "custom-argocd.example.com",
		"ARGOCD_NAMESPACE":            "custom-argocd",
		"KUBERNETES_NAMESPACE":        "custom-namespace",
		"ALLOWED_RESOURCE_TYPES":      "jobs,secrets",
		"ALLOW_NEW_NAMESPACES":        "false",
		"AUTHORIZATION_REQUIRED_ROLE": "custom-role",
	}

	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer clearEnvVars()

	cfg, err := Load()
	require.NoError(t, err)

	// Verify environment variable overrides
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "45s", cfg.Server.Timeout)
	assert.Equal(t, "custom-argocd.example.com", cfg.ArgoCD.Server)
	assert.Equal(t, "custom-argocd", cfg.ArgoCD.Namespace)
	assert.Equal(t, "custom-namespace", cfg.Kubernetes.Namespace)
	assert.Equal(t, []string{"jobs", "secrets"}, cfg.Security.AllowedResourceTypes)
	assert.False(t, cfg.Registration.AllowNewNamespaces)
	assert.Equal(t, "custom-role", cfg.Authorization.RequiredRole)
}

func TestLoad_ConfigFile(t *testing.T) {
	clearEnvVars()

	// Create temporary config file
	configContent := `
server:
  port: 7070
  timeout: 60s
argocd:
  server: "file-argocd.example.com"
  namespace: "file-argocd"
  grpc: false
security:
  allowedResourceTypes:
  - jobs
  - configmaps
registration:
  allowNewNamespaces: false
authorization:
  requiredRole: "file-role"
  enableSubjectAccessReview: false
`

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0644))

	os.Setenv("CONFIG_PATH", configFile)
	defer os.Unsetenv("CONFIG_PATH")

	cfg, err := Load()
	require.NoError(t, err)

	// Verify file-based configuration
	assert.Equal(t, 7070, cfg.Server.Port)
	assert.Equal(t, "60s", cfg.Server.Timeout)
	assert.Equal(t, "file-argocd.example.com", cfg.ArgoCD.Server)
	assert.Equal(t, "file-argocd", cfg.ArgoCD.Namespace)
	assert.False(t, cfg.ArgoCD.GRPC)
	assert.Equal(t, []string{"jobs", "configmaps"}, cfg.Security.AllowedResourceTypes)
	assert.False(t, cfg.Registration.AllowNewNamespaces)
	assert.Equal(t, "file-role", cfg.Authorization.RequiredRole)
	assert.False(t, cfg.Authorization.EnableSubjectAccessReview)
}

func TestLoad_EnvironmentOverridesFile(t *testing.T) {
	clearEnvVars()

	// Create config file
	configContent := `
server:
  port: 7070
argocd:
  server: "file-argocd.example.com"
`

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0644))

	// Set environment variables that should override file
	os.Setenv("CONFIG_PATH", configFile)
	os.Setenv("PORT", "8888")
	os.Setenv("ARGOCD_SERVER", "env-argocd.example.com")
	defer clearEnvVars()

	cfg, err := Load()
	require.NoError(t, err)

	// Environment should override file
	assert.Equal(t, 8888, cfg.Server.Port)
	assert.Equal(t, "env-argocd.example.com", cfg.ArgoCD.Server)
}

func TestLoad_InvalidConfigFile(t *testing.T) {
	clearEnvVars()

	// Create invalid YAML file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("invalid: yaml: content: ["), 0644))

	os.Setenv("CONFIG_PATH", configFile)
	defer os.Unsetenv("CONFIG_PATH")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load config file")
}

func TestLoad_NonExistentConfigFile(t *testing.T) {
	clearEnvVars()

	os.Setenv("CONFIG_PATH", "/nonexistent/config.yaml")
	defer os.Unsetenv("CONFIG_PATH")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load config file")
}

func TestLoad_InvalidEnvironmentVariables(t *testing.T) {
	clearEnvVars()

	testCases := []struct {
		name   string
		envVar string
		value  string
	}{
		{"invalid port", "PORT", "invalid"},
		{"invalid allow new namespaces", "ALLOW_NEW_NAMESPACES", "invalid"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clearEnvVars()
			os.Setenv(tc.envVar, tc.value)
			defer os.Unsetenv(tc.envVar)

			cfg, err := Load()
			// Should not error, invalid values should be ignored and defaults used
			require.NoError(t, err)
			assert.NotNil(t, cfg)
		})
	}
}

func TestValidateResourceRestrictions(t *testing.T) {
	tests := []struct {
		name        string
		allowList   []ServiceResourceRestriction
		denyList    []ServiceResourceRestriction
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid allowList only",
			allowList:   []ServiceResourceRestriction{{Group: "apps", Kind: "Deployment"}},
			denyList:    nil,
			expectError: false,
		},
		{
			name:        "Valid denyList only",
			allowList:   nil,
			denyList:    []ServiceResourceRestriction{{Group: "kafka.strimzi.io", Kind: "KafkaTopic"}},
			expectError: false,
		},
		{
			name:        "Neither allowList nor denyList",
			allowList:   nil,
			denyList:    nil,
			expectError: false,
		},
		{
			name:        "Both allowList and denyList provided",
			allowList:   []ServiceResourceRestriction{{Group: "apps", Kind: "Deployment"}},
			denyList:    []ServiceResourceRestriction{{Group: "kafka.strimzi.io", Kind: "KafkaTopic"}},
			expectError: true,
			errorMsg:    "cannot specify both resourceAllowList and resourceDenyList; provide only one",
		},
		{
			name:        "allowList with empty kind",
			allowList:   []ServiceResourceRestriction{{Group: "apps", Kind: ""}},
			denyList:    nil,
			expectError: true,
			errorMsg:    "resourceAllowList[0]: kind is required",
		},
		{
			name:        "denyList with empty kind",
			allowList:   nil,
			denyList:    []ServiceResourceRestriction{{Group: "apps", Kind: ""}},
			expectError: true,
			errorMsg:    "resourceDenyList[0]: kind is required",
		},
		{
			name:        "allowList with empty group (valid for core resources)",
			allowList:   []ServiceResourceRestriction{{Group: "", Kind: "ConfigMap"}},
			denyList:    nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateResourceRestrictions(tt.allowList, tt.denyList)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoad_ConfigFile_WithResourceRestrictions(t *testing.T) {
	clearEnvVars()

	// Test with allowList
	configContentAllowList := `
server:
  port: 7070
security:
  resourceAllowList:
  - group: "apps"
    kind: "Deployment"
  - group: ""
    kind: "ConfigMap"
`

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(configContentAllowList), 0644))

	os.Setenv("CONFIG_PATH", configFile)
	defer os.Unsetenv("CONFIG_PATH")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Len(t, cfg.Security.ResourceAllowList, 2)
	assert.Equal(t, "apps", cfg.Security.ResourceAllowList[0].Group)
	assert.Equal(t, "Deployment", cfg.Security.ResourceAllowList[0].Kind)
	assert.Equal(t, "", cfg.Security.ResourceAllowList[1].Group)
	assert.Equal(t, "ConfigMap", cfg.Security.ResourceAllowList[1].Kind)
	assert.Empty(t, cfg.Security.ResourceDenyList)
}

func TestLoad_ConfigFile_WithInvalidResourceRestrictions(t *testing.T) {
	clearEnvVars()

	// Test with both allowList and denyList (invalid)
	configContent := `
security:
  resourceAllowList:
  - group: "apps"
    kind: "Deployment"
  resourceDenyList:
  - group: ""
    kind: "Secret"
`

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0644))

	os.Setenv("CONFIG_PATH", configFile)
	defer os.Unsetenv("CONFIG_PATH")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot specify both resourceAllowList and resourceDenyList")
}

func TestLoadFromFile_Success(t *testing.T) {
	cfg := &Config{}

	configContent := `
server:
  port: 9999
  timeout: "10s"
`

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0644))

	err := loadFromFile(cfg, configFile)
	require.NoError(t, err)

	assert.Equal(t, 9999, cfg.Server.Port)
	assert.Equal(t, "10s", cfg.Server.Timeout)
}

func TestLoadFromFile_FileNotFound(t *testing.T) {
	cfg := &Config{}

	err := loadFromFile(cfg, "/nonexistent/file.yaml")
	assert.Error(t, err)
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	cfg := &Config{}

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("invalid: yaml: ["), 0644))

	err := loadFromFile(cfg, configFile)
	assert.Error(t, err)
}

// Helper function to clear all environment variables used by the config
func clearEnvVars() {
	envVars := []string{
		"PORT",
		"SERVER_TIMEOUT",
		"ARGOCD_SERVER",
		"ARGOCD_NAMESPACE",
		"KUBERNETES_NAMESPACE",
		"ALLOWED_RESOURCE_TYPES",
		"ALLOW_NEW_NAMESPACES",
		"AUTHORIZATION_REQUIRED_ROLE",
		"CONFIG_PATH",
	}

	for _, env := range envVars {
		os.Unsetenv(env)
	}
}
