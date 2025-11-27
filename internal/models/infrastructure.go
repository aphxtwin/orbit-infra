package models

import (
	"time"

	"github.com/google/uuid"
)

// InfrastructureState stores Terraform outputs and AWS resource identifiers
// Maps to the 'infrastructure_state' table in PostgreSQL
type InfrastructureState struct {
	ID                 uuid.UUID `db:"id"`                   // Primary key
	TenantID           uuid.UUID `db:"tenant_id"`            // Foreign key to tenants table
	TerraformWorkspace string    `db:"terraform_workspace"`  // Terraform Cloud workspace name
	LastApplyID        *string   `db:"last_apply_id"`        // Last Terraform apply ID (nullable)
	LastRunID          *string   `db:"last_run_id"`          // Last Terraform run ID (nullable)
	EC2InstanceID      *string   `db:"ec2_instance_id"`      // AWS EC2 instance ID (nullable)
	EC2PublicIP        *string   `db:"ec2_public_ip"`        // EC2 public IP address (nullable)
	RDSEndpoint        *string   `db:"rds_endpoint"`         // RDS database endpoint (nullable)
	S3BackupBucket     *string   `db:"s3_backup_bucket"`     // S3 bucket name for backups (nullable)
	Outputs            []byte    `db:"outputs"`              // Full Terraform outputs as JSONB (stored as bytes)
	CreatedAt          time.Time `db:"created_at"`           // When this record was created
	UpdatedAt          time.Time `db:"updated_at"`           // Last modification time
}
// TableName specifies the database table name for GORM