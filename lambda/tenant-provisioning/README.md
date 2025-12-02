# Tenant Provisioning Lambda Function

This Lambda function handles tenant provisioning requests from the Go controller.

## Current Implementation

Currently returns mock successful responses without actually provisioning infrastructure.
This allows you to test the integration flow end-to-end.

## Deployment

### Option 1: Deploy via AWS Console

1. Go to AWS Lambda Console
2. Create function:
   - Name: `tenant-provisioning`
   - Runtime: Python 3.12
   - Architecture: x86_64
3. Copy the code from `lambda_function.py` into the Lambda editor
4. Deploy

### Option 2: Deploy via AWS CLI

```bash
cd lambda/tenant-provisioning

# Create deployment package
zip -r function.zip lambda_function.py

# Create Lambda function
aws lambda create-function \
  --function-name tenant-provisioning \
  --runtime python3.12 \
  --role arn:aws:iam::YOUR_ACCOUNT_ID:role/YOUR_LAMBDA_ROLE \
  --handler lambda_function.lambda_handler \
  --zip-file fileb://function.zip \
  --region sa-east-1

# Or update existing function
aws lambda update-function-code \
  --function-name tenant-provisioning \
  --zip-file fileb://function.zip \
  --region sa-east-1
```

### Option 3: Deploy via Terraform (recommended for production)

See `terraform/` directory (to be created).

## Required IAM Permissions

The Lambda execution role needs:
- Basic Lambda execution permissions (CloudWatch Logs)
- Later: RDS, S3, VPC permissions when adding real provisioning

## Testing

Test the Lambda directly:

```bash
aws lambda invoke \
  --function-name tenant-provisioning \
  --payload '{"tenant_id":"test-123","environment":"staging","config":{"region":"sa-east-1","database_instance_class":"db.t3.micro","storage_gb":20}}' \
  --region sa-east-1 \
  response.json

cat response.json
```

## Future Enhancements

- [ ] Add actual RDS provisioning
- [ ] Add S3 bucket creation
- [ ] Add VPC/networking setup
- [ ] Add async processing with Step Functions for long-running tasks
- [ ] Add status check endpoint
- [ ] Add deletion/cleanup logic
- [ ] Add resource tagging
- [ ] Add cost estimation
