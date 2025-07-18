package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/konflux-ci/gitops-registration-service/internal/config"
	"github.com/konflux-ci/gitops-registration-service/internal/handlers"
	"github.com/konflux-ci/gitops-registration-service/internal/services"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// Server represents the HTTP server
type Server struct {
	config   *config.Config
	logger   *logrus.Logger
	router   *chi.Mux
	server   *http.Server
	services *services.Services
}

// New creates a new server instance
func New(cfg *config.Config, logger *logrus.Logger) (*Server, error) {
	// Initialize services
	svc, err := services.New(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize services: %w", err)
	}

	// Validate impersonation configuration if enabled
	if cfg.Security.Impersonation.Enabled {
		logger.Infof("Impersonation is enabled, validating ClusterRole: %s", cfg.Security.Impersonation.ClusterRole)

		validation, err := svc.Kubernetes.ValidateClusterRole(context.Background(), cfg.Security.Impersonation.ClusterRole)
		if err != nil {
			return nil, fmt.Errorf("failed to validate ClusterRole %s: %w", cfg.Security.Impersonation.ClusterRole, err)
		}

		if !validation.Exists {
			return nil, fmt.Errorf("ClusterRole %s does not exist", cfg.Security.Impersonation.ClusterRole)
		}

		// Log security warnings
		if len(validation.Warnings) > 0 {
			logger.Warnf("ClusterRole %s security warnings:", cfg.Security.Impersonation.ClusterRole)
			for _, warning := range validation.Warnings {
				logger.Warnf("  - %s", warning)
			}
		}

		logger.Infof("ClusterRole %s validated successfully for impersonation", cfg.Security.Impersonation.ClusterRole)
	}

	// Create router
	router := chi.NewRouter()

	s := &Server{
		config:   cfg,
		logger:   logger,
		router:   router,
		services: svc,
	}

	// Setup middleware
	s.setupMiddleware()

	// Setup routes
	s.setupRoutes()

	// Create HTTP server
	s.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           router,
		ReadHeaderTimeout: 30 * time.Second, // Prevent Slowloris attacks
	}

	return s, nil
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	s.logger.WithField("port", s.config.Server.Port).Info("Starting HTTP server")

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		return s.server.Shutdown(ctx)
	case err := <-errChan:
		return err
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server")
	return s.server.Shutdown(ctx)
}

// setupMiddleware configures middleware for the router
func (s *Server) setupMiddleware() {
	// Request ID middleware
	s.router.Use(middleware.RequestID)

	// Structured logging middleware
	s.router.Use(middleware.RequestLogger(&middleware.DefaultLogFormatter{
		Logger:  s.logger,
		NoColor: true,
	}))

	// Recovery middleware
	s.router.Use(middleware.Recoverer)

	// Timeout middleware
	timeout, err := time.ParseDuration(s.config.Server.Timeout)
	if err != nil {
		s.logger.WithError(err).Warn("Invalid timeout duration, using default 30s")
		timeout = 30 * time.Second
	}
	s.router.Use(middleware.Timeout(timeout))

	// CORS middleware
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Content-Type middleware
	s.router.Use(middleware.SetHeader("Content-Type", "application/json"))
}

// setupRoutes configures API routes
func (s *Server) setupRoutes() {
	// Health check endpoints
	s.router.Get("/health/live", s.healthLive)
	s.router.Get("/health/ready", s.healthReady)

	// Metrics endpoint
	s.router.Handle("/metrics", promhttp.Handler())

	// API routes
	s.router.Route("/api/v1", func(r chi.Router) {
		// Registration handlers
		registrationHandler := handlers.NewRegistrationHandler(s.services, s.logger)

		r.Route("/registrations", func(r chi.Router) {
			r.Post("/", registrationHandler.CreateRegistration)
			r.Get("/", registrationHandler.ListRegistrations)
			r.Post("/existing", registrationHandler.RegisterExistingNamespace)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", registrationHandler.GetRegistration)
				r.Delete("/", registrationHandler.DeleteRegistration)
				r.Get("/status", registrationHandler.GetRegistrationStatus)
				r.Post("/sync", registrationHandler.SyncRegistration)
			})
		})

	})
}

// healthLive handles liveness probe requests
func (s *Server) healthLive(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "gitops-registration-service",
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.WithError(err).Error("Failed to encode health response")
	}
}

// healthReady handles readiness probe requests
func (s *Server) healthReady(w http.ResponseWriter, r *http.Request) {
	// Check dependencies
	if err := s.checkDependencies(r.Context()); err != nil {
		s.logger.WithError(err).Error("Readiness check failed")

		response := map[string]interface{}{
			"status":    "not ready",
			"error":     err.Error(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			s.logger.WithError(err).Error("Failed to encode error response")
		}
		return
	}

	response := map[string]interface{}{
		"status":    "ready",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "gitops-registration-service",
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.WithError(err).Error("Failed to encode ready response")
	}
}

// checkDependencies verifies that all required dependencies are available
func (s *Server) checkDependencies(ctx context.Context) error {
	// Check Kubernetes API connectivity
	if err := s.services.Kubernetes.HealthCheck(ctx); err != nil {
		return fmt.Errorf("kubernetes api unavailable: %w", err)
	}

	// Check ArgoCD connectivity
	if err := s.services.ArgoCD.HealthCheck(ctx); err != nil {
		return fmt.Errorf("argocd api unavailable: %w", err)
	}

	return nil
}
