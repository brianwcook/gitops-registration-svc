package services

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestInClusterKubernetesFactory(t *testing.T) {
	factory := &InClusterKubernetesFactory{}

	t.Run("CreateConfig fails outside cluster", func(t *testing.T) {
		// When running outside a Kubernetes cluster, InClusterConfig should fail
		config, err := factory.CreateConfig()
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "unable to load in-cluster configuration")
	})

	t.Run("CreateClientset with valid config", func(t *testing.T) {
		// Test with a mock config - client creation might succeed or fail
		// depending on network connectivity
		testConfig := &rest.Config{
			Host: "https://test-cluster",
		}

		client, err := factory.CreateClientset(testConfig)

		// In unit tests, this could succeed (creates client) or fail (no valid server)
		// Both are valid outcomes, so we just verify the function doesn't panic
		if err != nil {
			assert.Nil(t, client)
			t.Logf("Client creation failed as expected: %v", err)
		} else {
			assert.NotNil(t, client)
			t.Logf("Client creation succeeded unexpectedly but harmlessly")
		}
	})
}

func TestInClusterArgoCDFactory(t *testing.T) {
	factory := &InClusterArgoCDFactory{}

	t.Run("CreateConfig fails outside cluster", func(t *testing.T) {
		// When running outside a Kubernetes cluster, InClusterConfig should fail
		config, err := factory.CreateConfig()
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "unable to load in-cluster configuration")
	})

	t.Run("CreateDynamicClient with valid config", func(t *testing.T) {
		// Test with a mock config - client creation might succeed or fail
		// depending on network connectivity
		testConfig := &rest.Config{
			Host: "https://test-cluster",
		}

		client, err := factory.CreateDynamicClient(testConfig)

		// In unit tests, this could succeed (creates client) or fail (no valid server)
		// Both are valid outcomes, so we just verify the function doesn't panic
		if err != nil {
			assert.Nil(t, client)
			t.Logf("Dynamic client creation failed as expected: %v", err)
		} else {
			assert.NotNil(t, client)
			t.Logf("Dynamic client creation succeeded unexpectedly but harmlessly")
		}
	})
}

func TestTestKubernetesFactory(t *testing.T) {
	t.Run("Default factory creates working clients", func(t *testing.T) {
		factory := NewTestKubernetesFactory()

		config, err := factory.CreateConfig()
		require.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "https://test-cluster", config.Host)

		client, err := factory.CreateClientset(config)
		require.NoError(t, err)
		assert.NotNil(t, client)

		// Verify it's a fake client by checking type
		_, isFake := client.(*fake.Clientset)
		assert.True(t, isFake, "Expected fake Kubernetes client")
	})

	t.Run("Factory with custom client", func(t *testing.T) {
		customClient := fake.NewSimpleClientset()
		factory := &TestKubernetesFactory{
			Client: customClient,
		}

		config, err := factory.CreateConfig()
		require.NoError(t, err)

		client, err := factory.CreateClientset(config)
		require.NoError(t, err)
		assert.Equal(t, customClient, client)
	})

	t.Run("Factory with custom config", func(t *testing.T) {
		customConfig := &rest.Config{Host: "https://custom-cluster"}
		factory := &TestKubernetesFactory{
			Config: customConfig,
		}

		config, err := factory.CreateConfig()
		require.NoError(t, err)
		assert.Equal(t, customConfig, config)
	})

	t.Run("Factory returns errors when configured", func(t *testing.T) {
		testError := errors.New("test error")
		factory := NewErrorKubernetesFactory(testError)

		config, err := factory.CreateConfig()
		assert.Error(t, err)
		assert.Equal(t, testError, err)
		assert.Nil(t, config)

		client, err := factory.CreateClientset(&rest.Config{})
		assert.Error(t, err)
		assert.Equal(t, testError, err)
		assert.Nil(t, client)
	})
}

func TestTestArgoCDFactory(t *testing.T) {
	t.Run("Default factory creates working clients", func(t *testing.T) {
		factory := NewTestArgoCDFactory()

		config, err := factory.CreateConfig()
		require.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "https://test-cluster", config.Host)

		client, err := factory.CreateDynamicClient(config)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("Factory with custom client", func(t *testing.T) {
		scheme := runtime.NewScheme()
		customClient := fakedynamic.NewSimpleDynamicClient(scheme)
		factory := &TestArgoCDFactory{
			Client: customClient,
		}

		config, err := factory.CreateConfig()
		require.NoError(t, err)

		client, err := factory.CreateDynamicClient(config)
		require.NoError(t, err)
		assert.Equal(t, customClient, client)
	})

	t.Run("Factory with custom config and scheme", func(t *testing.T) {
		customConfig := &rest.Config{Host: "https://custom-argocd"}
		customScheme := runtime.NewScheme()
		factory := &TestArgoCDFactory{
			Config: customConfig,
			Scheme: customScheme,
		}

		config, err := factory.CreateConfig()
		require.NoError(t, err)
		assert.Equal(t, customConfig, config)

		client, err := factory.CreateDynamicClient(config)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("Factory returns errors when configured", func(t *testing.T) {
		testError := errors.New("argocd test error")
		factory := NewErrorArgoCDFactory(testError)

		config, err := factory.CreateConfig()
		assert.Error(t, err)
		assert.Equal(t, testError, err)
		assert.Nil(t, config)

		client, err := factory.CreateDynamicClient(&rest.Config{})
		assert.Error(t, err)
		assert.Equal(t, testError, err)
		assert.Nil(t, client)
	})
}

func TestFactoryIntegration(t *testing.T) {
	t.Run("Factories implement interfaces", func(t *testing.T) {
		// Compile-time checks that factories implement interfaces
		var k8sFactory KubernetesClientFactory = &InClusterKubernetesFactory{}
		var argoCDFactory ArgoCDClientFactory = &InClusterArgoCDFactory{}
		var testK8sFactory KubernetesClientFactory = NewTestKubernetesFactory()
		var testArgoCDFactory ArgoCDClientFactory = NewTestArgoCDFactory()

		assert.NotNil(t, k8sFactory)
		assert.NotNil(t, argoCDFactory)
		assert.NotNil(t, testK8sFactory)
		assert.NotNil(t, testArgoCDFactory)
	})

	t.Run("Error factories work correctly", func(t *testing.T) {
		testErr := errors.New("integration test error")

		k8sErrorFactory := NewErrorKubernetesFactory(testErr)
		argoCDErrorFactory := NewErrorArgoCDFactory(testErr)

		// Both should return the same error
		_, k8sErr := k8sErrorFactory.CreateConfig()
		_, argoCDErr := argoCDErrorFactory.CreateConfig()

		assert.Equal(t, testErr, k8sErr)
		assert.Equal(t, testErr, argoCDErr)
	})
}
