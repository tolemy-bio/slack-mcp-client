#!/bin/bash
set -e

# Orby Slack Client - GCE VM Deployment Script
# Deploys the slack-mcp-client on a GCE VM using Terraform

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TERRAFORM_DIR="${SCRIPT_DIR}/terraform"

echo "🚀 Deploying Orby Slack Client (GCE VM)"
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

machine_type = "e2-micro"

slack_bot_token = "${SLACK_BOT_TOKEN}"
slack_app_token = "${SLACK_APP_TOKEN}"

litellm_api_key  = "${LITELLM_API_KEY}"
litellm_base_url = "${LITELLM_BASE_URL:-https://litellm.tolemy.bio/v1}"
litellm_model    = "${LITELLM_MODEL:-claude-3-5-haiku-20241022}"

mcp_server_url = "${MCP_SERVER_URL:-https://orby-mcp-server-228973215278.europe-west1.run.app/mcp}"
mcp_auth_token = "${MCP_AUTH_TOKEN:-}"
EOF
    echo "✅ Created terraform.tfvars"
fi

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
    echo "  View logs:    $(terraform output -raw logs_command)"
    echo "  SSH to VM:    $(terraform output -raw ssh_command)"
else
    echo "Cancelled."
    rm -f tfplan
fi
