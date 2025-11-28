package lambda

import "time"

// InvocationType specifies how to invoke the Lambda function
type InvocationType string

const (
	// InvocationTypeRequestResponse invokes synchronously and waits for response
	InvocationTypeRequestResponse InvocationType = "RequestResponse"
	// InvocationTypeEvent invokes asynchronously (fire and forget)
	InvocationTypeEvent InvocationType = "Event"
)

// ProvisioningRequest represents a tenant provisioning request
type ProvisioningRequest struct {
	TenantID    string                 `json:"tenant_id"`
	Environment string                 `json:"environment"` // staging, production
	Config      ProvisioningConfig     `json:"config"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ProvisioningConfig holds configuration for tenant infrastructure
type ProvisioningConfig struct {
	Region              string `json:"region"`
	DatabaseInstanceClass string `json:"database_instance_class"`
	StorageGB           int    `json:"storage_gb"`
}

// ProvisioningResponse represents the Lambda response for provisioning
type ProvisioningResponse struct {
	ExecutionID string                 `json:"execution_id"`
	Status      ProvisioningStatus     `json:"status"`
	Resources   *ProvisionedResources  `json:"resources,omitempty"`
	Error       *ErrorDetails          `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ProvisioningStatus represents the current status of a provisioning operation
type ProvisioningStatus string

const (
	StatusPending    ProvisioningStatus = "pending"
	StatusInProgress ProvisioningStatus = "in_progress"
	StatusCompleted  ProvisioningStatus = "completed"
	StatusFailed     ProvisioningStatus = "failed"
)

// ProvisionedResources contains details about provisioned AWS resources
type ProvisionedResources struct {
	RDSEndpoint string `json:"rds_endpoint,omitempty"`
	S3Bucket    string `json:"s3_bucket,omitempty"`
	VPCID       string `json:"vpc_id,omitempty"`
	SubnetIDs   []string `json:"subnet_ids,omitempty"`
}

// ErrorDetails provides detailed error information from Lambda
type ErrorDetails struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// DeletionRequest represents a tenant deletion request
type DeletionRequest struct {
	TenantID    string `json:"tenant_id"`
	Environment string `json:"environment"`
}

// DeletionResponse represents the Lambda response for deletion
type DeletionResponse struct {
	ExecutionID string             `json:"execution_id"`
	Status      ProvisioningStatus `json:"status"`
	Error       *ErrorDetails      `json:"error,omitempty"`
}

// StatusCheckRequest requests the current status of infrastructure
type StatusCheckRequest struct {
	TenantID    string `json:"tenant_id"`
	Environment string `json:"environment"`
}

// StatusCheckResponse provides the current infrastructure status
type StatusCheckResponse struct {
	Status    ProvisioningStatus    `json:"status"`
	Resources *ProvisionedResources `json:"resources,omitempty"`
	Error     *ErrorDetails         `json:"error,omitempty"`
	LastCheck time.Time             `json:"last_check"`
}

// InvocationResult wraps the result of a Lambda invocation
type InvocationResult struct {
	RequestID      string
	StatusCode     int32
	ExecutedVersion string
	LogResult      string // Base64 encoded logs (only if requested)
	Payload        []byte
	FunctionError  string
}