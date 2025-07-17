package types

import (
	"time"
)

// Registration represents a GitOps repository registration
type Registration struct {
	ID          string             `json:"id"`
	Repository  Repository         `json:"repository"`
	Namespace   string             `json:"namespace"`
	Status      RegistrationStatus `json:"status"`
	CreatedAt   time.Time          `json:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt"`
	Labels      map[string]string  `json:"labels,omitempty"`
	Annotations map[string]string  `json:"annotations,omitempty"`
}

// Repository represents a Git repository configuration
type Repository struct {
	URL         string      `json:"url"`
	Branch      string      `json:"branch"`
	Credentials Credentials `json:"credentials,omitempty"`
}

// Credentials represents repository access credentials
type Credentials struct {
	Type      string `json:"type"` // token, ssh, github-app
	SecretRef string `json:"secretRef"`
}

// RegistrationStatus represents the status of a registration
type RegistrationStatus struct {
	Phase              string    `json:"phase"` // pending, active, failed, deleting
	Message            string    `json:"message,omitempty"`
	ArgoCDApplication  string    `json:"argocdApplication,omitempty"`
	ArgoCDAppProject   string    `json:"argocdAppProject,omitempty"`
	LastSyncTime       time.Time `json:"lastSyncTime,omitempty"`
	NamespaceCreated   bool      `json:"namespaceCreated"`
	AppProjectCreated  bool      `json:"appProjectCreated"`
	ApplicationCreated bool      `json:"applicationCreated"`
}

// RegistrationRequest represents a request to register a new GitOps repository
type RegistrationRequest struct {
	Repository Repository `json:"repository"`
	Namespace  string     `json:"namespace"`
}

// ExistingNamespaceRequest represents a request to register an existing namespace
type ExistingNamespaceRequest struct {
	Repository        Repository `json:"repository"`
	ExistingNamespace string     `json:"existingNamespace"`
}

// UserInfo represents authenticated user information
type UserInfo struct {
	Username string            `json:"username"`
	Email    string            `json:"email,omitempty"`
	Groups   []string          `json:"groups,omitempty"`
	Extra    map[string]string `json:"extra,omitempty"`
}

// AppProject represents an ArgoCD AppProject configuration
type AppProject struct {
	Name                       string                  `json:"name"`
	Namespace                  string                  `json:"namespace"`
	SourceRepos                []string                `json:"sourceRepos"`
	Destinations               []AppProjectDestination `json:"destinations"`
	Roles                      []AppProjectRole        `json:"roles,omitempty"`
	ClusterResourceWhitelist   []AppProjectResource    `json:"clusterResourceWhitelist,omitempty"`
	NamespaceResourceWhitelist []AppProjectResource    `json:"namespaceResourceWhitelist,omitempty"`
	ClusterResourceBlacklist   []AppProjectResource    `json:"clusterResourceBlacklist,omitempty"`
	NamespaceResourceBlacklist []AppProjectResource    `json:"namespaceResourceBlacklist,omitempty"`
}

// AppProjectDestination represents allowed destinations for an AppProject
type AppProjectDestination struct {
	Server    string `json:"server"`
	Namespace string `json:"namespace"`
}

// AppProjectRole represents a role within an AppProject
type AppProjectRole struct {
	Name     string   `json:"name"`
	Policies []string `json:"policies"`
}

// AppProjectResource represents allowed resources for an AppProject
type AppProjectResource struct {
	Group string `json:"group"`
	Kind  string `json:"kind"`
}

// Application represents an ArgoCD Application configuration
type Application struct {
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace"`
	Project     string                 `json:"project"`
	Source      ApplicationSource      `json:"source"`
	Destination ApplicationDestination `json:"destination"`
	SyncPolicy  ApplicationSyncPolicy  `json:"syncPolicy,omitempty"`
}

// ApplicationSource represents the source configuration for an Application
type ApplicationSource struct {
	RepoURL        string `json:"repoURL"`
	Path           string `json:"path"`
	TargetRevision string `json:"targetRevision"`
}

// ApplicationDestination represents the destination for an Application
type ApplicationDestination struct {
	Server    string `json:"server"`
	Namespace string `json:"namespace"`
}

// ApplicationSyncPolicy represents sync policy for an Application
type ApplicationSyncPolicy struct {
	Automated   *ApplicationSyncPolicyAutomated `json:"automated,omitempty"`
	SyncOptions []string                        `json:"syncOptions,omitempty"`
	Retry       *ApplicationSyncPolicyRetry     `json:"retry,omitempty"`
}

// ApplicationSyncPolicyAutomated represents automated sync policy
type ApplicationSyncPolicyAutomated struct {
	Prune    bool `json:"prune,omitempty"`
	SelfHeal bool `json:"selfHeal,omitempty"`
}

// ApplicationSyncPolicyRetry represents retry policy for sync operations
type ApplicationSyncPolicyRetry struct {
	Limit   int64                              `json:"limit,omitempty"`
	Backoff *ApplicationSyncPolicyRetryBackoff `json:"backoff,omitempty"`
}

// ApplicationSyncPolicyRetryBackoff represents backoff policy for retries
type ApplicationSyncPolicyRetryBackoff struct {
	Duration    string `json:"duration,omitempty"`
	Factor      int64  `json:"factor,omitempty"`
	MaxDuration string `json:"maxDuration,omitempty"`
}

// ApplicationStatus represents the status of an ArgoCD Application
type ApplicationStatus struct {
	Phase        string    `json:"phase"`
	Message      string    `json:"message,omitempty"`
	LastSyncTime time.Time `json:"lastSyncTime,omitempty"`
	Health       string    `json:"health"`
	Sync         string    `json:"sync"`
}

// ServiceRegistrationStatus represents current service registration settings
type ServiceRegistrationStatus struct {
	AllowNewNamespaces bool   `json:"allowNewNamespaces"`
	Message            string `json:"message,omitempty"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string                 `json:"error"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
	Code    int                    `json:"code,omitempty"`
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status    string                 `json:"status"`
	Timestamp string                 `json:"timestamp"`
	Service   string                 `json:"service,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// CapacityStatus represents current capacity status (stub types for tests)
type CapacityStatus struct {
	Enabled                 bool                 `json:"enabled"`
	Current                 CapacityCurrentUsage `json:"current"`
	Limits                  CapacityLimits       `json:"limits"`
	Status                  string               `json:"status"`
	Message                 string               `json:"message,omitempty"`
	AllowNewNamespaces      bool                 `json:"allowNewNamespaces"`
	AllowExistingNamespaces bool                 `json:"allowExistingNamespaces"`
}

// CapacityCurrentUsage represents current usage metrics
type CapacityCurrentUsage struct {
	Namespaces         int     `json:"namespaces"`
	UtilizationPercent float64 `json:"utilizationPercent"`
}

// CapacityLimits represents capacity limits
type CapacityLimits struct {
	MaxNamespaces      int     `json:"maxNamespaces"`
	EmergencyThreshold float64 `json:"emergencyThreshold"`
}
