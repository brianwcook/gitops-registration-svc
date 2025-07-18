# Testability Analysis & Refactoring Recommendations

## 🔍 Current Architecture Issues

### 1. **Hard Dependencies on External Services**

**Problem**: Both `NewKubernetesServiceReal` and `NewArgoCDServiceReal` directly call `rest.InClusterConfig()`, making them impossible to unit test outside a Kubernetes cluster.

```go
// Current - NOT testable
func NewKubernetesServiceReal(cfg *config.Config, logger *logrus.Logger) (KubernetesService, error) {
    config, err := rest.InClusterConfig()  // ❌ Hard dependency
    if err != nil {
        return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
    }
    clientset, err := kubernetes.NewForConfig(config)  // ❌ Hard dependency
    // ...
}
```

### 2. **Tight Coupling Between Services**

**Problem**: The `Services.New()` function creates everything at once with no way to inject test doubles.

```go
// Current - Tightly coupled
func New(cfg *config.Config, logger *logrus.Logger) (*Services, error) {
    k8sService, err := NewKubernetesServiceReal(cfg, logger)        // ❌ Hard-coded
    argoCDService, err := NewArgoCDServiceReal(cfg, logger)        // ❌ Hard-coded
    registrationService := NewRegistrationServiceReal(cfg, k8sService, argoCDService, logger)
    // ...
}
```

### 3. **Mixed Abstraction Levels**

**Problem**: Business logic (registration workflows) is mixed with infrastructure concerns (Kubernetes API calls).

### 4. **No Interface for External Dependencies**

**Problem**: No abstraction layer between our code and Kubernetes/ArgoCD clients.

## 🎯 Refactoring Recommendations

### **Option A: Dependency Injection with Client Factories (Recommended)**

**Benefits**: Easy to test, minimal code changes, maintains existing interfaces.

#### Step 1: Create Client Factory Interfaces

```go
// internal/services/factories.go
package services

import (
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/rest"
)

type KubernetesClientFactory interface {
    CreateConfig() (*rest.Config, error)
    CreateClientset(*rest.Config) (kubernetes.Interface, error)
}

type ArgoCDClientFactory interface {
    CreateConfig() (*rest.Config, error)
    CreateDynamicClient(*rest.Config) (dynamic.Interface, error)
}

// Production implementations
type InClusterKubernetesFactory struct{}
func (f *InClusterKubernetesFactory) CreateConfig() (*rest.Config, error) {
    return rest.InClusterConfig()
}
func (f *InClusterKubernetesFactory) CreateClientset(config *rest.Config) (kubernetes.Interface, error) {
    return kubernetes.NewForConfig(config)
}

// Test implementations
type TestKubernetesFactory struct {
    Client kubernetes.Interface
}
func (f *TestKubernetesFactory) CreateConfig() (*rest.Config, error) {
    return &rest.Config{}, nil
}
func (f *TestKubernetesFactory) CreateClientset(config *rest.Config) (kubernetes.Interface, error) {
    return f.Client, nil
}
```

#### Step 2: Refactor Service Constructors

```go
// Updated constructors - TESTABLE
func NewKubernetesServiceReal(cfg *config.Config, logger *logrus.Logger, factory KubernetesClientFactory) (KubernetesService, error) {
    config, err := factory.CreateConfig()
    if err != nil {
        return nil, fmt.Errorf("failed to create config: %w", err)
    }
    
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
```

#### Step 3: Update Services Constructor

```go
func New(cfg *config.Config, logger *logrus.Logger) (*Services, error) {
    return NewWithFactories(cfg, logger, &InClusterKubernetesFactory{}, &InClusterArgoCDFactory{})
}

func NewWithFactories(cfg *config.Config, logger *logrus.Logger, k8sFactory KubernetesClientFactory, argoCDFactory ArgoCDClientFactory) (*Services, error) {
    k8sService, err := NewKubernetesServiceReal(cfg, logger, k8sFactory)
    if err != nil {
        return nil, fmt.Errorf("failed to create kubernetes service: %w", err)
    }
    
    argoCDService, err := NewArgoCDServiceReal(cfg, logger, argoCDFactory)
    if err != nil {
        return nil, fmt.Errorf("failed to create argocd service: %w", err)
    }
    
    // ...rest unchanged
}
```

### **Option B: Extract Business Logic from Infrastructure**

**Benefits**: Cleanest separation, easiest to test business logic, follows domain-driven design.

#### Step 1: Create Domain Services

```go
// internal/domain/registration.go
package domain

import (
    "context"
    "github.com/konflux-ci/gitops-registration-service/internal/types"
)

// Pure business logic - no external dependencies
type RegistrationOrchestrator struct {
    logger *logrus.Logger
}

func (r *RegistrationOrchestrator) ValidateRegistrationRequest(req *types.RegistrationRequest) error {
    if req.Namespace == "" {
        return fmt.Errorf("namespace is required")
    }
    if req.Repository.URL == "" {
        return fmt.Errorf("repository URL is required")
    }
    return nil
}

func (r *RegistrationOrchestrator) BuildNamespaceMetadata(req *types.RegistrationRequest, registrationID string) (map[string]string, map[string]string) {
    repoHash := GenerateRepositoryHash(req.Repository.URL)
    repoDomain := extractRepositoryDomain(req.Repository.URL)
    
    labels := map[string]string{
        "gitops.io/registration-id":    registrationID[:8],
        "gitops.io/repository-hash":    repoHash,
        "gitops.io/repository-domain":  repoDomain,
        "gitops.io/managed-by":         "gitops-registration-service",
        "app.kubernetes.io/managed-by": "gitops-registration-service",
    }
    
    annotations := map[string]string{
        "gitops.io/repository-url":    req.Repository.URL,
        "gitops.io/repository-branch": req.Repository.Branch,
        "gitops.io/registration-id":   registrationID,
    }
    
    return labels, annotations
}
```

#### Step 2: Testable Registration Service

```go
// Easily testable with mocked infrastructure
type registrationService struct {
    orchestrator *domain.RegistrationOrchestrator
    k8s          KubernetesService
    argocd       ArgoCDService
    logger       *logrus.Logger
}

func (r *registrationService) CreateRegistration(ctx context.Context, req *types.RegistrationRequest) (*types.Registration, error) {
    // Pure business logic - easily testable
    if err := r.orchestrator.ValidateRegistrationRequest(req); err != nil {
        return nil, err
    }
    
    registrationID := uuid.New().String()
    registration := r.orchestrator.BuildRegistrationRecord(registrationID, req)
    
    // Infrastructure calls - mockable
    labels, annotations := r.orchestrator.BuildNamespaceMetadata(req, registrationID)
    if err := r.k8s.CreateNamespaceWithMetadata(ctx, req.Namespace, labels, annotations); err != nil {
        return nil, err
    }
    
    // Continue with other infrastructure calls...
    return registration, nil
}
```

### **Option C: Configuration-Based Testing (Quickest Win)**

**Benefits**: Minimal changes, immediate testability improvement.

```go
// Add test mode to config
type Config struct {
    TestMode bool `yaml:"test_mode" env:"TEST_MODE"`
    // ...existing fields
}

func NewKubernetesServiceReal(cfg *config.Config, logger *logrus.Logger) (KubernetesService, error) {
    if cfg.TestMode {
        // Return stub implementation for testing
        return &kubernetesServiceStub{logger: logger}, nil
    }
    
    // Production implementation
    config, err := rest.InClusterConfig()
    // ...rest unchanged
}
```

## 🚀 Implementation Plan

### **Phase 1: Quick Wins (Recommended Start)**

1. **Fix linter errors** in test files
2. **Add cmd/server tests** using Option C approach
3. **Extract utility functions** to separate testable files
4. **Add more business logic tests** using existing mock patterns

### **Phase 2: Structural Improvements**

1. **Implement Option A** (Client Factories) for new features
2. **Gradually refactor existing services** to use factories
3. **Extract domain logic** (Option B) for complex business workflows

### **Phase 3: Long-term Architecture**

1. **Full domain-driven design** separation
2. **Event-driven architecture** for better testability
3. **Integration test suite** with real Kubernetes

## 📊 Expected Coverage Improvement

| Phase | Current | Target | Key Changes |
|-------|---------|--------|-------------|
| Phase 1 | 47.4% | 65% | Fix tests, add cmd/server tests |
| Phase 2 | 65% | 80% | Client factories, business logic extraction |
| Phase 3 | 80% | 90%+ | Full domain separation |

## ✅ Immediate Next Steps

1. **Fix current test compilation errors**
2. **Add cmd/server tests with test mode**
3. **Create client factory interfaces**
4. **Refactor one service (start with simplest) as proof of concept**

This approach gives you immediate coverage improvements while setting up for long-term architectural improvements. 