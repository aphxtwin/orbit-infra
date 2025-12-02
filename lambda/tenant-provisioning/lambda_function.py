"""
Tenant Provisioning Lambda Function - Terraform Cloud Integration

This Lambda integrates with Terraform Cloud to provision real infrastructure for tenants.
It loads credentials from AWS Parameter Store, triggers Terraform runs, and returns results.
"""

import json
import uuid
import secrets
import string
import time
import os
import urllib.request
import urllib.error
from datetime import datetime

# AWS SDK for Parameter Store access
try:
    import boto3
    ssm_client = boto3.client('ssm', region_name=os.environ.get('AWS_REGION', 'sa-east-1'))
except ImportError:
    print("Warning: boto3 not available - Parameter Store access will fail")
    ssm_client = None


# ============================================================================
# PARAMETER STORE HELPER
# ============================================================================

def load_parameter(name, decrypt=True):
    """Load a single parameter from AWS Systems Manager Parameter Store"""
    try:
        response = ssm_client.get_parameter(Name=name, WithDecryption=decrypt)
        return response['Parameter']['Value']
    except Exception as e:
        print(f"ERROR loading parameter {name}: {e}")
        raise


def load_parameters():
    """Load all required parameters from Parameter Store"""
    print("📦 Loading parameters from Parameter Store...")

    params = {
        # Terraform Cloud
        'tfc_token': load_parameter('/provisioning/tfc/token'),
        'tfc_organization': 'bicidev-orbit',
        # NOTE: workspace_id is NO LONGER USED - each tenant gets its own workspace

        # AWS Credentials
        'aws_access_key': load_parameter('/orbit/aws/access-key-id'),
        'aws_secret_key': load_parameter('/orbit/aws/secret-access-key'),
        'ami_id': load_parameter('/orbit/aws/ami-id', decrypt=False),
        'ssh_key_name': load_parameter('/orbit/aws/ssh-key-name', decrypt=False),

        # Cloudflare
        'cloudflare_api_key': load_parameter('/orbit/cloudflare/api-key'),
        'cloudflare_zone_id': load_parameter('/orbit/cloudflare/zone-id', decrypt=False),

        # GitHub
        'github_repo': load_parameter('/orbit/github/repo', decrypt=False),
        'github_token': load_parameter('/orbit/github/token'),

        # Configuration
        'domain_name': load_parameter('/orbit/config/domain-name', decrypt=False),
        'backend_url': load_parameter('/orbit/config/backend-url', decrypt=False),
        'frontend_url': load_parameter('/orbit/config/frontend-url', decrypt=False),
    }

    print(f"✅ Loaded {len(params)} parameters from Parameter Store")
    return params


# ============================================================================
# TERRAFORM CLOUD API CLIENT
# ============================================================================

class TerraformCloudClient:
    """Client for interacting with Terraform Cloud API"""

    def __init__(self, token, organization, workspace_id=None):
        """Initialize TFC client with credentials"""
        self.token = token
        self.organization = organization
        self.workspace_id = workspace_id  # Can be set later for per-tenant workspaces
        self.base_url = "https://app.terraform.io/api/v2"

    def _make_request(self, method, path, data=None):
        """Make an authenticated request to Terraform Cloud API"""
        url = f"{self.base_url}{path}"
        headers = {
            'Authorization': f'Bearer {self.token}',
            'Content-Type': 'application/vnd.api+json',
        }

        req_data = json.dumps(data).encode('utf-8') if data else None
        request = urllib.request.Request(url, data=req_data, headers=headers, method=method)

        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                return json.loads(response.read().decode('utf-8'))
        except urllib.error.HTTPError as e:
            error_body = e.read().decode('utf-8')
            print(f"❌ TFC API Error: {e.code} - {error_body}")
            raise Exception(f"Terraform Cloud API error: {e.code} - {error_body}")

    def set_variable(self, key, value, sensitive=False, category="terraform"):
        """Create or update a workspace variable (workspace-scoped)"""
        # Step 1: List existing variables to check if it exists
        vars_response = self._make_request('GET', f'/workspaces/{self.workspace_id}/vars')
        existing_var = None

        for var in vars_response.get('data', []):
            if var['attributes']['key'] == key and var['attributes']['category'] == category:
                existing_var = var
                break

        # Step 2: Prepare variable payload
        variable_data = {
            'data': {
                'type': 'vars',
                'attributes': {
                    'key': key,
                    'value': str(value),
                    'category': category,  # "terraform" or "env"
                    'hcl': False,
                    'sensitive': sensitive
                }
            }
        }

        # Step 3: Update existing or create new
        if existing_var:
            # Update existing variable - PATCH /vars/:variable_id
            var_id = existing_var['id']
            variable_data['data']['id'] = var_id  # Include ID in payload for PATCH
            return self._make_request('PATCH', f'/vars/{var_id}', variable_data)
        else:
            # Create new variable - POST /workspaces/:workspace_id/vars
            variable_data['data']['relationships'] = {
                'workspace': {
                    'data': {
                        'id': self.workspace_id,
                        'type': 'workspaces'
                    }
                }
            }
            return self._make_request('POST', f'/workspaces/{self.workspace_id}/vars', variable_data)

    def create_run(self, message="Tenant provisioning via Lambda"):
        """Create and trigger a new Terraform run with auto-apply"""
        run_data = {
            'data': {
                'attributes': {
                    'message': message,
                    'auto-apply': True  # Auto-apply after plan succeeds
                },
                'type': 'runs',
                'relationships': {
                    'workspace': {
                        'data': {
                            'type': 'workspaces',
                            'id': self.workspace_id
                        }
                    }
                }
            }
        }

        response = self._make_request('POST', '/runs', run_data)
        return response['data']['id']

    def get_run_status(self, run_id):
        """Get the current status of a run"""
        response = self._make_request('GET', f'/runs/{run_id}')
        return response['data']['attributes']['status']

    def wait_for_run(self, run_id, timeout=600, poll_interval=10):
        """Wait for a run to complete, polling status every poll_interval seconds"""
        start_time = time.time()

        while True:
            if time.time() - start_time > timeout:
                raise Exception(f"Terraform run timed out after {timeout} seconds")

            status = self.get_run_status(run_id)
            print(f"   Run status: {status}")

            if status == 'applied':
                print("✅ Terraform run completed successfully!")
                return True
            elif status in ['errored', 'canceled', 'discarded']:
                raise Exception(f"Terraform run failed with status: {status}")
            elif status in ['planned_and_finished', 'planned']:
                # This means plan succeeded but not applying (shouldn't happen with auto-apply)
                print("⚠️  Plan completed but not applied")
                return True

            # Still running
            time.sleep(poll_interval)

    def get_outputs(self, run_id):
        """Get outputs from a completed run"""
        # Step 1: Get the run details to find the state version
        run_response = self._make_request('GET', f'/runs/{run_id}')
        relationships = run_response['data'].get('relationships', {})

        # Step 2: Get state version ID
        state_version_data = relationships.get('state-versions', {}).get('data')
        if not state_version_data:
            print("⚠️  No state version found")
            return {}

        state_version_id = state_version_data[0]['id'] if isinstance(state_version_data, list) else state_version_data['id']

        # Step 3: Get outputs from state version
        outputs_response = self._make_request('GET', f'/state-versions/{state_version_id}')

        # Step 4: Extract outputs
        outputs = {}
        output_data = outputs_response.get('data', {}).get('attributes', {}).get('outputs', {})

        for key, value_data in output_data.items():
            outputs[key] = value_data.get('value')

        return outputs

    def create_workspace(self, workspace_name, terraform_version="~> 1.6.0"):
        """Create a new workspace for a tenant"""
        workspace_data = {
            'data': {
                'type': 'workspaces',
                'attributes': {
                    'name': workspace_name,
                    'terraform-version': terraform_version,
                    'auto-apply': True,
                    'description': f'Tenant workspace: {workspace_name}',
                    'execution-mode': 'remote',
                    'file-triggers-enabled': False,
                    'queue-all-runs': False
                }
            }
        }

        response = self._make_request('POST', f'/organizations/{self.organization}/workspaces', workspace_data)
        workspace_id = response['data']['id']
        print(f"✅ Created workspace: {workspace_name} (ID: {workspace_id})")
        return workspace_id

    def get_workspace(self, workspace_name):
        """Get workspace by name, return None if not found"""
        try:
            response = self._make_request('GET', f'/organizations/{self.organization}/workspaces/{workspace_name}')
            return response['data']['id']
        except Exception as e:
            if '404' in str(e):
                return None
            raise

    def delete_workspace(self, workspace_name):
        """Delete a workspace (and all its state)"""
        try:
            self._make_request('DELETE', f'/organizations/{self.organization}/workspaces/{workspace_name}')
            print(f"✅ Deleted workspace: {workspace_name}")
            return True
        except Exception as e:
            print(f"❌ Failed to delete workspace: {e}")
            raise

    def get_or_create_workspace(self, workspace_name):
        """Get existing workspace or create new one"""
        workspace_id = self.get_workspace(workspace_name)
        if workspace_id:
            print(f"📦 Using existing workspace: {workspace_name} (ID: {workspace_id})")
            return workspace_id
        else:
            return self.create_workspace(workspace_name)


def lambda_handler(event, context):
    """
    Main handler - routes to provisioning or deletion based on action

    Provisioning event format:
    {
        "action": "provision",  # optional, default
        "tenant_id": "uuid-string",
        "environment": "staging" | "production",
        "config": {...},
        "metadata": {...}
    }

    Deletion event format:
    {
        "action": "delete",
        "tenant_id": "uuid-string",
        "environment": "staging" | "production",
        "metadata": {
            "subdomain": "tenant-name"
        }
    }
    """
    # Route to appropriate handler based on action
    action = event.get('action', 'provision')

    if action == 'delete':
        return deletion_handler(event, context)
    else:
        return provision_handler(event, context)


def provision_handler(event, context):
    """
    Handle tenant provisioning requests

    Expected event format:
    {
        "tenant_id": "uuid-string",
        "environment": "staging" | "production",
        "config": {
            "region": "sa-east-1",
            "database_instance_class": "db.t3.micro",
            "storage_gb": 20
        },
        "metadata": {
            "subdomain": "tenant-name",
            "admin_email": "admin@example.com",
            "name": "Tenant Name"
        }
    }
    """

    execution_id = str(uuid.uuid4())

    print("=" * 70)
    print("🚀 TENANT PROVISIONING - TERRAFORM CLOUD INTEGRATION")
    print("=" * 70)
    print(f"📋 Execution ID: {execution_id}")
    print(f"📦 Request: {json.dumps(event, indent=2)}")

    try:
        # Validate required fields
        tenant_id = event.get('tenant_id')
        environment = event.get('environment')
        config = event.get('config', {})
        metadata = event.get('metadata', {})

        if not tenant_id:
            return error_response("missing_tenant_id", "tenant_id is required")

        if not environment:
            return error_response("missing_environment", "environment is required")

        if environment not in ['staging', 'production']:
            return error_response("invalid_environment",
                                "environment must be 'staging' or 'production'")

        # Extract metadata
        subdomain = metadata.get('subdomain', 'unknown')
        admin_email = metadata.get('admin_email', 'admin@example.com')
        tenant_name = metadata.get('name', 'Tenant')

        # Extract config with defaults
        region = config.get('region', 'sa-east-1')
        db_instance_class = config.get('database_instance_class', 'db.t3.micro')
        storage_gb = config.get('storage_gb', 20)

        print(f"\n🏢 Tenant: {tenant_name}")
        print(f"🌐 Subdomain: {subdomain}")
        print(f"📧 Admin Email: {admin_email}")
        print(f"🌍 Environment: {environment}")
        print(f"📍 Region: {region}")

        # ====================================================================
        # LOAD PARAMETERS FROM PARAMETER STORE
        # ====================================================================
        print("\n" + "=" * 70)
        print("📦 LOADING CONFIGURATION FROM PARAMETER STORE")
        print("=" * 70)

        params = load_parameters()

        # ====================================================================
        # GENERATE SECRETS
        # ====================================================================
        print("\n" + "=" * 70)
        print("🔐 GENERATING SECURE CREDENTIALS")
        print("=" * 70)

        admin_password = generate_password(32)
        db_password = generate_password(32)
        jwt_secret = generate_secret(48)
        webhook_secret = generate_secret(32)
        db_name = f"odoo_{subdomain.replace('-', '_')}_{environment}"

        print(f"   ✅ Admin password: [GENERATED - 32 chars]")
        print(f"   ✅ DB password: [GENERATED - 32 chars]")
        print(f"   ✅ JWT secret: [GENERATED - base64]")
        print(f"   ✅ Webhook secret: [GENERATED - base64]")
        print(f"   ✅ Database name: {db_name}")

        # ====================================================================
        # INITIALIZE TERRAFORM CLOUD CLIENT & CREATE WORKSPACE
        # ====================================================================
        print("\n" + "=" * 70)
        print("🔧 INITIALIZING TERRAFORM CLOUD CLIENT")
        print("=" * 70)

        # Generate unique workspace name per tenant
        workspace_name = f"tenant-{subdomain}-{environment}"
        print(f"   Target workspace: {workspace_name}")

        # Initialize TFC client (workspace will be set after creation)
        tfc = TerraformCloudClient(
            token=params['tfc_token'],
            organization=params['tfc_organization']
        )

        # Create or get the tenant-specific workspace
        workspace_id = tfc.get_or_create_workspace(workspace_name)
        tfc.workspace_id = workspace_id  # Set the workspace for this tenant

        print(f"   Organization: {params['tfc_organization']}")
        print(f"   Workspace ID: {workspace_id}")

        # ====================================================================
        # SET TERRAFORM VARIABLES
        # ====================================================================
        print("\n" + "=" * 70)
        print("📝 SETTING TERRAFORM WORKSPACE VARIABLES")
        print("=" * 70)

        variables = {
            # Tenant-specific (dynamic)
            'subdomain': subdomain,
            'tenant_name': tenant_name,
            'odoo_admin_email': admin_email,
            'odoo_db_name': db_name,
            'region': region,

            # Generated secrets (sensitive)
            'odoo_admin_password': (admin_password, True),
            'odoo_db_password': (db_password, True),
            'odoo_jwt_secret': (jwt_secret, True),

            # Static configuration
            'domain_name': params['domain_name'],
            'backend_url': params['backend_url'],
            'frontend_url': params['frontend_url'],
            'github_branch': 'main',

            # Sensitive static config
            'aws_access_key': (params['aws_access_key'], True),
            'aws_secret_key': (params['aws_secret_key'], True),
            'ami_id': params['ami_id'],
            'ssh_key_name': params['ssh_key_name'],
            'cloudflare_api_key': (params['cloudflare_api_key'], True),
            'cloudflare_zone_id': params['cloudflare_zone_id'],
            'github_repo': params['github_repo'],
            'github_token': (params['github_token'], True),
        }

        for key, value in variables.items():
            sensitive = False
            if isinstance(value, tuple):
                value, sensitive = value

            print(f"   Setting: {key} {'(sensitive)' if sensitive else ''}")
            tfc.set_variable(key, value, sensitive=sensitive)

        print(f"✅ Set {len(variables)} variables")

        # ====================================================================
        # CREATE AND TRIGGER TERRAFORM RUN
        # ====================================================================
        print("\n" + "=" * 70)
        print("🚀 CREATING TERRAFORM RUN")
        print("=" * 70)

        run_message = f"Provision {tenant_name} ({subdomain}) - {environment}"
        run_id = tfc.create_run(run_message)

        print(f"✅ Run created: {run_id}")
        print(f"📝 Message: {run_message}")
        print("\n⏳ Waiting for Terraform run to complete...")
        print("   (This may take 3-5 minutes)")

        # ====================================================================
        # WAIT FOR RUN TO COMPLETE
        # ====================================================================
        start_time = time.time()
        tfc.wait_for_run(run_id, timeout=600, poll_interval=10)
        duration = time.time() - start_time

        print(f"✅ Run completed in {duration:.1f} seconds")

        # ====================================================================
        # EXTRACT OUTPUTS
        # ====================================================================
        print("\n" + "=" * 70)
        print("📤 EXTRACTING TERRAFORM OUTPUTS")
        print("=" * 70)

        outputs = tfc.get_outputs(run_id)

        print(f"   Outputs received: {list(outputs.keys())}")
        for key, value in outputs.items():
            print(f"   {key}: {value}")

        # ====================================================================
        # BUILD RESPONSE
        # ====================================================================
        full_domain = outputs.get('full_domain', f"{subdomain}.{params['domain_name']}")

        resources = {
            "public_ip": outputs.get('public_ip'),
            "instance_id": outputs.get('instance_id'),
            "full_domain": full_domain,
            "odoo_host": full_domain,
        }

        # Build complete response with secrets
        response = {
            "execution_id": execution_id,
            "status": "completed",
            "resources": resources,
            "secrets": {
                # Application credentials
                "admin_email": admin_email,
                "admin_password": admin_password,
                "db_name": db_name,
                "db_password": db_password,
                "jwt_secret": jwt_secret,
                "webhook_secret": webhook_secret,

                # URLs
                "odoo_host": full_domain,
                "backend_url": params['backend_url'],
                "front_url": params['frontend_url'],
            },
            "metadata": {
                "provisioned_at": datetime.utcnow().isoformat() + "Z",
                "tenant_id": tenant_id,
                "tenant_name": tenant_name,
                "subdomain": subdomain,
                "admin_email": admin_email,
                "environment": environment,
                "region": region,
                "terraform_run_id": run_id,
                "terraform_workspace_name": workspace_name,
                "terraform_workspace_id": workspace_id,
                "duration_seconds": round(duration, 2)
            }
        }

        # Print credentials summary for easy copying
        print("\n" + "=" * 70)
        print("📋 CREDENTIALS TO COPY")
        print("=" * 70)
        print(f"\n🌐 ODOO INSTANCE:")
        print(f"   URL:      https://{full_domain}")
        print(f"   Email:    {admin_email}")
        print(f"   Password: {admin_password}")

        print(f"\n🗄️  DATABASE:")
        print(f"   Name:     {db_name}")
        print(f"   Password: {db_password}")

        print(f"\n🔑 SECRETS:")
        print(f"   JWT Secret:     {jwt_secret}")
        print(f"   Webhook Secret: {webhook_secret}")

        print(f"\n☁️  INFRASTRUCTURE:")
        print(f"   Public IP:    {outputs.get('public_ip')}")
        print(f"   Instance ID:  {outputs.get('instance_id')}")

        print("\n" + "=" * 70)
        print("✅ PROVISIONING COMPLETE!")
        print("=" * 70)

        return response

    except Exception as e:
        print(f"\n❌ ERROR: {str(e)}")
        import traceback
        traceback.print_exc()
        return error_response("provisioning_error", str(e))


def deletion_handler(event, context):
    """
    Handle tenant deletion requests - destroys infrastructure by deleting the workspace

    Expected event format:
    {
        "action": "delete",
        "tenant_id": "uuid-string",
        "environment": "staging" | "production",
        "metadata": {
            "subdomain": "tenant-name"
        }
    }
    """
    execution_id = str(uuid.uuid4())

    print("=" * 70)
    print("🗑️  TENANT DELETION - TERRAFORM CLOUD WORKSPACE CLEANUP")
    print("=" * 70)
    print(f"📋 Execution ID: {execution_id}")

    try:
        tenant_id = event.get('tenant_id')
        environment = event.get('environment')
        metadata = event.get('metadata', {})
        subdomain = metadata.get('subdomain')

        if not tenant_id or not environment or not subdomain:
            return error_response("missing_parameters",
                                "tenant_id, environment, and subdomain are required")

        # Load TFC credentials
        params = load_parameters()

        # Generate workspace name (must match provisioning)
        workspace_name = f"tenant-{subdomain}-{environment}"

        print(f"🎯 Target workspace: {workspace_name}")

        # Initialize TFC client
        tfc = TerraformCloudClient(
            token=params['tfc_token'],
            organization=params['tfc_organization']
        )

        # Delete the workspace (this destroys all resources and removes state)
        print(f"\n🔥 Deleting workspace and all associated infrastructure...")
        tfc.delete_workspace(workspace_name)

        print("\n" + "=" * 70)
        print("✅ DELETION COMPLETE!")
        print("=" * 70)

        return {
            "execution_id": execution_id,
            "status": "completed",
            "action": "delete",
            "metadata": {
                "deleted_at": datetime.utcnow().isoformat() + "Z",
                "tenant_id": tenant_id,
                "environment": environment,
                "workspace_name": workspace_name
            }
        }

    except Exception as e:
        print(f"\n❌ ERROR: {str(e)}")
        import traceback
        traceback.print_exc()
        return error_response("deletion_error", str(e))


def generate_password(length=32):
    """
    Generate a cryptographically secure random password
    Uses uppercase, lowercase, digits, and safe special characters
    """
    charset = string.ascii_letters + string.digits + "!@#$%^&*-_=+"
    return ''.join(secrets.choice(charset) for _ in range(length))


def generate_secret(byte_length=48):
    """
    Generate a cryptographically secure random secret
    Returns a base64-like encoded string
    """
    return secrets.token_urlsafe(byte_length)


def error_response(code, message):
    """Helper to create error responses"""
    execution_id = str(uuid.uuid4())

    print("\n" + "=" * 60)
    print(f"❌ PROVISIONING FAILED")
    print("=" * 60)
    print(f"Error Code: {code}")
    print(f"Message: {message}")
    print("=" * 60)

    return {
        "execution_id": execution_id,
        "status": "failed",
        "error": {
            "code": code,
            "message": message
        }
    }
