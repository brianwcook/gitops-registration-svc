package services

import (
	"context"
	"fmt"

	"github.com/konflux-ci/gitops-registration-service/internal/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Constants for commonly used strings
const (
	GitOpsRegistrationService = "gitops-registration-service"
)

// kubernetesService is the real implementation of KubernetesService
type kubernetesService struct {
	client kubernetes.Interface
	cfg    *config.Config
	logger *logrus.Logger
}

// NewKubernetesServiceReal creates a new real KubernetesService implementation
func NewKubernetesServiceReal(cfg *config.Config, logger *logrus.Logger) (KubernetesService, error) {
	factory := &InClusterKubernetesFactory{}
	return NewKubernetesServiceWithFactory(cfg, logger, factory)
}

// NewKubernetesServiceWithFactory creates a KubernetesService using the provided factory
func NewKubernetesServiceWithFactory(cfg *config.Config, logger *logrus.Logger, factory KubernetesClientFactory) (KubernetesService, error) {
	// Create config using factory
	config, err := factory.CreateConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	// Create clientset using factory
	clientset, err := factory.CreateClientset(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &kubernetesService{
		client: clientset,
		cfg:    cfg,
		logger: logger,
	}, nil
}

func (k *kubernetesService) HealthCheck(ctx context.Context) error {
	// Check if we can reach the Kubernetes API
	_, err := k.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("kubernetes api health check failed: %w", err)
	}
	return nil
}

func (k *kubernetesService) CreateNamespace(ctx context.Context, name string, labels map[string]string) error {
	k.logger.WithField("namespace", name).Info("Creating namespace")

	// Set up default labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["gitops.io/managed-by"] = GitOpsRegistrationService
	labels["app.kubernetes.io/managed-by"] = GitOpsRegistrationService

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}

	_, err := k.client.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			k.logger.WithField("namespace", name).Info("Namespace already exists")
			return nil
		}
		return fmt.Errorf("failed to create namespace %s: %w", name, err)
	}

	k.logger.WithField("namespace", name).Info("Successfully created namespace")
	return nil
}

func (k *kubernetesService) CreateNamespaceWithMetadata(ctx context.Context, name string, labels, annotations map[string]string) error {
	k.logger.WithField("namespace", name).Info("Creating namespace with metadata")

	// Set up default labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["gitops.io/managed-by"] = GitOpsRegistrationService
	labels["app.kubernetes.io/managed-by"] = GitOpsRegistrationService

	// Set up annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
	}

	_, err := k.client.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			k.logger.WithField("namespace", name).Info("Namespace already exists")
			return nil
		}
		return fmt.Errorf("failed to create namespace %s: %w", name, err)
	}

	k.logger.WithFields(logrus.Fields{
		"namespace":   name,
		"labels":      labels,
		"annotations": annotations,
	}).Info("Successfully created namespace with metadata")
	return nil
}

func (k *kubernetesService) DeleteNamespace(ctx context.Context, name string) error {
	k.logger.WithField("namespace", name).Info("Deleting namespace")

	err := k.client.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			k.logger.WithField("namespace", name).Info("Namespace already deleted")
			return nil
		}
		return fmt.Errorf("failed to delete namespace %s: %w", name, err)
	}

	k.logger.WithField("namespace", name).Info("Successfully deleted namespace")
	return nil
}

func (k *kubernetesService) UpdateNamespaceLabels(ctx context.Context, name string, labels map[string]string) error {
	k.logger.WithField("namespace", name).Info("Updating namespace labels")

	// Get the current namespace
	namespace, err := k.client.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get namespace %s: %w", name, err)
	}

	// Initialize labels if nil
	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
	}

	// Merge the new labels with existing ones
	for key, value := range labels {
		namespace.Labels[key] = value
	}

	// Update the namespace
	_, err = k.client.CoreV1().Namespaces().Update(ctx, namespace, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update namespace %s labels: %w", name, err)
	}

	k.logger.WithFields(logrus.Fields{
		"namespace": name,
		"labels":    labels,
	}).Info("Successfully updated namespace labels")
	return nil
}

func (k *kubernetesService) UpdateNamespaceMetadata(ctx context.Context, name string, labels, annotations map[string]string) error {
	k.logger.WithField("namespace", name).Info("Updating namespace metadata")

	// Get the current namespace
	namespace, err := k.client.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get namespace %s: %w", name, err)
	}

	// Initialize labels and annotations if nil
	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
	}
	if namespace.Annotations == nil {
		namespace.Annotations = make(map[string]string)
	}

	// Merge the new labels with existing ones
	for key, value := range labels {
		namespace.Labels[key] = value
	}

	// Merge the new annotations with existing ones
	for key, value := range annotations {
		namespace.Annotations[key] = value
	}

	// Update the namespace
	_, err = k.client.CoreV1().Namespaces().Update(ctx, namespace, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update namespace %s metadata: %w", name, err)
	}

	k.logger.WithFields(logrus.Fields{
		"namespace":   name,
		"labels":      labels,
		"annotations": annotations,
	}).Info("Successfully updated namespace metadata")
	return nil
}

func (k *kubernetesService) NamespaceExists(ctx context.Context, name string) (bool, error) {
	_, err := k.client.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check namespace existence: %w", err)
	}
	return true, nil
}

func (k *kubernetesService) CountNamespaces(ctx context.Context) (int, error) {
	namespaces, err := k.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to list namespaces: %w", err)
	}
	return len(namespaces.Items), nil
}

func (k *kubernetesService) CreateServiceAccount(ctx context.Context, namespace, name string) error {
	k.logger.WithFields(logrus.Fields{
		"namespace": namespace,
		"name":      name,
	}).Info("Creating service account")

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"gitops.io/managed-by":         GitOpsRegistrationService,
				"app.kubernetes.io/managed-by": GitOpsRegistrationService,
				"gitops.io/tenant":             namespace,
			},
		},
	}

	_, err := k.client.CoreV1().ServiceAccounts(namespace).Create(ctx, serviceAccount, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			k.logger.WithFields(logrus.Fields{
				"namespace": namespace,
				"name":      name,
			}).Info("Service account already exists")
			return nil
		}
		return fmt.Errorf("failed to create service account %s in namespace %s: %w", name, namespace, err)
	}

	k.logger.WithFields(logrus.Fields{
		"namespace": namespace,
		"name":      name,
	}).Info("Successfully created service account")
	return nil
}

func (k *kubernetesService) CreateRoleBinding(ctx context.Context, namespace, name, role, serviceAccount string) error {
	k.logger.WithFields(logrus.Fields{
		"namespace":      namespace,
		"name":           name,
		"role":           role,
		"serviceAccount": serviceAccount,
	}).Info("Creating role binding")

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"gitops.io/managed-by":         GitOpsRegistrationService,
				"app.kubernetes.io/managed-by": GitOpsRegistrationService,
				"gitops.io/tenant":             namespace,
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccount,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     role,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	_, err := k.client.RbacV1().RoleBindings(namespace).Create(ctx, roleBinding, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			k.logger.WithFields(logrus.Fields{
				"namespace":      namespace,
				"name":           name,
				"role":           role,
				"serviceAccount": serviceAccount,
			}).Info("Role binding already exists")
			return nil
		}
		return fmt.Errorf("failed to create role binding %s in namespace %s: %w", name, namespace, err)
	}

	k.logger.WithFields(logrus.Fields{
		"namespace":      namespace,
		"name":           name,
		"role":           role,
		"serviceAccount": serviceAccount,
	}).Info("Successfully created role binding")
	return nil
}

// ValidateClusterRole validates a ClusterRole and returns security warnings
func (k *kubernetesService) ValidateClusterRole(ctx context.Context, name string) (*ClusterRoleValidation, error) {
	validation := &ClusterRoleValidation{
		Exists:        false,
		Warnings:      []string{},
		ResourceTypes: []string{},
	}

	clusterRole, err := k.client.RbacV1().ClusterRoles().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return validation, nil // Exists remains false
		}
		return nil, fmt.Errorf("failed to get ClusterRole %s: %w", name, err)
	}

	validation.Exists = true

	// Analyze rules for security issues
	for _, rule := range clusterRole.Rules {
		k.checkClusterAdminPermissions(rule, validation)
		k.checkNamespaceSpanningPermissions(rule, validation)
		k.checkClusterScopedPermissions(rule, validation)
		validation.ResourceTypes = append(validation.ResourceTypes, rule.Resources...)
	}

	return validation, nil
}

// checkClusterAdminPermissions checks for cluster-admin level permissions
func (k *kubernetesService) checkClusterAdminPermissions(rule rbacv1.PolicyRule, validation *ClusterRoleValidation) {
	if containsAll(rule.Verbs, []string{"*"}) && containsAll(rule.Resources, []string{"*"}) {
		validation.HasClusterAdmin = true
		validation.Warnings = append(validation.Warnings, "ClusterRole has cluster-admin level permissions (*/* resources)")
	}
}

// checkNamespaceSpanningPermissions checks for namespace-spanning permissions
func (k *kubernetesService) checkNamespaceSpanningPermissions(rule rbacv1.PolicyRule, validation *ClusterRoleValidation) {
	if !contains(rule.Verbs, "list") && !contains(rule.Verbs, "watch") {
		return
	}

	for _, resource := range rule.Resources {
		if resource == "namespaces" || resource == "*" {
			validation.HasNamespaceSpanning = true
			validation.Warnings = append(validation.Warnings, "ClusterRole can list/watch across namespaces")
			break
		}
	}
}

// checkClusterScopedPermissions checks for cluster-scoped resource modification
func (k *kubernetesService) checkClusterScopedPermissions(rule rbacv1.PolicyRule, validation *ClusterRoleValidation) {
	clusterScopedResources := []string{"nodes", "namespaces", "clusterroles", "clusterrolebindings", "persistentvolumes"}

	modifyingVerbs := []string{"create", "update", "delete", "patch"}
	hasModifyingVerb := false
	for _, verb := range modifyingVerbs {
		if contains(rule.Verbs, verb) {
			hasModifyingVerb = true
			break
		}
	}

	if !hasModifyingVerb {
		return
	}

	for _, resource := range rule.Resources {
		if contains(clusterScopedResources, resource) || resource == "*" {
			validation.HasClusterScoped = true
			validation.Warnings = append(validation.Warnings, fmt.Sprintf("ClusterRole can modify cluster-scoped resource: %s", resource))
		}
	}
}

// CreateServiceAccountWithGenerateName creates a service account with generated name
func (k *kubernetesService) CreateServiceAccountWithGenerateName(ctx context.Context, namespace, baseName string) (string, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: baseName + "-",
			Namespace:    namespace,
			Labels: map[string]string{
				"gitops.io/managed-by": GitOpsRegistrationService,
				"gitops.io/purpose":    "impersonation",
			},
		},
	}

	created, err := k.client.CoreV1().ServiceAccounts(namespace).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create service account in namespace %s: %w", namespace, err)
	}

	k.logger.Infof("Created service account %s in namespace %s", created.Name, namespace)
	return created.Name, nil
}

// CreateRoleBindingForServiceAccount creates a RoleBinding binding a ClusterRole to a ServiceAccount
func (k *kubernetesService) CreateRoleBindingForServiceAccount(ctx context.Context, namespace, name, clusterRole, serviceAccountName string) error {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"gitops.io/managed-by": GitOpsRegistrationService,
				"gitops.io/purpose":    "impersonation",
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole,
		},
	}

	_, err := k.client.RbacV1().RoleBindings(namespace).Create(ctx, roleBinding, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create RoleBinding %s in namespace %s: %w", name, namespace, err)
	}

	k.logger.Infof("Created RoleBinding %s in namespace %s", name, namespace)
	return nil
}

// CheckAppProjectConflict checks if an AppProject exists for the given repository hash
func (k *kubernetesService) CheckAppProjectConflict(ctx context.Context, repositoryHash string) (bool, error) {
	// This is a placeholder - the actual implementation would use ArgoCD client
	// to check for AppProjects with the repository hash label
	// For now, we'll implement this in the ArgoCD service
	return false, nil
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item || s == "*" {
			return true
		}
	}
	return false
}

func containsAll(slice, items []string) bool {
	for _, item := range items {
		if !contains(slice, item) {
			return false
		}
	}
	return true
}
