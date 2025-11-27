package models

import (
	"time"

	"github.com/google/uuid"
)

// TenantEnvironment represents an environment variable for a tenant
// The Value field contains encrypted data (encrypted at rest using AWS KMS)
// Maps to the 'tenant_environment' table in PostgreSQL
type TenantEnvironment struct {
	ID        uuid.UUID `db:"id"`         // Primary key
	TenantID  uuid.UUID `db:"tenant_id"`  // Foreign key to tenants table
	Key       string    `db:"key"`        // Environment variable name (e.g., "DB_PASSWORD")
	Value     string    `db:"value"`      // Encrypted value (base64-encoded ciphertext)
	IsSecret  bool      `db:"is_secret"`  // If true, hide from UI/logs
	CreatedAt time.Time `db:"created_at"` // When this env var was created
}

// DecryptedEnvironment holds a decrypted environment variable
// Used for working with env vars after decryption
// This struct is NOT mapped to database - it's for application use only
type DecryptedEnvironment struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Key       string
	Value     string    // Decrypted plaintext value
	IsSecret  bool
	CreatedAt time.Time
}
