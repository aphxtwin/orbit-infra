# Provisioning Service Architecture

This document provides a complete overview of the provisioning service architecture, showing how each module and component interacts.

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              PROVISIONING SERVICE                                    │
│                          (Tenant Multi-Tenancy Orchestrator)                         │
└─────────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────────┐
│  ENTRY POINT: cmd/controller/main.go                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────────┐   │
│  │  1. Load Config from ENV vars → internal/config/config.go                   │   │
│  │     ├─ AppConfig (environment, log level, port)                             │   │
│  │     ├─ DatabaseConfig (host, port, credentials, pool settings)              │   │
│  │     └─ AWSConfig (region, KMS ARN, Lambda function name)                    │   │
│  │                                                                               │   │
│  │  2. Initialize Database → internal/db/db.go                                  │   │
│  │     ├─ Creates pgxpool.Pool with connection pooling                          │   │
│  │     ├─ Configures MaxConns, MinConns, connection lifetimes                   │   │
│  │     └─ Pings DB to verify connectivity                                       │   │
│  │                                                                               │   │
│  │  3. Run Migrations → internal/migrate/migrate.go                             │   │
│  │     └─ Applies schema migrations from migrations/ directory                  │   │
│  │                                                                               │   │
│  │  4. Initialize AWS Lambda Client → internal/aws/client.go                    │   │
│  │     ├─ Loads AWS credentials (env vars or IAM role)                          │   │
│  │     ├─ Creates Lambda SDK client for specified region                        │   │
│  │     └─ Provides methods: ProvisionTenant(), DeleteTenant(), CheckStatus()    │   │
│  │                                                                               │   │
│  │  5. Create Orchestrator → internal/orchestrator/orchestrator.go              │   │
│  │     └─ Wires together DB + Lambda Client + Function Name                     │   │
│  │                                                                               │   │
│  │  6. Create HTTP Handlers → internal/handlers/tenant.go                       │   │
│  │     └─ TenantHandler wraps Orchestrator                                      │   │
│  │                                                                               │   │
│  │  7. Start HTTP Server (port :8080)                                           │   │
│  │     ├─ GET  /health  → DB health check                                       │   │
│  │     ├─ GET  /ready   → Readiness probe                                       │   │
│  │     └─ POST /tenants → Create tenant endpoint                                │   │
│  └─────────────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────────┐
│  REQUEST FLOW: POST /tenants (Tenant Creation)                                       │
└─────────────────────────────────────────────────────────────────────────────────────┘

   Client Request
       │
       │ POST /tenants
       │ {
       │   "name": "Acme Motors",
       │   "subdomain": "acme",
       │   "environment": "production",
       │   "region": "us-east-1",
       │   "admin_email": "admin@acme.com",
       │   "database_instance_class": "db.t3.small",
       │   "storage_gb": 50
       │ }
       │
       ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│  HTTP LAYER: internal/handlers/tenant.go                                     │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │  TenantHandler.CreateTenant()                                          │ │
│  │  • Parses JSON request → api.CreateTenantRequest                       │ │
│  │  • Validates JSON structure                                            │ │
│  │  • Delegates to Orchestrator                                           │ │
│  │  • Returns 202 Accepted or error (400/500)                             │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────┘
       │
       │ api.CreateTenantRequest
       ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│  BUSINESS LOGIC: internal/orchestrator/orchestrator.go                       │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │  Orchestrator.CreateTenant()                                           │ │
│  │                                                                         │ │
│  │  Step 1: Validation                                                    │ │
│  │  ├─ validateCreateRequest()                                            │ │
│  │  │  • Checks required fields (name, subdomain, environment, region)    │ │
│  │  │  • Validates environment ∈ {staging, production}                    │ │
│  │  │  └─ Returns error if validation fails                               │ │
│  │                                                                         │ │
│  │  Step 2: Generate Tenant ID & Set Defaults                             │ │
│  │  ├─ Generate UUID for tenant                                           │ │
│  │  ├─ Set default db_instance_class: "db.t3.micro"                       │ │
│  │  ├─ Set default storage_gb: 20                                         │ │
│  │  └─ Build REX URL: https://{subdomain}.rex.example.com                 │ │
│  │                                                                         │ │
│  │  Step 3: Create Tenant in Database ──────────────┐                     │ │
│  │  ├─ createTenantInDB()                            │                     │ │
│  │  │  • Creates models.Tenant object                │                     │ │
│  │  │  • Status: "pending"                           │                     │ │
│  │  │  • INSERT INTO tenants table                   │                     │ │
│  │  │                                                 │                     │ │
│  │  Step 4: Update Status to "provisioning" ────────┤                     │ │
│  │  ├─ updateTenantStatus()                          │                     │ │
│  │  │  • UPDATE tenants SET status = 'provisioning'  │                     │ │
│  │  │                                                 │                     │ │
│  │  Step 5: Invoke Lambda (SYNCHRONOUS) ────────────┼──────┐              │ │
│  │  ├─ lambdaClient.ProvisionTenant()                │      │              │ │
│  │  │  • Builds ProvisioningRequest                  │      │              │ │
│  │  │  • Invokes Lambda function synchronously       │      │              │ │
│  │  │  • Waits for Lambda to complete (blocks)       │      │              │ │
│  │  │  • Returns ProvisioningResponse                │      │              │ │
│  │  │                                                 │      │              │ │
│  │  Step 6: Handle Lambda Response                   │      │              │ │
│  │  ├─ If Status == "completed":                     │      │              │ │
│  │  │  ├─ updateTenantStatus("active") ──────────────┤      │              │ │
│  │  │  └─ storeInfrastructureState() ────────────────┤      │              │ │
│  │  │     • Saves RDS endpoint, S3 bucket            │      │              │ │
│  │  │     • Stores metadata as JSONB                 │      │              │ │
│  │  │                                                 │      │              │ │
│  │  ├─ If Status == "failed":                        │      │              │ │
│  │  │  ├─ updateTenantStatus("failed") ──────────────┤      │              │ │
│  │  │  └─ Return error to client                     │      │              │ │
│  │  │                                                 │      │              │ │
│  │  Step 7: Return Response                          │      │              │ │
│  │  └─ api.CreateTenantResponse                      │      │              │ │
│  │     • tenant_id, status, created_at               │      │              │ │
│  └────────────────────────────────────────────────────┼──────┼──────────────┘ │
└────────────────────────────────────────────────────────┼──────┼────────────────┘
                                                         │      │
           ┌─────────────────────────────────────────────┘      │
           ▼                                                     │
┌──────────────────────────────────────────────────────┐        │
│  DATABASE LAYER: internal/db/                        │        │
│  ┌────────────────────────────────────────────────┐  │        │
│  │  db.DB (wraps pgxpool.Pool)                    │  │        │
│  │  • Connection pooling (5-25 connections)       │  │        │
│  │  • Automatic connection lifecycle management   │  │        │
│  │  • Health checks via Ping()                    │  │        │
│  └────────────────────────────────────────────────┘  │        │
│                                                       │        │
│  Tables (via raw SQL queries):                       │        │
│  ┌────────────────────────────────────────────────┐  │        │
│  │  tenants                                       │  │        │
│  │  ├─ id (UUID, PK)                              │  │        │
│  │  ├─ name, subdomain, rex_url                   │  │        │
│  │  ├─ admin_email                                │  │        │
│  │  ├─ status (pending/provisioning/active/...)   │  │        │
│  │  ├─ region                                     │  │        │
│  │  ├─ config (JSONB)                             │  │        │
│  │  └─ created_at, updated_at                     │  │        │
│  │                                                 │  │        │
│  │  infrastructure_state                          │  │        │
│  │  ├─ id (UUID, PK)                              │  │        │
│  │  ├─ tenant_id (FK → tenants.id)                │  │        │
│  │  ├─ rds_endpoint                               │  │        │
│  │  ├─ s3_backup_bucket                           │  │        │
│  │  ├─ outputs (JSONB - Lambda metadata)          │  │        │
│  │  └─ created_at, updated_at                     │  │        │
│  │                                                 │  │        │
│  │  provisioning_logs                             │  │        │
│  │  └─ Event logs for provisioning operations     │  │        │
│  │                                                 │  │        │
│  │  environments                                   │  │        │
│  │  └─ Environment configuration                  │  │        │
│  └────────────────────────────────────────────────┘  │        │
└──────────────────────────────────────────────────────┘        │
                                                                 │
                 ┌───────────────────────────────────────────────┘
                 ▼
┌──────────────────────────────────────────────────────────────────────┐
│  AWS LAYER: internal/aws/client.go                                   │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  Lambda Client (wraps AWS SDK v2)                             │  │
│  │                                                                 │  │
│  │  ProvisionTenant(functionName, request)                        │  │
│  │  ├─ Serializes ProvisioningRequest → JSON                      │  │
│  │  ├─ Invokes Lambda with InvocationType: RequestResponse        │  │
│  │  │  (SYNCHRONOUS - waits for completion)                       │  │
│  │  ├─ Receives InvocationResult                                  │  │
│  │  │  • StatusCode, ExecutedVersion, Payload                     │  │
│  │  │  • FunctionError (if Lambda failed)                         │  │
│  │  ├─ Deserializes JSON → ProvisioningResponse                   │  │
│  │  └─ Returns response or error                                  │  │
│  │                                                                 │  │
│  │  Also supports:                                                 │  │
│  │  • DeleteTenant() - Remove tenant infrastructure               │  │
│  │  • CheckStatus() - Query provisioning status                   │  │
│  │  • InvokeAsync() - Fire-and-forget invocations                 │  │
│  └────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────┘
                 │
                 │ HTTP/JSON over AWS API
                 ▼
┌──────────────────────────────────────────────────────────────────────┐
│  AWS LAMBDA FUNCTION (External - not in this codebase)               │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  Tenant Provisioning Lambda                                    │  │
│  │  • Receives ProvisioningRequest (tenant_id, config, metadata)  │  │
│  │  • Provisions AWS resources using Terraform/CloudFormation:    │  │
│  │    ├─ RDS PostgreSQL instance (tenant database)                │  │
│  │    ├─ S3 bucket (backups)                                      │  │
│  │    ├─ Security groups, VPC config                              │  │
│  │    └─ IAM roles                                                │  │
│  │  • Returns ProvisioningResponse:                               │  │
│  │    ├─ Status: "completed" | "failed"                           │  │
│  │    ├─ Resources: {rds_endpoint, s3_bucket}                     │  │
│  │    ├─ Metadata: {outputs...}                                   │  │
│  │    └─ Error: {code, message} (if failed)                       │  │
│  └────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│  DATA MODELS: internal/models/                                               │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  tenant.go                                                             │  │
│  │  • Tenant struct (maps to tenants table)                              │  │
│  │  • Status constants (pending, provisioning, active, failed, etc.)     │  │
│  │  • Helper methods: IsActive(), IsPending(), CanProvision()            │  │
│  │                                                                         │  │
│  │  infrastructure.go                                                     │  │
│  │  • InfrastructureState struct                                         │  │
│  │  • Stores provisioned AWS resource details                            │  │
│  │                                                                         │  │
│  │  log.go                                                                │  │
│  │  • ProvisioningLog struct                                             │  │
│  │  • Event logging for audit trail                                      │  │
│  │                                                                         │  │
│  │  environment.go                                                        │  │
│  │  • Environment struct                                                 │  │
│  │  • Configuration per environment (staging/production)                 │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│  API TYPES: internal/api/types.go                                            │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  CreateTenantRequest                                                   │  │
│  │  • Input DTO for tenant creation                                      │  │
│  │  • JSON tags for HTTP deserialization                                 │  │
│  │                                                                         │  │
│  │  CreateTenantResponse                                                  │  │
│  │  • Output DTO with tenant_id, status, timestamps                      │  │
│  │                                                                         │  │
│  │  ErrorResponse                                                         │  │
│  │  • Standardized error format                                          │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│  SUPPORTING MODULES                                                           │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  internal/migrate/migrate.go                                          │  │
│  │  • Runs golang-migrate for schema migrations                          │  │
│  │  • Reads from migrations/ directory                                   │  │
│  │  • Tracks version in schema_migrations table                          │  │
│  │                                                                         │  │
│  │  internal/db/transaction.go                                           │  │
│  │  • Helpers for database transactions (if needed)                      │  │
│  │                                                                         │  │
│  │  internal/db/errors.go                                                │  │
│  │  • Custom database error types                                        │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│  KEY CHARACTERISTICS                                                          │
│                                                                               │
│  • SYNCHRONOUS provisioning: Orchestrator waits for Lambda to complete       │
│  • State transitions: pending → provisioning → active/failed                 │
│  • Database-first: Tenant record created before Lambda invocation            │
│  • Rollback on failure: Status updated to "failed" if Lambda errors          │
│  • Infrastructure state persisted after successful provisioning              │
│  • Connection pooling for database efficiency                                │
│  • Health checks for Kubernetes/container orchestration                      │
│  • Environment-based configuration (12-factor app)                           │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Module Descriptions

### Entry Point (`cmd/controller/main.go`)
The main application entry point that:
- Loads configuration from environment variables
- Initializes all dependencies (database, AWS clients)
- Runs database migrations
- Sets up HTTP routes and server
- Handles graceful shutdown

### HTTP Layer (`internal/handlers/`)
HTTP request handlers that:
- Parse and validate incoming JSON requests
- Delegate business logic to the orchestrator
- Return appropriate HTTP status codes and responses
- Handle error formatting

### Business Logic (`internal/orchestrator/`)
Core orchestration logic that:
- Validates tenant creation requests
- Manages tenant lifecycle state transitions
- Coordinates between database and AWS Lambda
- Handles provisioning success/failure scenarios

### Database Layer (`internal/db/`)
PostgreSQL connection management:
- Connection pooling with configurable limits
- Health check support
- Transaction support
- Direct SQL query execution

### AWS Layer (`internal/aws/`)
AWS SDK wrapper for Lambda invocations:
- Synchronous and asynchronous invocation support
- JSON serialization/deserialization
- Error handling and status code checks
- Support for multiple Lambda operations (provision, delete, status check)

### Data Models (`internal/models/`)
Domain models representing database entities:
- Tenant lifecycle and state
- Infrastructure resource details
- Provisioning event logs
- Environment configurations

### API Types (`internal/api/`)
Data Transfer Objects (DTOs) for HTTP API:
- Request/response structures
- JSON serialization tags
- Error response formats

### Configuration (`internal/config/`)
Environment-based configuration loading:
- Application settings (port, environment)
- Database connection parameters
- AWS settings (region, Lambda function names)

### Migrations (`internal/migrate/`)
Database schema version management:
- Applies migrations from `migrations/` directory
- Tracks schema version
- Ensures database schema consistency

## State Transitions

```
pending → provisioning → active
                      ↘ failed
```

- **pending**: Tenant record created, awaiting provisioning
- **provisioning**: Lambda invocation in progress
- **active**: Infrastructure successfully provisioned
- **failed**: Provisioning failed, can be retried

## Key Design Decisions

1. **Synchronous provisioning**: The API waits for Lambda to complete before responding. This ensures immediate feedback to the caller but ties up the HTTP connection.

2. **Database-first approach**: Tenant records are created before Lambda invocation to ensure state is persisted even if provisioning fails.

3. **Status-based state machine**: Tenant lifecycle is managed through explicit status transitions tracked in the database.

4. **Lambda as infrastructure provisioner**: Actual AWS resource provisioning is delegated to Lambda functions, keeping the controller lightweight.

5. **Connection pooling**: Database connections are pooled for efficiency with configurable limits.

6. **12-factor configuration**: All configuration comes from environment variables for deployment flexibility.