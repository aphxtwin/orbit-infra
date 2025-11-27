package models

import (
	"time"

	"github.com/google/uuid"
)

// ProvisioningLog represents an audit trail entry for tenant provisioning
// Maps to the 'provisioning_logs' table in PostgreSQL
type ProvisioningLog struct {
	ID           uuid.UUID `db:"id"`            // Primary key
	TenantID     uuid.UUID `db:"tenant_id"`     // Foreign key to tenants table
	Stage        string    `db:"stage"`         // Provisioning stage (e.g., "terraform_plan")
	Status       string    `db:"status"`        // Status: started, completed, failed
	Message      string    `db:"message"`       // Human-readable log message
	ErrorDetails []byte    `db:"error_details"` // JSONB error details (stored as bytes)
	Timestamp    time.Time `db:"timestamp"`     // When this log entry was created
}

// Provisioning stage constants
const (
	StageInitialization   = "initialization"
	StageSecretGeneration = "secret_generation"
	StageTerraformPlan    = "terraform_plan"
	StageTerraformApply   = "terraform_apply"
	StageConfigDeployment = "config_deployment"
	StageCompletion       = "completion"
	StageDeprovisioning   = "deprovisioning"
)

// Log status constants
const (
	LogStatusStarted   = "started"
	LogStatusCompleted = "completed"
	LogStatusFailed    = "failed"
)
