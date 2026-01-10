#!/bin/bash

# Script to extract credentials from Terraform and populate .env.testing
# This script reads from terraform.tfvars or terraform state/outputs

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TERRAFORM_DIR="$SCRIPT_DIR/terraform"
ENV_FILE="$SCRIPT_DIR/.env.testing"

echo "=========================================="
echo "Extracting Credentials from Terraform"
echo "=========================================="
echo ""

# Check if terraform directory exists
if [ ! -d "$TERRAFORM_DIR" ]; then
    echo "❌ ERROR: Terraform directory not found: $TERRAFORM_DIR"
    exit 1
fi

cd "$TERRAFORM_DIR"

# Function to extract value from terraform.tfvars
extract_from_tfvars() {
    local var_name=$1
    local tfvars_file="terraform.tfvars"
    
    if [ -f "$tfvars_file" ]; then
        # Extract value using awk - more reliable for parsing
        # Match: var_name = "value" or var_name = 'value' or var_name = value
        awk -v var="$var_name" '
            BEGIN { FS = "=" }
            $1 ~ "^[[:space:]]*" var "[[:space:]]*$" {
                # Remove leading/trailing whitespace and quotes
                gsub(/^[[:space:]]+|[[:space:]]+$/, "", $2)
                gsub(/^["'\'']|["'\'']$/, "", $2)
                print $2
                exit
            }
        ' "$tfvars_file" 2>/dev/null || echo ""
    else
        echo ""
    fi
}

# Function to extract from terraform state/outputs
extract_from_terraform() {
    local var_name=$1
    
    # Try to get from terraform output (if it's an output)
    terraform output -raw "$var_name" 2>/dev/null || echo ""
}

# Function to get from terraform show (for variables in state)
extract_from_state() {
    local var_name=$1
    
    # Try terraform show to get variable values
    terraform show -json 2>/dev/null | \
        jq -r ".values.root_module.variables.${var_name}.value // empty" 2>/dev/null || echo ""
}

# Initialize variables
SLACK_BOT_TOKEN=""
SLACK_APP_TOKEN=""
OPENAI_API_KEY=""
LOG_LEVEL="debug"
MCP_FILESYSTEM_PATH="${HOME}"

# Try to extract Slack Bot Token
echo "🔍 Looking for SLACK_BOT_TOKEN..."
if [ -f "terraform.tfvars" ]; then
    SLACK_BOT_TOKEN=$(extract_from_tfvars "slack_bot_token")
fi
if [ -z "$SLACK_BOT_TOKEN" ]; then
    SLACK_BOT_TOKEN=$(extract_from_state "slack_bot_token")
fi
if [ -n "$SLACK_BOT_TOKEN" ] && [ "$SLACK_BOT_TOKEN" != "xoxb-your-bot-token" ]; then
    echo "   ✅ Found: ${SLACK_BOT_TOKEN:0:20}..."
else
    echo "   ⚠️  Not found or using placeholder"
    SLACK_BOT_TOKEN=""
fi

# Try to extract Slack App Token
echo "🔍 Looking for SLACK_APP_TOKEN..."
if [ -f "terraform.tfvars" ]; then
    SLACK_APP_TOKEN=$(extract_from_tfvars "slack_app_token")
fi
if [ -z "$SLACK_APP_TOKEN" ]; then
    SLACK_APP_TOKEN=$(extract_from_state "slack_app_token")
fi
if [ -n "$SLACK_APP_TOKEN" ] && [ "$SLACK_APP_TOKEN" != "xapp-your-app-token" ]; then
    echo "   ✅ Found: ${SLACK_APP_TOKEN:0:20}..."
else
    echo "   ⚠️  Not found or using placeholder"
    SLACK_APP_TOKEN=""
fi

# Try to extract API Key (could be litellm_api_key or openai_api_key)
echo "🔍 Looking for API key (LiteLLM or OpenAI)..."
if [ -f "terraform.tfvars" ]; then
    OPENAI_API_KEY=$(extract_from_tfvars "litellm_api_key")
    if [ -z "$OPENAI_API_KEY" ]; then
        OPENAI_API_KEY=$(extract_from_tfvars "openai_api_key")
    fi
fi
if [ -z "$OPENAI_API_KEY" ]; then
    OPENAI_API_KEY=$(extract_from_state "litellm_api_key")
fi
if [ -z "$OPENAI_API_KEY" ]; then
    OPENAI_API_KEY=$(extract_from_state "openai_api_key")
fi
if [ -n "$OPENAI_API_KEY" ] && [ "$OPENAI_API_KEY" != "your-litellm-api-key" ] && [ "$OPENAI_API_KEY" != "your-openai-api-key" ]; then
    echo "   ✅ Found: ${OPENAI_API_KEY:0:20}..."
    echo "   ℹ️  Note: Using LiteLLM API key. If you need OpenAI directly, set OPENAI_API_KEY manually."
else
    echo "   ⚠️  Not found or using placeholder"
    OPENAI_API_KEY=""
fi

# Try to extract LiteLLM Base URL
echo "🔍 Looking for LiteLLM Base URL..."
if [ -f "terraform.tfvars" ]; then
    LITELLM_BASE_URL=$(extract_from_tfvars "litellm_base_url")
fi
if [ -z "$LITELLM_BASE_URL" ]; then
    LITELLM_BASE_URL=$(extract_from_state "litellm_base_url")
fi
if [ -n "$LITELLM_BASE_URL" ] && [ "$LITELLM_BASE_URL" != "your-litellm-base-url" ]; then
    echo "   ✅ Found: $LITELLM_BASE_URL"
else
    echo "   ⚠️  Not found, will use default OpenAI endpoint"
    LITELLM_BASE_URL=""
fi

# Try to extract LiteLLM Model
echo "🔍 Looking for LiteLLM Model..."
if [ -f "terraform.tfvars" ]; then
    LITELLM_MODEL=$(extract_from_tfvars "litellm_model")
fi
if [ -z "$LITELLM_MODEL" ]; then
    LITELLM_MODEL=$(extract_from_state "litellm_model")
fi
if [ -n "$LITELLM_MODEL" ] && [ "$LITELLM_MODEL" != "your-litellm-model" ]; then
    echo "   ✅ Found: $LITELLM_MODEL"
else
    echo "   ⚠️  Not found, will use default model (gpt-4o)"
    LITELLM_MODEL="gpt-4o"  # Set default if not found
fi

echo ""
echo "=========================================="
echo "Writing to .env.testing"
echo "=========================================="
echo ""

# Create or update .env.testing
cd "$SCRIPT_DIR"

cat > "$ENV_FILE" << EOF
# Slack MCP Client Testing Environment Variables
# Auto-generated from Terraform on $(date)
# Edit this file to add/override values

# Required: Slack Bot Token (starts with xoxb-)
SLACK_BOT_TOKEN=${SLACK_BOT_TOKEN}

# Required: Slack App Token (starts with xapp-)
SLACK_APP_TOKEN=${SLACK_APP_TOKEN}

# Required: OpenAI API Key (starts with sk-)
# Note: If using LiteLLM, this may be your LiteLLM API key
OPENAI_API_KEY=${OPENAI_API_KEY}

# Optional: LiteLLM Base URL (if using LiteLLM proxy)
LITELLM_BASE_URL=${LITELLM_BASE_URL}

# Optional: LiteLLM Model (if using LiteLLM proxy)
LITELLM_MODEL=${LITELLM_MODEL}

# Optional: Log Level (debug, info, warn, error)
# Default: debug
LOG_LEVEL=${LOG_LEVEL}

# Optional: Home directory for filesystem MCP server
# Default: \$HOME
MCP_FILESYSTEM_PATH=${MCP_FILESYSTEM_PATH}
EOF

echo "✅ Created/updated $ENV_FILE"
echo ""

# Check if all required values are present
MISSING_VARS=()
if [ -z "$SLACK_BOT_TOKEN" ]; then
    MISSING_VARS+=("SLACK_BOT_TOKEN")
fi
if [ -z "$SLACK_APP_TOKEN" ]; then
    MISSING_VARS+=("SLACK_APP_TOKEN")
fi
if [ -z "$OPENAI_API_KEY" ]; then
    MISSING_VARS+=("OPENAI_API_KEY")
fi

if [ ${#MISSING_VARS[@]} -gt 0 ]; then
    echo "⚠️  Warning: Some required variables are missing:"
    for var in "${MISSING_VARS[@]}"; do
        echo "   - $var"
    done
    echo ""
    echo "Please edit $ENV_FILE and fill in the missing values."
    echo ""
    echo "You can:"
    echo "  1. Check terraform.tfvars in the terraform/ directory"
    echo "  2. Run 'terraform output' in the terraform/ directory"
    echo "  3. Manually edit $ENV_FILE"
    echo ""
    exit 1
else
    echo "✅ All required variables found!"
    echo ""
    echo "You can now run:"
    echo "  ./test-multi-step.sh"
    echo ""
fi
