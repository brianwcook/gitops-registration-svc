package services

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

// KubernetesClientFactory creates Kubernetes clients for services
type KubernetesClientFactory interface {
	CreateConfig() (*rest.Config, error)
	CreateClientset(*rest.Config) (kubernetes.Interface, error)
}

// ArgoCDClientFactory creates ArgoCD clients for services
type ArgoCDClientFactory interface {
	CreateConfig() (*rest.Config, error)
	CreateDynamicClient(*rest.Config) (dynamic.Interface, error)
}

// Production implementations

// InClusterKubernetesFactory creates real Kubernetes clients using in-cluster config
type InClusterKubernetesFactory struct{}

func (f *InClusterKubernetesFactory) CreateConfig() (*rest.Config, error) {
	return rest.InClusterConfig()
}

func (f *InClusterKubernetesFactory) CreateClientset(config *rest.Config) (kubernetes.Interface, error) {
	return kubernetes.NewForConfig(config)
}

// InClusterArgoCDFactory creates real ArgoCD clients using in-cluster config
type InClusterArgoCDFactory struct{}

func (f *InClusterArgoCDFactory) CreateConfig() (*rest.Config, error) {
	return rest.InClusterConfig()
}

func (f *InClusterArgoCDFactory) CreateDynamicClient(config *rest.Config) (dynamic.Interface, error) {
	return dynamic.NewForConfig(config)
}

// Test implementations

// TestKubernetesFactory creates fake Kubernetes clients for testing
type TestKubernetesFactory struct {
	Client kubernetes.Interface
	Config *rest.Config
	Error  error // Error to return from CreateConfig or CreateClientset
}

func (f *TestKubernetesFactory) CreateConfig() (*rest.Config, error) {
	if f.Error != nil {
		return nil, f.Error
	}
	if f.Config != nil {
		return f.Config, nil
	}
	// Return a basic config for testing
	return &rest.Config{Host: "https://test-cluster"}, nil
}

func (f *TestKubernetesFactory) CreateClientset(config *rest.Config) (kubernetes.Interface, error) {
	if f.Error != nil {
		return nil, f.Error
	}
	if f.Client != nil {
		return f.Client, nil
	}
	// Return a fake client with no pre-existing objects
	return fake.NewSimpleClientset(), nil
}

// TestArgoCDFactory creates fake ArgoCD clients for testing
type TestArgoCDFactory struct {
	Client dynamic.Interface
	Config *rest.Config
	Error  error           // Error to return from CreateConfig or CreateDynamicClient
	Scheme *runtime.Scheme // Optional scheme for fake client
}

func (f *TestArgoCDFactory) CreateConfig() (*rest.Config, error) {
	if f.Error != nil {
		return nil, f.Error
	}
	if f.Config != nil {
		return f.Config, nil
	}
	// Return a basic config for testing
	return &rest.Config{Host: "https://test-cluster"}, nil
}

func (f *TestArgoCDFactory) CreateDynamicClient(config *rest.Config) (dynamic.Interface, error) {
	if f.Error != nil {
		return nil, f.Error
	}
	if f.Client != nil {
		return f.Client, nil
	}
	// Return a fake dynamic client
	scheme := f.Scheme
	if scheme == nil {
		scheme = runtime.NewScheme()
	}
	return fakedynamic.NewSimpleDynamicClient(scheme), nil
}

// Helper functions for creating pre-configured test factories

// NewTestKubernetesFactory creates a test factory with a fake Kubernetes client
func NewTestKubernetesFactory() *TestKubernetesFactory {
	return &TestKubernetesFactory{
		Client: fake.NewSimpleClientset(),
		Config: &rest.Config{Host: "https://test-cluster"},
	}
}

// NewTestArgoCDFactory creates a test factory with a fake ArgoCD client
func NewTestArgoCDFactory() *TestArgoCDFactory {
	scheme := runtime.NewScheme()
	return &TestArgoCDFactory{
		Client: fakedynamic.NewSimpleDynamicClient(scheme),
		Config: &rest.Config{Host: "https://test-cluster"},
		Scheme: scheme,
	}
}

// NewErrorKubernetesFactory creates a test factory that returns errors
func NewErrorKubernetesFactory(err error) *TestKubernetesFactory {
	return &TestKubernetesFactory{
		Error: err,
	}
}

// NewErrorArgoCDFactory creates a test factory that returns errors
func NewErrorArgoCDFactory(err error) *TestArgoCDFactory {
	return &TestArgoCDFactory{
		Error: err,
	}
}
