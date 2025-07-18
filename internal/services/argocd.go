package services

import (
	"context"
	"fmt"
	"time"

	"github.com/konflux-ci/gitops-registration-service/internal/config"
	"github.com/konflux-ci/gitops-registration-service/internal/types"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// argoCDService is the real implementation of ArgoCDService
type argoCDService struct {
	client    dynamic.Interface
	cfg       *config.Config
	logger    *logrus.Logger
	namespace string
}

// ArgoCD CRD GroupVersionResources
var (
	appProjectGVR = schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "appprojects",
	}

	applicationGVR = schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}
)

// NewArgoCDServiceReal creates a new real ArgoCDService implementation
func NewArgoCDServiceReal(cfg *config.Config, logger *logrus.Logger) (ArgoCDService, error) {
	factory := &InClusterArgoCDFactory{}
	return NewArgoCDServiceWithFactory(cfg, logger, factory)
}

// NewArgoCDServiceWithFactory creates an ArgoCDService using the provided factory
func NewArgoCDServiceWithFactory(cfg *config.Config, logger *logrus.Logger, factory ArgoCDClientFactory) (ArgoCDService, error) {
	// Create config using factory
	config, err := factory.CreateConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	// Create dynamic client using factory
	client, err := factory.CreateDynamicClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &argoCDService{
		client:    client,
		cfg:       cfg,
		logger:    logger,
		namespace: "argocd", // ArgoCD is typically installed in the argocd namespace
	}, nil
}

func (a *argoCDService) CreateAppProject(ctx context.Context, project *types.AppProject) error {
	a.logger.WithField("project", project.Name).Info("Creating ArgoCD AppProject")

	spec := a.buildProjectSpec(project)
	appProject := a.buildAppProjectResource(project, spec)

	_, err := a.client.Resource(appProjectGVR).Namespace(a.namespace).Create(ctx, appProject, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			a.logger.WithField("project", project.Name).Info("AppProject already exists")
			return nil
		}
		return fmt.Errorf("failed to create AppProject %s: %w", project.Name, err)
	}

	a.logger.WithField("project", project.Name).Info("Successfully created ArgoCD AppProject")
	return nil
}

// buildProjectSpec creates the spec section for an AppProject
func (a *argoCDService) buildProjectSpec(project *types.AppProject) map[string]interface{} {
	spec := map[string]interface{}{
		"sourceRepos": project.SourceRepos,
		"destinations": []interface{}{
			map[string]interface{}{
				"namespace": project.Destinations[0].Namespace,
				"server":    project.Destinations[0].Server,
			},
		},
		"roles": []interface{}{
			map[string]interface{}{
				"name": "tenant-role",
				"policies": []string{
					fmt.Sprintf("p, proj:%s:tenant-role, applications, sync, %s/*, allow", project.Name, project.Name),
					fmt.Sprintf("p, proj:%s:tenant-role, applications, get, %s/*, allow", project.Name, project.Name),
					fmt.Sprintf("p, proj:%s:tenant-role, applications, update, %s/*, allow", project.Name, project.Name),
				},
			},
		},
	}

	a.addResourceRestrictions(spec, project)
	return spec
}

// addResourceRestrictions adds resource allow/deny lists to the project spec
func (a *argoCDService) addResourceRestrictions(spec map[string]interface{}, project *types.AppProject) {
	switch {
	case len(project.ClusterResourceWhitelist) > 0 || len(project.NamespaceResourceWhitelist) > 0:
		// Use whitelist (allowList)
		if len(project.ClusterResourceWhitelist) > 0 {
			spec["clusterResourceWhitelist"] = a.convertResourceListToInterface(project.ClusterResourceWhitelist)
		}
		if len(project.NamespaceResourceWhitelist) > 0 {
			spec["namespaceResourceWhitelist"] = a.convertResourceListToInterface(project.NamespaceResourceWhitelist)
		}
	case len(project.ClusterResourceBlacklist) > 0 || len(project.NamespaceResourceBlacklist) > 0:
		// Use blacklist (denyList)
		if len(project.ClusterResourceBlacklist) > 0 {
			spec["clusterResourceBlacklist"] = a.convertResourceListToInterface(project.ClusterResourceBlacklist)
		}
		if len(project.NamespaceResourceBlacklist) > 0 {
			spec["namespaceResourceBlacklist"] = a.convertResourceListToInterface(project.NamespaceResourceBlacklist)
		}
	default:
		// No restrictions provided - use default secure whitelist
		spec["clusterResourceWhitelist"] = []interface{}{}
		spec["namespaceResourceWhitelist"] = a.buildDefaultResourceWhitelist()
	}
}

// buildDefaultResourceWhitelist returns the default secure resource whitelist
func (a *argoCDService) buildDefaultResourceWhitelist() []interface{} {
	return []interface{}{
		map[string]interface{}{"group": "", "kind": "ConfigMap"},
		map[string]interface{}{"group": "", "kind": "Secret"},
		map[string]interface{}{"group": "", "kind": "Service"},
		map[string]interface{}{"group": "", "kind": "ServiceAccount"},
		map[string]interface{}{"group": "apps", "kind": "Deployment"},
		map[string]interface{}{"group": "apps", "kind": "ReplicaSet"},
		map[string]interface{}{"group": "batch", "kind": "Job"},
		map[string]interface{}{"group": "batch", "kind": "CronJob"},
		map[string]interface{}{"group": "rbac.authorization.k8s.io", "kind": "Role"},
		map[string]interface{}{"group": "rbac.authorization.k8s.io", "kind": "RoleBinding"},
		map[string]interface{}{"group": "networking.k8s.io", "kind": "NetworkPolicy"},
	}
}

// buildAppProjectResource creates the full AppProject unstructured resource
func (a *argoCDService) buildAppProjectResource(project *types.AppProject, spec map[string]interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "AppProject",
			"metadata": map[string]interface{}{
				"name":      project.Name,
				"namespace": a.namespace,
				"labels": map[string]interface{}{
					"gitops.io/managed-by":         "gitops-registration-service",
					"app.kubernetes.io/managed-by": "gitops-registration-service",
					"gitops.io/tenant":             project.Destinations[0].Namespace,
				},
			},
			"spec": spec,
		},
	}
}

func (a *argoCDService) convertResourceListToInterface(resources []types.AppProjectResource) []interface{} {
	result := make([]interface{}, len(resources))
	for i, resource := range resources {
		result[i] = map[string]interface{}{
			"group": resource.Group,
			"kind":  resource.Kind,
		}
	}
	return result
}

// deleteResource is a helper function that handles deletion of ArgoCD resources
func (a *argoCDService) deleteResource(ctx context.Context, name, resourceType string, gvr schema.GroupVersionResource) error {
	a.logger.WithField(resourceType, name).Infof("Deleting ArgoCD %s", resourceType)

	err := a.client.Resource(gvr).Namespace(a.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			a.logger.WithField(resourceType, name).Infof("%s already deleted", resourceType)
			return nil
		}
		return fmt.Errorf("failed to delete %s %s: %w", resourceType, name, err)
	}

	a.logger.WithField(resourceType, name).Infof("Successfully deleted ArgoCD %s", resourceType)
	return nil
}

func (a *argoCDService) DeleteAppProject(ctx context.Context, name string) error {
	return a.deleteResource(ctx, name, "project", appProjectGVR)
}

func (a *argoCDService) CreateApplication(ctx context.Context, app *types.Application) error {
	a.logger.WithField("application", app.Name).Info("Creating ArgoCD Application")

	// Build Application resource - no kustomize needed since namespaces match
	application := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"name":      app.Name,
				"namespace": a.namespace,
				"labels": map[string]interface{}{
					"gitops.io/managed-by":         "gitops-registration-service",
					"app.kubernetes.io/managed-by": "gitops-registration-service",
					"gitops.io/tenant":             app.Destination.Namespace,
				},
			},
			"spec": map[string]interface{}{
				"project": app.Project,
				"source": map[string]interface{}{
					"repoURL":        app.Source.RepoURL,
					"targetRevision": app.Source.TargetRevision,
					"path":           app.Source.Path,
				},
				"destination": map[string]interface{}{
					"server":    app.Destination.Server,
					"namespace": app.Destination.Namespace,
				},
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{
						"prune":    true,
						"selfHeal": true,
					},
					"syncOptions": []interface{}{
						"CreateNamespace=false", // We create namespaces separately
						"PrunePropagationPolicy=background",
						"PruneLast=true",
					},
				},
			},
		},
	}

	_, err := a.client.Resource(applicationGVR).Namespace(a.namespace).Create(ctx, application, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			a.logger.WithField("application", app.Name).Info("Application already exists")
			return nil
		}
		return fmt.Errorf("failed to create Application %s: %w", app.Name, err)
	}

	a.logger.WithField("application", app.Name).Info("Successfully created ArgoCD Application")
	return nil
}

func (a *argoCDService) DeleteApplication(ctx context.Context, name string) error {
	return a.deleteResource(ctx, name, "Application", applicationGVR)
}

// GetApplicationStatus retrieves the status of an ArgoCD Application
func (a *argoCDService) GetApplicationStatus(ctx context.Context, name string) (*types.ApplicationStatus, error) {
	a.logger.WithField("application", name).Info("Getting ArgoCD Application status")

	app, err := a.client.Resource(applicationGVR).Namespace(a.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("application %s not found", name)
		}
		return nil, fmt.Errorf("failed to get Application %s: %w", name, err)
	}

	// Extract status information
	status := &types.ApplicationStatus{
		Phase:   "Unknown",
		Health:  "Unknown",
		Sync:    "Unknown",
		Message: "Application found",
	}

	// Try to extract health status
	if healthStatus, found, err := unstructured.NestedString(app.Object, "status", "health", "status"); err == nil && found {
		status.Health = healthStatus
	}

	// Try to extract sync status and last operation time
	if operationTime, found, err := unstructured.NestedString(app.Object, "status", "operationState", "finishedAt"); err == nil && found {
		if timestamp, err := time.Parse(time.RFC3339, operationTime); err == nil {
			status.LastSyncTime = timestamp
		}
	}

	return status, nil
}

func (a *argoCDService) HealthCheck(ctx context.Context) error {
	// Simple health check - try to list AppProjects
	_, err := a.client.Resource(appProjectGVR).Namespace(a.namespace).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("ArgoCD health check failed: %w", err)
	}
	return nil
}

// CheckAppProjectConflict checks if an AppProject exists for the given repository hash
func (a *argoCDService) CheckAppProjectConflict(ctx context.Context, repositoryHash string) (bool, error) {
	labelSelector := fmt.Sprintf("%s=%s", RepositoryHashLabel, repositoryHash)

	appProjects, err := a.client.Resource(appProjectGVR).Namespace(a.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return false, fmt.Errorf("failed to check AppProject conflict for repository hash %s: %w", repositoryHash, err)
	}

	exists := len(appProjects.Items) > 0
	if exists {
		a.logger.Infof("Found existing AppProject for repository hash %s", repositoryHash)
	}

	return exists, nil
}
