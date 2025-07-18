package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds the complete application configuration
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	ArgoCD        ArgoCDConfig        `yaml:"argocd"`
	Kubernetes    KubernetesConfig    `yaml:"kubernetes"`
	Security      SecurityConfig      `yaml:"security"`
	Registration  RegistrationConfig  `yaml:"registration"`
	Authorization AuthorizationConfig `yaml:"authorization"`
	Tenants       TenantsConfig       `yaml:"tenants"`
	Capacity      CapacityConfig      `yaml:"capacity"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port    int    `yaml:"port"`
	Timeout string `yaml:"timeout"`
}

// ArgoCDConfig holds ArgoCD connection configuration
type ArgoCDConfig struct {
	Server    string `yaml:"server"`
	Namespace string `yaml:"namespace"`
	GRPC      bool   `yaml:"grpc"`
}

// KubernetesConfig holds Kubernetes client configuration
type KubernetesConfig struct {
	Namespace string `yaml:"namespace"`
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	AllowedResourceTypes       []string                     `yaml:"allowedResourceTypes"`
	ResourceAllowList          []ServiceResourceRestriction `yaml:"resourceAllowList,omitempty"`
	ResourceDenyList           []ServiceResourceRestriction `yaml:"resourceDenyList,omitempty"`
	RequireAppProjectPerTenant bool                         `yaml:"requireAppProjectPerTenant"`
	// Deprecated: Use Impersonation.Enabled instead
	EnableServiceAccountImpersonation bool `yaml:"enableServiceAccountImpersonation"`
	// New impersonation configuration
	Impersonation ImpersonationConfig `yaml:"impersonation"`
}

// ImpersonationConfig holds ArgoCD impersonation configuration
type ImpersonationConfig struct {
	Enabled                bool   `yaml:"enabled"`
	ClusterRole            string `yaml:"clusterRole"`
	ServiceAccountBaseName string `yaml:"serviceAccountBaseName"`
	ValidatePermissions    bool   `yaml:"validatePermissions"`
	AutoCleanup            bool   `yaml:"autoCleanup"`
}

// ServiceResourceRestriction represents a resource type restriction for service-level configuration
type ServiceResourceRestriction struct {
	Group string `yaml:"group" json:"group"`
	Kind  string `yaml:"kind" json:"kind"`
}

// RegistrationConfig holds registration control settings
type RegistrationConfig struct {
	AllowNewNamespaces bool `yaml:"allowNewNamespaces"`
}

// AuthorizationConfig holds authorization configuration
type AuthorizationConfig struct {
	RequiredRole              string `yaml:"requiredRole"`
	EnableSubjectAccessReview bool   `yaml:"enableSubjectAccessReview"`
	AuditFailedAttempts       bool   `yaml:"auditFailedAttempts"`
}

// TenantsConfig holds tenant-related configuration
type TenantsConfig struct {
	NamespacePrefix      string            `yaml:"namespacePrefix"`
	DefaultResourceQuota map[string]string `yaml:"defaultResourceQuota"`
}

// CapacityConfig holds capacity management configuration
type CapacityConfig struct {
	Enabled bool           `yaml:"enabled"`
	Limits  CapacityLimits `yaml:"limits"`
}

// CapacityLimits represents capacity limits configuration
type CapacityLimits struct {
	MaxNamespaces      int     `yaml:"maxNamespaces"`
	MaxTenantsPerUser  int     `yaml:"maxTenantsPerUser"`
	EmergencyThreshold float64 `yaml:"emergencyThreshold"`
}

// Load reads configuration from environment variables and config file
func Load() (*Config, error) {
	// Set defaults
	cfg := getDefaultConfig()

	// Load from config file if specified (before environment variable overrides)
	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		if err := loadFromFile(cfg, configPath); err != nil {
			return nil, fmt.Errorf("failed to load config file %s: %w", configPath, err)
		}
	}

	// Override with environment variables (these take precedence over file config)
	applyEnvironmentOverrides(cfg)

	// Validate resource restrictions
	if err := validateResourceRestrictions(cfg.Security.ResourceAllowList, cfg.Security.ResourceDenyList); err != nil {
		return nil, fmt.Errorf("invalid resource restrictions configuration: %w", err)
	}

	return cfg, nil
}

// getDefaultConfig returns a Config with default values
func getDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:    8080,
			Timeout: "30s",
		},
		ArgoCD: ArgoCDConfig{
			Server:    "argocd-server.argocd.svc.cluster.local",
			Namespace: "argocd",
			GRPC:      true,
		},
		Kubernetes: KubernetesConfig{
			Namespace: "gitops-registration-system",
		},
		Security: SecurityConfig{
			AllowedResourceTypes: []string{
				"jobs",
				"cronjobs",
				"secrets",
				"rolebindings",
			},
			RequireAppProjectPerTenant:        true,
			EnableServiceAccountImpersonation: true,
			Impersonation: ImpersonationConfig{
				Enabled:                false, // Default to disabled for security
				ClusterRole:            "",    // Must be explicitly set when enabled
				ServiceAccountBaseName: "gitops-sa",
				ValidatePermissions:    true,
				AutoCleanup:            true,
			},
		},
		Registration: RegistrationConfig{
			AllowNewNamespaces: true,
		},
		Authorization: AuthorizationConfig{
			RequiredRole:              "konflux-admin-user-actions",
			EnableSubjectAccessReview: true,
			AuditFailedAttempts:       true,
		},
		Tenants: TenantsConfig{
			NamespacePrefix: "",
			DefaultResourceQuota: map[string]string{
				"requests.cpu":           "1",
				"requests.memory":        "2Gi",
				"limits.cpu":             "4",
				"limits.memory":          "8Gi",
				"persistentvolumeclaims": "10",
			},
		},
	}
}

// applyEnvironmentOverrides applies environment variable overrides to the config
func applyEnvironmentOverrides(cfg *Config) {
	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Server.Port = p
		}
	}

	if timeout := os.Getenv("SERVER_TIMEOUT"); timeout != "" {
		cfg.Server.Timeout = timeout
	}

	if argoCDServer := os.Getenv("ARGOCD_SERVER"); argoCDServer != "" {
		cfg.ArgoCD.Server = argoCDServer
	}

	if argoCDNamespace := os.Getenv("ARGOCD_NAMESPACE"); argoCDNamespace != "" {
		cfg.ArgoCD.Namespace = argoCDNamespace
	}

	if k8sNamespace := os.Getenv("KUBERNETES_NAMESPACE"); k8sNamespace != "" {
		cfg.Kubernetes.Namespace = k8sNamespace
	}

	if allowedResources := os.Getenv("ALLOWED_RESOURCE_TYPES"); allowedResources != "" {
		cfg.Security.AllowedResourceTypes = strings.Split(allowedResources, ",")
	}

	if allowNewNamespaces := os.Getenv("ALLOW_NEW_NAMESPACES"); allowNewNamespaces != "" {
		if allowed, err := strconv.ParseBool(allowNewNamespaces); err == nil {
			cfg.Registration.AllowNewNamespaces = allowed
		}
	}

	if requiredRole := os.Getenv("AUTHORIZATION_REQUIRED_ROLE"); requiredRole != "" {
		cfg.Authorization.RequiredRole = requiredRole
	}
}

// loadFromFile loads configuration from a YAML file
func loadFromFile(cfg *Config, path string) error {
	// Validate path to prevent file inclusion vulnerabilities
	cleanPath := filepath.Clean(path)

	// Check for directory traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("config path contains directory traversal: %s", path)
	}

	// Validate file extension
	if ext := filepath.Ext(cleanPath); ext != ".yaml" && ext != ".yml" {
		return fmt.Errorf("config file must be a YAML file (.yaml or .yml): %s", path)
	}

	data, err := os.ReadFile(cleanPath) // #nosec G304 -- path is validated above
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, cfg)
}

// validateResourceRestrictions validates service-level resource restrictions
func validateResourceRestrictions(allowList, denyList []ServiceResourceRestriction) error {
	// Ensure only allowList OR denyList is provided, not both
	if len(allowList) > 0 && len(denyList) > 0 {
		return fmt.Errorf("cannot specify both resourceAllowList and resourceDenyList; provide only one")
	}

	// Validate allowList entries
	for i, resource := range allowList {
		if resource.Kind == "" {
			return fmt.Errorf("resourceAllowList[%d]: kind is required", i)
		}
		// Note: group can be empty for core resources, so we don't validate it
	}

	// Validate denyList entries
	for i, resource := range denyList {
		if resource.Kind == "" {
			return fmt.Errorf("resourceDenyList[%d]: kind is required", i)
		}
		// Note: group can be empty for core resources, so we don't validate it
	}

	return nil
}

// ValidateImpersonationConfig validates the impersonation configuration
func (c *Config) ValidateImpersonationConfig() error {
	if !c.Security.Impersonation.Enabled {
		return nil // No validation needed if disabled
	}

	if c.Security.Impersonation.ClusterRole == "" {
		return fmt.Errorf("impersonation.clusterRole must be set when impersonation is enabled")
	}

	if c.Security.Impersonation.ServiceAccountBaseName == "" {
		return fmt.Errorf("impersonation.serviceAccountBaseName cannot be empty")
	}

	return nil
}
