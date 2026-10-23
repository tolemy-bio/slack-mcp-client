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

# No secret passes through this script, terraform.tfvars or instance metadata any more.
# The VM reads its credentials from Secret Manager (orby-<name>, declared in
# terraform/main.tf); terraform.tfvars is only needed to override plain configuration.

# Build and push MCP server Docker image
echo ""
echo "🐳 Building MCP server Docker image..."
PROJECT_ID="${GCP_PROJECT_ID:-gen-lang-client-0335698828}"
IMAGE_NAME="gcr.io/${PROJECT_ID}/orby-mcp-server"
ORBY_DIR="${SCRIPT_DIR}/../orby"

# Build using Cloud Build (works from Mac, builds for Linux)
cd "${ORBY_DIR}"

# Create temporary cloudbuild.yaml
cat > /tmp/cloudbuild-orby.yaml << YAML
steps:
  - name: 'gcr.io/cloud-builders/docker'
    args: ['build', '-t', '${IMAGE_NAME}', '-f', 'deploy/Dockerfile', '.']
images: ['${IMAGE_NAME}']
YAML

gcloud builds submit --config=/tmp/cloudbuild-orby.yaml --project "${PROJECT_ID}" .
rm -f /tmp/cloudbuild-orby.yaml
echo "✅ Docker image pushed to ${IMAGE_NAME}"

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
