package handlers

import (
	"encoding/json"
	"net/http"

	"provisioning/internal/api"
	"provisioning/internal/orchestrator"
)

// TenantHandler handles HTTP requests for tenant operations
type TenantHandler struct {
	orchestrator *orchestrator.Orchestrator
}

// NewTenantHandler creates a new tenant handler
func NewTenantHandler(orch *orchestrator.Orchestrator) *TenantHandler {
	return &TenantHandler{
		orchestrator: orch,
	}
}

// CreateTenant handles POST /tenants requests
func (h *TenantHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req api.CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON", err.Error())
		return
	}

	// Call orchestrator to create tenant
	resp, err := h.orchestrator.CreateTenant(r.Context(), req)
	if err != nil {
		// Check if it's a validation error
		if isValidationError(err) {
			h.writeError(w, http.StatusBadRequest, "validation failed", err.Error())
			return
		}

		// Otherwise it's an internal error
		h.writeError(w, http.StatusInternalServerError, "failed to create tenant", err.Error())
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)
}

// writeError writes an error response
func (h *TenantHandler) writeError(w http.ResponseWriter, statusCode int, error string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(api.ErrorResponse{
		Error:   error,
		Message: message,
	})
}

// isValidationError checks if an error is a validation error
func isValidationError(err error) bool {
	// Simple check: if error message contains "validation failed", it's a validation error
	return err != nil && len(err.Error()) > 0 &&
		(contains(err.Error(), "validation failed") ||
		 contains(err.Error(), "is required") ||
		 contains(err.Error(), "must be"))
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) &&
		hasSubstring(s, substr))
}

// hasSubstring is a helper to check substring
func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
