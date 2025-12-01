# Lambda Client Documentation

## File Locations

```
/home/ivo/Desktop/Projects/Bici/orbit/provisioning/
├── internal/aws/lambda/
│   ├── client.go    # Lambda client implementation
│   └── types.go     # Request/response types
├── internal/models/ # Database models
│   ├── tenant.go
│   ├── environment.go
│   ├── infrastructure.go
│   └── log.go
└── docs/lambda/     # This documentation
    └── README.md
```

## Quick Start

### 1. Import the Package

```go
import "provisioning/internal/aws/lambda"
```

### 2. Create Client

```go
ctx := context.Background()

client, err := lambda.NewClient(ctx, lambda.Config{
    Region: "us-east-1",
})
if err != nil {
    log.Fatal(err)
}
```

### 3. Provision a Tenant

```go
req := lambda.ProvisioningRequest{
    TenantID:    "tenant-uuid",
    Environment: "production",
    Config: lambda.ProvisioningConfig{
        Region:                "us-east-1",
        DatabaseInstanceClass: "db.t3.micro",
        StorageGB:            20,
    },
}

resp, err := client.ProvisionTenant(ctx, "my-provisioning-lambda", req)
if err != nil {
    log.Fatal(err)
}

if resp.Status == lambda.StatusCompleted {
    log.Printf("✓ Provisioned! RDS: %s", resp.Resources.RDSEndpoint)
}
```

## Docker Environment

### Running in Docker Container

The controller runs in the `provisioning-controller` container:

```bash
# Exec into controller container
docker exec -it provisioning-controller sh

# Working directory is /controller
cd /controller

# Run Go code
go run cmd/controller/main.go
```

### Environment Variables in Docker

Set in `docker-compose.yml`:

```yaml
controller:
  environment:
    # AWS credentials
    AWS_REGION: us-east-1
    AWS_ACCESS_KEY_ID: ${AWS_ACCESS_KEY_ID}
    AWS_SECRET_ACCESS_KEY: ${AWS_SECRET_ACCESS_KEY}

    # Lambda function names
    PROVISIONING_LAMBDA_NAME: tenant-provisioning
    DELETION_LAMBDA_NAME: tenant-deletion
    STATUS_CHECK_LAMBDA_NAME: tenant-status
```

Or set locally:

```bash
export AWS_ACCESS_KEY_ID=your_key
export AWS_SECRET_ACCESS_KEY=your_secret
export AWS_REGION=us-east-1

docker-compose up -d
```

## Available Methods

### 1. ProvisionTenant

Synchronously provisions infrastructure for a new tenant.

**Location**: `internal/aws/lambda/client.go:95`

```go
resp, err := client.ProvisionTenant(ctx, functionName, lambda.ProvisioningRequest{
    TenantID:    "uuid-here",
    Environment: "production",
    Config: lambda.ProvisioningConfig{
        Region:                "us-east-1",
        DatabaseInstanceClass: "db.t3.micro",
        StorageGB:            20,
    },
    Metadata: map[string]interface{}{
        "subdomain": "acme",
        "admin_email": "admin@acme.com",
    },
})
```

**Response** (`internal/aws/lambda/types.go:31`):
```go
type ProvisioningResponse struct {
    ExecutionID string
    Status      ProvisioningStatus  // pending, in_progress, completed, failed
    Resources   *ProvisionedResources
    Error       *ErrorDetails
    Metadata    map[string]interface{}
}
```

### 2. DeleteTenant

Tears down tenant infrastructure.

**Location**: `internal/aws/lambda/client.go:123`

```go
resp, err := client.DeleteTenant(ctx, functionName, lambda.DeletionRequest{
    TenantID:    "uuid-here",
    Environment: "production",
})
```

### 3. CheckStatus

Queries current provisioning status.

**Location**: `internal/aws/lambda/client.go:151`

```go
resp, err := client.CheckStatus(ctx, functionName, lambda.StatusCheckRequest{
    TenantID:    "uuid-here",
    Environment: "production",
})

log.Printf("Status: %s", resp.Status)
log.Printf("Last check: %s", resp.LastCheck)
```

### 4. InvokeAsync

Fire-and-forget async invocation.

**Location**: `internal/aws/lambda/client.go:180`

```go
err := client.InvokeAsync(ctx, "cleanup-function", map[string]string{
    "tenant_id": "uuid-here",
    "action": "cleanup",
})
// Returns immediately, doesn't wait
```

## Type Definitions

All types are in `internal/aws/lambda/types.go`:

### Request Types

```go
// ProvisioningRequest (line 16)
type ProvisioningRequest struct {
    TenantID    string
    Environment string  // staging, production
    Config      ProvisioningConfig
    Metadata    map[string]interface{}
}

// DeletionRequest (line 65)
type DeletionRequest struct {
    TenantID    string
    Environment string
}

// StatusCheckRequest (line 78)
type StatusCheckRequest struct {
    TenantID    string
    Environment string
}
```

### Response Types

```go
// ProvisioningResponse (line 31)
type ProvisioningResponse struct {
    ExecutionID string
    Status      ProvisioningStatus
    Resources   *ProvisionedResources
    Error       *ErrorDetails
}

// ProvisionedResources (line 50)
type ProvisionedResources struct {
    RDSEndpoint string
    S3Bucket    string
    VPCID       string
    SubnetIDs   []string
}
```

### Status Constants

```go
// ProvisioningStatus (line 40-47)
const (
    StatusPending    = "pending"
    StatusInProgress = "in_progress"
    StatusCompleted  = "completed"
    StatusFailed     = "failed"
)
```

## Integration with Database Layer

The Lambda client works with the database models in `internal/models/`:

```go
import (
    "provisioning/internal/aws/lambda"
    "provisioning/internal/models"
    "provisioning/internal/db"
)

// 1. Create tenant in database
tenant := &models.Tenant{
    ID:         uuid.New(),
    Name:       "Acme Dealership",
    Subdomain:  "acme",
    Status:     models.TenantStatusPending,
    Region:     "us-east-1",
}

err := tenantRepo.Create(ctx, tenant)

// 2. Trigger Lambda provisioning
lambdaReq := lambda.ProvisioningRequest{
    TenantID:    tenant.ID.String(),
    Environment: "production",
    Config: lambda.ProvisioningConfig{
        Region:                tenant.Region,
        DatabaseInstanceClass: "db.t3.micro",
        StorageGB:            20,
    },
}

lambdaResp, err := lambdaClient.ProvisionTenant(ctx, "provisioning-lambda", lambdaReq)

// 3. Update tenant status
if lambdaResp.Status == lambda.StatusCompleted {
    tenantRepo.UpdateStatus(ctx, tenant.ID, models.TenantStatusActive)

    // 4. Store infrastructure state
    infraState := &models.InfrastructureState{
        TenantID:      tenant.ID,
        RDSEndpoint:   &lambdaResp.Resources.RDSEndpoint,
        S3BackupBucket: &lambdaResp.Resources.S3Bucket,
    }
    infraRepo.Create(ctx, infraState)
}
```

## AWS Setup

### IAM Permissions Required

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["lambda:InvokeFunction"],
      "Resource": "arn:aws:lambda:us-east-1:*:function:provisioning-*"
    }
  ]
}
```

### Credentials Loading Order

The client uses AWS SDK v2 credential chain:

1. **Environment variables** (Docker)
   ```bash
   AWS_ACCESS_KEY_ID=xxx
   AWS_SECRET_ACCESS_KEY=xxx
   AWS_REGION=us-east-1
   ```

2. **IAM Role** (EC2/ECS in production)
   - Attach role to EC2 instance
   - No credentials in code

3. **AWS Config files** (`~/.aws/credentials`)
   ```ini
   [default]
   aws_access_key_id = xxx
   aws_secret_access_key = xxx
   ```

## Error Handling

### Invocation Errors vs Function Errors

```go
resp, err := client.ProvisionTenant(ctx, "my-lambda", req)

// Invocation error (network, permissions, Lambda not found)
if err != nil {
    log.Printf("Failed to invoke Lambda: %v", err)
    return err
}

// Function error (Terraform failed, business logic error)
if resp.Status == lambda.StatusFailed {
    log.Printf("Provisioning failed: %s", resp.Error.Message)
    log.Printf("Error code: %s", resp.Error.Code)
    return fmt.Errorf("provisioning failed")
}

// Success
log.Printf("Provisioning completed: %s", resp.ExecutionID)
```

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `access denied` | Missing IAM permissions | Add `lambda:InvokeFunction` to policy |
| `function not found` | Wrong function name/region | Check function name and AWS_REGION |
| `timeout` | Lambda took too long | Increase context timeout |
| `status 200 but failed` | Terraform error | Check `resp.Error.Details` |

## Testing

### Using LocalStack

```bash
# Start LocalStack
docker run -d -p 4566:4566 localstack/localstack

# Set endpoint
export AWS_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
```

### Mock for Unit Tests

```go
type MockLambdaClient struct{}

func (m *MockLambdaClient) ProvisionTenant(ctx context.Context, fn string, req lambda.ProvisioningRequest) (*lambda.ProvisioningResponse, error) {
    return &lambda.ProvisioningResponse{
        ExecutionID: "test-123",
        Status:      lambda.StatusCompleted,
        Resources: &lambda.ProvisionedResources{
            RDSEndpoint: "test.rds.amazonaws.com",
            S3Bucket:    "test-bucket",
        },
    }, nil
}
```

## Best Practices

1. **Always use context with timeout**
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
   defer cancel()
   ```

2. **Log execution IDs for debugging**
   ```go
   log.Printf("Provisioning execution ID: %s", resp.ExecutionID)
   ```

3. **Use async for long operations**
   ```go
   // If Terraform takes >15 minutes
   err := client.InvokeAsync(ctx, "long-lambda", req)
   // Poll status separately with CheckStatus
   ```

4. **Implement retries**
   ```go
   for i := 0; i < 3; i++ {
       resp, err = client.ProvisionTenant(ctx, fn, req)
       if err == nil { break }
       time.Sleep(time.Duration(i+1) * time.Second)
   }
   ```

## Troubleshooting

### Container can't reach Lambda

```bash
# Check AWS credentials in container
docker exec provisioning-controller env | grep AWS

# Test AWS access
docker exec provisioning-controller aws lambda list-functions
```

### Function returns 200 but status is failed

This is normal - invocation succeeded but Terraform failed:

```go
if resp.Status == lambda.StatusFailed {
    log.Printf("Terraform error: %s", resp.Error.Details)
}
```

### Timeout errors

Increase context timeout:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()
```

## Related Documentation

- Database models: `internal/models/*.go`
- Database layer: `internal/db/*.go`
- Configuration: `internal/config/config.go`
- Docker setup: `docker-compose.yml`
- Migrations: `migrations/*.sql`