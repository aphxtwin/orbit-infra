package api

import "time"

// CreateTenantRequest represents the request to create and provision a new tenant
type CreateTenantRequest struct {
	Name        string `json:"name"`
	Subdomain   string `json:"subdomain"`
	Environment string `json:"environment"` // staging, production
	Region      string `json:"region"`
	AdminEmail  string `json:"admin_email,omitempty"`

	// Infrastructure configuration
	DatabaseInstanceClass string `json:"database_instance_class,omitempty"` // defaults to db.t3.micro
	StorageGB             int    `json:"storage_gb,omitempty"`              // defaults to 20
}

// CreateTenantResponse represents the response after initiating tenant provisioning
type CreateTenantResponse struct {
	TenantID             string    `json:"tenant_id"`
	Status               string    `json:"status"`
	ProvisioningStarted  bool      `json:"provisioning_started"`
	CreatedAt            time.Time `json:"created_at"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}