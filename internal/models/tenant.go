package models

import (
	"time"

	"github.com/google/uuid"
)

// Tenant represents a dealership tenant in the provisioning system
// Maps directly to the 'tenants' table in PostgreSQL
type Tenant struct {
	ID         uuid.UUID `db:"id"`          // Primary key
	Name       string    `db:"name"`        // Business name
	Subdomain  string    `db:"subdomain"`   // URL subdomain (unique)
	RexURL     string    `db:"rex_url"`     // Full REX instance URL (unique)
	AdminEmail string    `db:"admin_email"` // Contact email for admin
	Status     string    `db:"status"`      // Provisioning lifecycle status
	Region     string    `db:"region"`      // AWS region (e.g., "us-east-1")
	Config     []byte    `db:"config"`      // JSONB config data (stored as bytes in Go)
	CreatedAt  time.Time `db:"created_at"`  // When record was created
	UpdatedAt  time.Time `db:"updated_at"`  // Last modification time

	// Security: BCrypt hashes (never plaintext)
	AdminPasswordHash  string `db:"admin_password_hash"`  // BCrypt hash of Odoo admin password
	DBPasswordHash     string `db:"db_password_hash"`     // BCrypt hash of database password
	JWTSecretHash      string `db:"jwt_secret_hash"`      // BCrypt hash of JWT secret
	WebhookSecretHash  string `db:"webhook_secret_hash"`  // BCrypt hash of webhook secret

	// Per-tenant database configuration
	DBName string `db:"db_name"` // Database name (odoo_{subdomain}_{env})
	DBUser string `db:"db_user"` // Database username (odoo_{subdomain})

	// Per-tenant Odoo host URL
	OdooHost string `db:"odoo_host"` // Odoo host ({subdomain}-odoo.bici-dev.com)
}

// Tenant status constants matching database values
const (
	TenantStatusPending      = "pending"
	TenantStatusProvisioning = "provisioning"
	TenantStatusActive       = "active"
	TenantStatusFailed       = "failed"
	TenantStatusSuspended    = "suspended"
	TenantStatusDestroying   = "destroying"
	TenantStatusDestroyed    = "destroyed"
)

// IsActive returns true if the tenant is in active status
func (t *Tenant) IsActive() bool {
	return t.Status == TenantStatusActive
}

// IsPending returns true if the tenant is pending provisioning
func (t *Tenant) IsPending() bool {
	return t.Status == TenantStatusPending
}

// IsFailed returns true if provisioning failed
func (t *Tenant) IsFailed() bool {
	return t.Status == TenantStatusFailed
}

// CanProvision returns true if tenant can be provisioned
// Tenants can be provisioned if pending or if provisioning previously failed
func (t *Tenant) CanProvision() bool {
	return t.IsPending() || t.IsFailed()
}
