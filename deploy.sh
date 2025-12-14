#!/bin/bash
set -e

# Orby Slack Client + MCP Server - GCE VM Deployment Script
# Deploys both slack-mcp-client and orby-mcp-server on a GCE VM using Terraform

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TERRAFORM_DIR="${SCRIPT_DIR}/terraform"

echo "🚀 Deploying Orby Slack Client + MCP Server (GCE VM)"
echo ""

# Check for terraform
if ! command -v terraform &> /dev/null; then
    echo "❌ Terraform not found. Install from: https://www.terraform.io/downloads"
    exit 1
fi

# Check for required env vars or terraform.tfvars
if [ ! -f "${TERRAFORM_DIR}/terraform.tfvars" ]; then
    echo "📝 No terraform.tfvars found. Checking environment variables..."
    
    # Check required vars
    MISSING_VARS=()
    [ -z "$SLACK_BOT_TOKEN" ] && MISSING_VARS+=("SLACK_BOT_TOKEN")
    [ -z "$SLACK_APP_TOKEN" ] && MISSING_VARS+=("SLACK_APP_TOKEN")
    [ -z "$SLACK_SIGNING_SECRET" ] && MISSING_VARS+=("SLACK_SIGNING_SECRET")
    [ -z "$NOTION_API_KEY" ] && MISSING_VARS+=("NOTION_API_KEY")
    [ -z "$LITELLM_API_KEY" ] && MISSING_VARS+=("LITELLM_API_KEY")
    
    if [ ${#MISSING_VARS[@]} -gt 0 ]; then
        echo "❌ Missing required environment variables: ${MISSING_VARS[*]}"
        echo ""
        echo "Either:"
        echo "  1. Set environment variables and re-run"
        echo "  2. Copy terraform.tfvars.template to terraform.tfvars and fill in values"
        exit 1
    fi
    
    # Create tfvars from environment
    echo "Creating terraform.tfvars from environment..."
    cat > "${TERRAFORM_DIR}/terraform.tfvars" << EOF
project_id = "${GCP_PROJECT_ID:-gen-lang-client-0335698828}"
region     = "europe-west1"
zone       = "europe-west1-b"

machine_type = "e2-small"

slack_bot_token      = "${SLACK_BOT_TOKEN}"
slack_app_token      = "${SLACK_APP_TOKEN}"
slack_signing_secret = "${SLACK_SIGNING_SECRET}"

notion_api_key = "${NOTION_API_KEY}"

litellm_api_key  = "${LITELLM_API_KEY}"
litellm_base_url = "${LITELLM_BASE_URL:-https://litellm.tolemy.bio/v1}"
litellm_model    = "${LITELLM_MODEL:-claude-sonnet-4-5}"

mcp_server_url = "${MCP_SERVER_URL:-http://localhost:8080/rpc}"
mcp_auth_token = "${MCP_AUTH_TOKEN:-}"
EOF
    echo "✅ Created terraform.tfvars"
fi

# Upload orby code to GCS for VM deployment
echo ""
echo "📦 Uploading orby code to GCS..."
BUCKET_NAME="orby-deployment-artifacts"
if ! gsutil ls gs://${BUCKET_NAME} &>/dev/null; then
    echo "Creating GCS bucket ${BUCKET_NAME}..."
    gsutil mb -l europe-west1 gs://${BUCKET_NAME}
fi
echo "Syncing orby code..."
gsutil -m rsync -r -d "${SCRIPT_DIR}/../orby/" "gs://${BUCKET_NAME}/orby/"

# Initialize and apply terraform
cd "${TERRAFORM_DIR}"

echo ""
echo "📦 Initializing Terraform..."
terraform init

echo ""
echo "🔍 Planning changes..."
terraform plan -out=tfplan

echo ""
read -p "Apply these changes? (y/N) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "🚀 Applying..."
    terraform apply tfplan
    
    echo ""
    echo "✅ Deployment complete!"
    echo ""
    terraform output
    echo ""
    echo "📝 Useful commands:"
    echo "  SSH to VM:              $(terraform output -raw ssh_command)"
    echo "  Slack client logs:      $(terraform output -raw logs_command_slack)"
    echo "  MCP server logs:        $(terraform output -raw logs_command_mcp)"
    echo ""
    echo "Or use the Makefile from orby/:"
    echo "  make logs-slack         # View Slack client logs"
    echo "  make logs-mcp           # View MCP server logs"
    echo "  make status-vm          # Check both services"
    echo "  make restart-all        # Restart both services"
else
    echo "Cancelled."
    rm -f tfplan
fi
