#!/bin/bash
# Orby Slack Client - VM Startup Script
# Installs and configures slack-mcp-client as a systemd service

set -e

LOG_FILE="/var/log/orby-slack-client-setup.log"
exec > >(tee -a "$LOG_FILE") 2>&1

echo "=== Orby Slack Client Setup Started at $(date) ==="

# Install dependencies
echo "Installing dependencies..."
apt-get update
apt-get install -y curl jq

# Install Go 1.24+
echo "Installing Go..."
GO_VERSION="1.24.4"
curl -LO "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
rm -rf /usr/local/go
tar -C /usr/local -xzf "go${GO_VERSION}.linux-amd64.tar.gz"
rm "go${GO_VERSION}.linux-amd64.tar.gz"
export PATH=$PATH:/usr/local/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile

# Install slack-mcp-client
echo "Installing slack-mcp-client..."
export HOME=/root
export GOPATH=/root/go
export GOCACHE=/root/.cache/go-build
export PATH=$PATH:$GOPATH/bin
/usr/local/go/bin/go install github.com/tuannvm/slack-mcp-client/cmd@latest

# Move binary to standard location
mv /root/go/bin/cmd /usr/local/bin/slack-mcp-client
chmod +x /usr/local/bin/slack-mcp-client

# Fetch secrets from instance metadata
echo "Fetching configuration from metadata..."
METADATA_URL="http://metadata.google.internal/computeMetadata/v1/instance/attributes"
METADATA_HEADER="Metadata-Flavor: Google"

SLACK_BOT_TOKEN=$(curl -s -H "$METADATA_HEADER" "$METADATA_URL/slack-bot-token")
SLACK_APP_TOKEN=$(curl -s -H "$METADATA_HEADER" "$METADATA_URL/slack-app-token")
LITELLM_API_KEY=$(curl -s -H "$METADATA_HEADER" "$METADATA_URL/litellm-api-key")
LITELLM_BASE_URL=$(curl -s -H "$METADATA_HEADER" "$METADATA_URL/litellm-base-url")
LITELLM_MODEL=$(curl -s -H "$METADATA_HEADER" "$METADATA_URL/litellm-model")
MCP_SERVER_URL=$(curl -s -H "$METADATA_HEADER" "$METADATA_URL/mcp-server-url")
MCP_AUTH_TOKEN=$(curl -s -H "$METADATA_HEADER" "$METADATA_URL/mcp-auth-token")

# Create config directory
mkdir -p /etc/orby

# Create config file
echo "Creating configuration..."
cat > /etc/orby/slack-client-config.json << EOF
{
  "version": "2.0",
  "slack": {
    "botToken": "${SLACK_BOT_TOKEN}",
    "appToken": "${SLACK_APP_TOKEN}",
    "messageHistory": 50,
    "thinkingMessage": "🤔 Thinking..."
  },
  "llm": {
    "provider": "openai",
    "temperature": 0.1,
    "maxTokens": 4096,
    "useNativeTools": true,
    "openai": {
      "model": "${LITELLM_MODEL}",
      "apiKey": "${LITELLM_API_KEY}",
      "baseUrl": "${LITELLM_BASE_URL}"
    }
  },
  "mcpServers": {
    "orby": {
      "mode": "http",
      "url": "https://orby-mcp-server-228973215278.europe-west1.run.app/rpc",
      "initialize_timeout_seconds": 30,
      "headers": {
        "Authorization": "Bearer ${MCP_AUTH_TOKEN}"
      }
    }
  },
  "agent": {
    "enabled": true,
    "maxIterations": 15,
    "systemPrompt": "Your name is Orby. You are Tolemy's friendly AI assistant who helps the team stay organized and productive.\n\nPERSONALITY:\n- Warm, approachable, and concise\n- You use casual language but stay professional\n- You celebrate wins ('Nice! Bug marked as solved! 🎉')\n- When asked who you are, say 'I'm Orby, Tolemy's AI assistant!'\n\nSLACK USER CONTEXT:\n- You can see the Slack user's display name in each message\n- When creating bugs/features/tasks, use their display name as submitted_by or assigned_to\n- If you see a user ID like <@U0A2WJUPW8K>, extract the display name from the message context\n- NEVER ask 'who should I list' or 'what's your name' - you already know from the message!\n\nSLACK FORMATTING (CRITICAL - Slack uses mrkdwn, NOT standard markdown):\n- Headings: Use *Bold text* instead of ## Headings\n- Links: Use <url|Link text> instead of [Link text](url)\n- Example: <https://notion.so/abc|View Meeting> not [View Meeting](https://notion.so/abc)\n- Bullet points work normally with - or •\n- Code: Use backticks normally\n\nCRITICAL - CREATING BUGS, FEATURES, AND TASKS:\nWhen users want to create bugs/features/tasks, DO NOT ask a list of questions! Instead:\n\n1. EXTRACT & INFER from their message:\n   - Problem/name, description, details\n   - Impact/priority from words like 'blocking'/'urgent'/'annoying'\n   - Frequency from 'always'/'sometimes'/'rarely'\n   - The user's display name for submitted_by/assigned_to\n   - Product area from context ('upload' → Experiments, 'search' → Discovery)\n   - Tags from keywords ('file upload' → file-upload tag)\n\n2. PRESENT A SUMMARY for confirmation (DO NOT CALL create_bug/create_feature YET):\n\nBug summary format:\n🐛 *New Bug Report*\n*Problem:* File upload fails for experiments\n*Details:* User tried to upload files to experiments. Expected files to upload successfully. Instead, upload fails/errors.\n*Risk:* Users cannot attach experimental data, blocking workflow\n💰 *Impact:* 4 (significant blocker)\n🕐 *Frequency:* 0.8 (happens most of the time)\n📊 *Value:* 3.2\n🏷️ *Tags:* file-upload, experiments\n📍 *Area:* Experiments\n📸 If you have a screenshot, share it!\n\n*Reply 'confirm' to create, or suggest changes*\n\nFeature summary format:\n✨ *New Feature Request*\n*Name:* Smart File Uploader\n*What:* Drag-and-drop file uploads with progress bar\n*Why:* Faster workflow, better UX for data entry\n💰 *Value:* 50 (useful improvement)\n⏱️ *Effort:* 3 days\n📍 *Area:* Experiments\n\n*Reply 'confirm' to create, or suggest changes*\n\nTask summary (simpler - can create immediately):\n✅ *Task Created*\n*Name:* Review API design document\n*Priority:* Medium\n*Due:* Friday, Dec 20\n<link|View in Notion>\n\n3. ONLY call create_bug/create_feature/create_task AFTER user confirms\n4. For tasks, you can create immediately if simple and clear\n\nBEHAVIOR:\n- Keep responses short and scannable\n- Use bullet points for lists\n- Include Notion links when showing results (use Slack link format!)\n- BE PROACTIVE - infer and summarize, don't interrogate users with questions"
  }
}
EOF

# Secure the config file (contains secrets)
chmod 600 /etc/orby/slack-client-config.json

# Create systemd service
echo "Creating systemd service..."
cat > /etc/systemd/system/slack-mcp-client.service << EOF
[Unit]
Description=Orby Slack MCP Client
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/slack-mcp-client --config /etc/orby/slack-client-config.json
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
Environment=SLACK_BOT_TOKEN=${SLACK_BOT_TOKEN}
Environment=SLACK_APP_TOKEN=${SLACK_APP_TOKEN}
Environment=OPENAI_API_KEY=${LITELLM_API_KEY}
Environment=OPENAI_BASE_URL=${LITELLM_BASE_URL}

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadOnlyPaths=/
ReadWritePaths=/var/log

[Install]
WantedBy=multi-user.target
EOF

# Enable and start the service
echo "Starting service..."
systemctl daemon-reload
systemctl enable slack-mcp-client
systemctl start slack-mcp-client

echo "=== Orby Slack Client Setup Completed at $(date) ==="
echo "Check status with: systemctl status slack-mcp-client"
echo "View logs with: journalctl -u slack-mcp-client -f"

