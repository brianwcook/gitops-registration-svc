package services

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/konflux-ci/gitops-registration-service/internal/config"
)

// kubernetesService is the real implementation of KubernetesService
type kubernetesService struct {
	client kubernetes.Interface
	cfg    *config.Config
	logger *logrus.Logger
}

// NewKubernetesServiceReal creates a new real KubernetesService implementation
func NewKubernetesServiceReal(cfg *config.Config, logger *logrus.Logger) (KubernetesService, error) {
	// Create in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
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
	labels["gitops.io/managed-by"] = "gitops-registration-service"
	labels["app.kubernetes.io/managed-by"] = "gitops-registration-service"

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
				"gitops.io/managed-by":         "gitops-registration-service",
				"app.kubernetes.io/managed-by": "gitops-registration-service",
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
				"gitops.io/managed-by":         "gitops-registration-service",
				"app.kubernetes.io/managed-by": "gitops-registration-service",
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
