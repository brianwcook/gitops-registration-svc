package services

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/konflux-ci/gitops-registration-service/internal/types"
)

func TestArgoCDService_ConvertResourceListToInterface(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	// Use stub implementation for testing
	argoCDService := &argoCDServiceStub{
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
			result := argoCDService.convertResourceListToInterface(tt.resources)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestArgoCDService_CreateAppProject_ResourceRestrictions(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	argoCDService := &argoCDServiceStub{
		logger: logger,
	}

	tests := []struct {
		name     string
		project  *types.AppProject
		expectOK bool
	}{
		{
			name: "AppProject with allowList",
			project: &types.AppProject{
				Name: "test-project-allowlist",
				Destinations: []types.AppProjectDestination{
					{Server: "https://kubernetes.default.svc", Namespace: "test-namespace"},
				},
				SourceRepos: []string{"https://github.com/test/repo"},
				ClusterResourceWhitelist: []types.AppProjectResource{
					{Group: "apps", Kind: "Deployment"},
				},
				NamespaceResourceWhitelist: []types.AppProjectResource{
					{Group: "", Kind: "ConfigMap"},
				},
			},
			expectOK: true,
		},
		{
			name: "AppProject with denyList",
			project: &types.AppProject{
				Name: "test-project-denylist",
				Destinations: []types.AppProjectDestination{
					{Server: "https://kubernetes.default.svc", Namespace: "test-namespace"},
				},
				SourceRepos: []string{"https://github.com/test/repo"},
				ClusterResourceBlacklist: []types.AppProjectResource{
					{Group: "kafka.strimzi.io", Kind: "KafkaTopic"},
				},
				NamespaceResourceBlacklist: []types.AppProjectResource{
					{Group: "database.example.com", Kind: "MySQLDatabase"},
				},
			},
			expectOK: true,
		},
		{
			name: "AppProject with no restrictions",
			project: &types.AppProject{
				Name: "test-project-no-restrictions",
				Destinations: []types.AppProjectDestination{
					{Server: "https://kubernetes.default.svc", Namespace: "test-namespace"},
				},
				SourceRepos: []string{"https://github.com/test/repo"},
			},
			expectOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := argoCDService.CreateAppProject(nil, tt.project)

			if tt.expectOK {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
