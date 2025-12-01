# AWS Lambda Integration Plan

## Overview
The provisioning controller service needs to integrate with AWS Lambda functions to orchestrate tenant infrastructure provisioning. This document outlines the integration architecture and implementation plan.

## Architecture

### Current State
- Provisioning controller service running in Docker
- PostgreSQL database tracking tenant state
- Models: Tenant, Environment, InfrastructureState, ProvisioningLog

### Target State
- Controller service invokes Lambda functions for infrastructure operations
- Lambda functions execute Terraform/CDK to provision AWS resources
- Asynchronous execution with status tracking
- Event-driven updates back to controller

## Lambda Functions

### 1. Tenant Provisioning Lambda
**Purpose:** Create complete tenant environment (RDS, S3, networking, etc.)

**Input:**
```json
{
  "tenant_id": "uuid",
  "environment": "staging|production",
  "config": {
    "region": "us-east-1",
    "database_instance_class": "db.t3.micro",
    "storage_gb": 20
  }
}
```

**Output:**
```json
{
  "execution_id": "uuid",
  "status": "pending|in_progress|completed|failed",
  "resources": {
    "rds_endpoint": "tenant-123.xyz.rds.amazonaws.com",
    "s3_bucket": "tenant-123-storage",
    "vpc_id": "vpc-xxx"
  }
}
```

### 2. Tenant Deletion Lambda
**Purpose:** Clean up all tenant resources

**Input:**
```json
{
  "tenant_id": "uuid",
  "environment": "staging|production"
}
```

### 3. Infrastructure Status Lambda
**Purpose:** Check current state of tenant infrastructure

**Input:**
```json
{
  "tenant_id": "uuid",
  "environment": "staging|production"
}
```

## Integration Flow

### Provisioning Flow
```
1. API Request → Controller Service
2. Controller validates request
3. Controller creates Tenant + Environment records (status: pending)
4. Controller invokes Lambda (async)
5. Lambda begins Terraform execution
6. Lambda sends progress updates via EventBridge
7. Controller listens to events, updates database
8. Lambda completes, final status update
9. Controller marks infrastructure as ready
```

### Communication Patterns

#### Option 1: Synchronous Invocation (Simple)
- Controller invokes Lambda with `InvocationType=RequestResponse`
- Waits for Lambda to complete (max 15 minutes)
- Suitable for quick operations only
- **Limitation:** Lambda max execution time is 15 minutes

#### Option 2: Asynchronous Invocation (Recommended)
- Controller invokes Lambda with `InvocationType=Event`
- Lambda returns immediately with execution ID
- Controller polls Lambda logs or DynamoDB for status
- Suitable for long-running operations

#### Option 3: Step Functions (Future)
- Controller starts Step Functions execution
- Step Functions orchestrates multiple Lambdas
- Built-in retry and error handling
- Best for complex multi-step workflows

## Implementation Components

### In Controller Service

#### 1. AWS Lambda Client Package
```
internal/aws/
  lambda/
    client.go      # Lambda invocation wrapper
    types.go       # Request/response types
    errors.go      # AWS-specific errors
```

**Features:**
- Invoke Lambda functions
- Handle AWS credentials (IAM role in production, env vars in dev)
- Retry logic with exponential backoff
- Error handling and logging

#### 2. Provisioning Service Layer
```
internal/service/
  provisioning.go  # Orchestrates provisioning workflow
```

**Responsibilities:**
- Validate provisioning requests
- Create database records
- Invoke Lambda functions
- Handle callbacks/status updates
- Update infrastructure state

#### 3. Status Tracking
Two approaches:

**A. Polling (Simpler for MVP)**
- Store Lambda execution ID in database
- Background worker polls Lambda status
- Updates database when status changes

**B. Event-Driven (Production Ready)**
- Lambda publishes events to EventBridge/SNS
- Controller subscribes to events via SQS
- Updates database on event receipt

### Database Schema Updates

#### Add to `infrastructure_state` table:
```sql
ALTER TABLE infrastructure_state ADD COLUMN lambda_execution_id TEXT;
ALTER TABLE infrastructure_state ADD COLUMN lambda_invocation_time TIMESTAMPTZ;
ALTER TABLE infrastructure_state ADD COLUMN last_status_check TIMESTAMPTZ;
```

#### Track provisioning operations:
```sql
CREATE TABLE provisioning_operations (
  id UUID PRIMARY KEY,
  tenant_id UUID REFERENCES tenants(id),
  operation_type TEXT, -- 'create', 'delete', 'update'
  lambda_function_arn TEXT,
  lambda_execution_id TEXT,
  status TEXT, -- 'pending', 'in_progress', 'completed', 'failed'
  input_payload JSONB,
  output_payload JSONB,
  error_message TEXT,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

## Configuration

### Environment Variables
```bash
# Lambda Configuration
LAMBDA_PROVISIONING_FUNCTION_ARN=arn:aws:lambda:us-east-1:123456789:function:tenant-provisioner
LAMBDA_DELETION_FUNCTION_ARN=arn:aws:lambda:us-east-1:123456789:function:tenant-deletion
LAMBDA_INVOCATION_TYPE=Event  # Event (async) or RequestResponse (sync)
LAMBDA_TIMEOUT_SECONDS=300

# AWS Configuration (already have AWS_REGION)
AWS_REGION=us-east-1
```

### IAM Permissions Required
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "lambda:InvokeFunction",
        "lambda:GetFunction"
      ],
      "Resource": [
        "arn:aws:lambda:*:*:function:tenant-*"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "logs:GetLogEvents",
        "logs:FilterLogEvents"
      ],
      "Resource": "arn:aws:logs:*:*:log-group:/aws/lambda/tenant-*"
    }
  ]
}
```

## Development Approach

### Phase 1: Basic Lambda Invocation (Current Focus)
- [ ] Create AWS Lambda client wrapper
- [ ] Add Lambda config to environment
- [ ] Test basic invocation (sync mode)
- [ ] Add error handling

### Phase 2: Async Execution & Polling
- [ ] Implement async invocation
- [ ] Store execution IDs in database
- [ ] Create background polling worker
- [ ] Update infrastructure state based on status

### Phase 3: Event-Driven Updates (Future)
- [ ] Lambda publishes to EventBridge/SNS
- [ ] Controller consumes via SQS
- [ ] Real-time status updates

### Phase 4: Production Hardening
- [ ] Add retry logic
- [ ] Implement circuit breakers
- [ ] Add metrics and monitoring
- [ ] Implement idempotency

## Testing Strategy

### Local Development
- Use LocalStack to mock Lambda
- Or create a "mock Lambda" HTTP endpoint for testing
- Test all error scenarios

### Integration Testing
- Deploy test Lambda to AWS
- Test full provisioning flow
- Verify cleanup on failure

## Open Questions

1. **Lambda Timeout:** What's the expected duration for tenant provisioning?
   - If < 15min: Can use synchronous invocation
   - If > 15min: Must use async + polling/events

2. **Lambda Location:** Where will the Lambda code live?
   - Same repo? Separate repo?
   - How is it deployed?

3. **Terraform State:** Where is Terraform state stored?
   - S3 backend with DynamoDB locking?
   - How does controller access/query state?

4. **Error Recovery:** What happens if Lambda fails mid-provision?
   - Manual cleanup?
   - Automatic rollback?
   - Retry strategy?

5. **Multi-Region:** Do we need to support provisioning across regions?
   - Impact on Lambda deployment
   - Impact on networking/VPC peering

## Next Steps

1. Create basic AWS SDK integration
2. Implement simple Lambda invocation
3. Test with a dummy Lambda function
4. Decide on async vs sync approach
5. Implement status tracking