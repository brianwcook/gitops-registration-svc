package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/konflux-ci/gitops-registration-service/internal/services"
	"github.com/konflux-ci/gitops-registration-service/internal/types"
	"github.com/sirupsen/logrus"
)

// isNamespaceConflictError checks if the error is a namespace conflict error
func isNamespaceConflictError(err error) bool {
	return strings.Contains(err.Error(), "already exists")
}

// isRepositoryConflictError checks if the error is a repository conflict error
func isRepositoryConflictError(err error) bool {
	return strings.Contains(err.Error(), "already registered")
}

// RegistrationHandler handles registration-related HTTP requests
type RegistrationHandler struct {
	services *services.Services
	logger   *logrus.Logger
}

// NewRegistrationHandler creates a new registration handler
func NewRegistrationHandler(services *services.Services, logger *logrus.Logger) *RegistrationHandler {
	return &RegistrationHandler{
		services: services,
		logger:   logger,
	}
}

// CreateRegistration handles POST /api/v1/registrations
func (h *RegistrationHandler) CreateRegistration(w http.ResponseWriter, r *http.Request) {
	var req types.RegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, "INVALID_REQUEST", "Invalid JSON request body", http.StatusBadRequest)
		return
	}

	// Extract and validate user info (check authentication first)
	userInfo, err := h.extractUserInfo(r)
	if err != nil {
		h.writeErrorResponse(w, "AUTHENTICATION_REQUIRED", "Valid authentication required", http.StatusUnauthorized)
		return
	}

	// Validate request
	if validationErr := h.services.Registration.ValidateRegistration(r.Context(), &req); validationErr != nil {
		h.writeErrorResponse(w, "INVALID_REQUEST", validationErr.Error(), http.StatusBadRequest)
		return
	}

	// Check if new namespace registration is allowed
	if controlErr := h.services.RegistrationControl.IsNewNamespaceAllowed(r.Context()); controlErr != nil {
		h.writeErrorResponse(w, "REGISTRATION_DISABLED", controlErr.Error(), http.StatusForbidden)
		return
	}

	h.logger.WithField("user", userInfo.Username).Info("Creating new registration")

	// Create registration
	registration, err := h.services.Registration.CreateRegistration(r.Context(), &req)
	if err != nil {
		h.logger.WithError(err).Error("Failed to create registration")

		// Check for specific error types to return appropriate status codes
		if isNamespaceConflictError(err) {
			h.writeErrorResponse(w, "NAMESPACE_CONFLICT", err.Error(), http.StatusConflict)
			return
		}
		if isRepositoryConflictError(err) {
			h.writeErrorResponse(w, "REPOSITORY_CONFLICT", err.Error(), http.StatusConflict)
			return
		}

		h.writeErrorResponse(w, "REGISTRATION_FAILED", "Failed to create registration", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(registration); err != nil {
		h.logger.WithError(err).Error("Failed to encode registration response")
	}
}

// RegisterExistingNamespace handles POST /api/v1/registrations/existing
func (h *RegistrationHandler) RegisterExistingNamespace(w http.ResponseWriter, r *http.Request) {
	var req types.ExistingNamespaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, "INVALID_REQUEST", "Invalid JSON request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := h.services.Registration.ValidateExistingNamespaceRequest(r.Context(), &req); err != nil {
		h.writeErrorResponse(w, "INVALID_REQUEST", err.Error(), http.StatusBadRequest)
		return
	}

	// Extract and validate user info
	userInfo, err := h.extractUserInfo(r)
	if err != nil {
		h.writeErrorResponse(w, "AUTHENTICATION_REQUIRED", "Valid authentication required", http.StatusUnauthorized)
		return
	}

	// Validate user has access to the existing namespace
	authErr := h.services.Authorization.ValidateNamespaceAccess(r.Context(), userInfo, req.ExistingNamespace)
	if authErr != nil {
		h.logger.WithFields(logrus.Fields{
			"user":      userInfo.Username,
			"namespace": req.ExistingNamespace,
			"error":     authErr,
		}).Warn("Unauthorized namespace access attempt")
		h.writeErrorResponse(w, "INSUFFICIENT_PERMISSIONS",
			"Insufficient permissions for target namespace", http.StatusForbidden)
		return
	}

	// Register existing namespace
	registration, err := h.services.Registration.RegisterExistingNamespace(r.Context(), &req, userInfo)
	if err != nil {
		h.logger.WithError(err).Error("Failed to register existing namespace")
		h.writeErrorResponse(w, "REGISTRATION_FAILED",
			"Failed to register existing namespace", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(registration); err != nil {
		h.logger.WithError(err).Error("Failed to encode registration response")
	}
}

// ListRegistrations handles GET /api/v1/registrations
func (h *RegistrationHandler) ListRegistrations(w http.ResponseWriter, r *http.Request) {
	// Extract query parameters for filtering
	filters := make(map[string]string)
	if namespace := r.URL.Query().Get("namespace"); namespace != "" {
		filters["namespace"] = namespace
	}

	registrations, err := h.services.Registration.ListRegistrations(r.Context(), filters)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list registrations")
		h.writeErrorResponse(w, "LIST_FAILED", "Failed to list registrations", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(registrations); err != nil {
		h.logger.WithError(err).Error("Failed to encode registrations response")
	}
}

// GetRegistration handles GET /api/v1/registrations/{id}
func (h *RegistrationHandler) GetRegistration(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		h.writeErrorResponse(w, "INVALID_REQUEST", "Registration ID required", http.StatusBadRequest)
		return
	}

	registration, err := h.services.Registration.GetRegistration(r.Context(), id)
	if err != nil {
		h.writeErrorResponse(w, "NOT_FOUND", "Registration not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(registration); err != nil {
		h.logger.WithError(err).Error("Failed to encode registration response")
	}
}

// DeleteRegistration handles DELETE /api/v1/registrations/{id}
func (h *RegistrationHandler) DeleteRegistration(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		h.writeErrorResponse(w, "INVALID_REQUEST", "Registration ID required", http.StatusBadRequest)
		return
	}

	if err := h.services.Registration.DeleteRegistration(r.Context(), id); err != nil {
		h.logger.WithError(err).Error("Failed to delete registration")
		h.writeErrorResponse(w, "DELETE_FAILED", "Failed to delete registration", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetRegistrationStatus handles GET /api/v1/registrations/{id}/status
func (h *RegistrationHandler) GetRegistrationStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		h.writeErrorResponse(w, "INVALID_REQUEST", "Registration ID required", http.StatusBadRequest)
		return
	}

	registration, err := h.services.Registration.GetRegistration(r.Context(), id)
	if err != nil {
		h.writeErrorResponse(w, "NOT_FOUND", "Registration not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(registration.Status); err != nil {
		h.logger.WithError(err).Error("Failed to encode registration status response")
	}
}

// SyncRegistration handles POST /api/v1/registrations/{id}/sync
func (h *RegistrationHandler) SyncRegistration(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		h.writeErrorResponse(w, "INVALID_REQUEST", "Registration ID required", http.StatusBadRequest)
		return
	}

	// TODO: Implement sync trigger logic
	h.logger.WithField("id", id).Info("Sync triggered for registration (stub)")

	response := map[string]interface{}{
		"message": "Sync triggered successfully",
		"id":      id,
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.WithError(err).Error("Failed to encode sync response")
	}
}

// Helper methods

// extractUserInfo extracts user information from request context/headers
func (h *RegistrationHandler) extractUserInfo(r *http.Request) (*types.UserInfo, error) {
	// Extract Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, http.ErrNoCookie
	}

	// Extract Bearer token
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return nil, http.ErrNoCookie
	}

	return h.services.Authorization.ExtractUserInfo(r.Context(), token)
}

// writeErrorResponse writes a standardized error response
func (h *RegistrationHandler) writeErrorResponse(w http.ResponseWriter, errorCode, message string, statusCode int) {
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(types.ErrorResponse{
		Error:   errorCode,
		Message: message,
		Code:    statusCode,
	}); err != nil {
		h.logger.WithError(err).Error("Failed to encode error response")
	}
}
