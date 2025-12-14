#!/bin/bash
# Orby Slack Client + MCP Server - VM Startup Script
# - Slack client: Built from source (public repo)
# - MCP server: Docker container (pulled from GCR)

set -e

LOG_FILE="/var/log/orby-setup.log"
exec > >(tee -a "$LOG_FILE") 2>&1

echo "=== Orby Services Setup Started at $(date) ==="

# Fetch secrets from instance metadata first (needed for config)
echo "Fetching configuration from metadata..."
METADATA_URL="http://metadata.google.internal/computeMetadata/v1/instance/attributes"
METADATA_HEADER="Metadata-Flavor: Google"

SLACK_BOT_TOKEN=$(curl -s -H "$METADATA_HEADER" "$METADATA_URL/slack-bot-token")
SLACK_APP_TOKEN=$(curl -s -H "$METADATA_HEADER" "$METADATA_URL/slack-app-token")
SLACK_SIGNING_SECRET=$(curl -s -H "$METADATA_HEADER" "$METADATA_URL/slack-signing-secret")
LITELLM_API_KEY=$(curl -s -H "$METADATA_HEADER" "$METADATA_URL/litellm-api-key")
LITELLM_BASE_URL=$(curl -s -H "$METADATA_HEADER" "$METADATA_URL/litellm-base-url")
LITELLM_MODEL=$(curl -s -H "$METADATA_HEADER" "$METADATA_URL/litellm-model")
NOTION_API_KEY=$(curl -s -H "$METADATA_HEADER" "$METADATA_URL/notion-api-key")
MCP_SERVER_URL=$(curl -s -H "$METADATA_HEADER" "$METADATA_URL/mcp-server-url")
RAG_PERSIST_DIR=$(curl -s -H "$METADATA_HEADER" "$METADATA_URL/rag-persist-dir")
GCP_PROJECT_ID=$(curl -s -H "$METADATA_HEADER" "http://metadata.google.internal/computeMetadata/v1/project/project-id")

# Install dependencies
echo "Installing dependencies..."
apt-get update
apt-get install -y curl jq git docker.io nginx certbot python3-certbot-nginx

# Start Docker
systemctl enable docker
systemctl start docker

# Install Go 1.24+
echo "Installing Go..."
GO_VERSION="1.24.4"
curl -LO "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
rm -rf /usr/local/go
tar -C /usr/local -xzf "go${GO_VERSION}.linux-amd64.tar.gz"
rm "go${GO_VERSION}.linux-amd64.tar.gz"
export PATH=$PATH:/usr/local/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile

# Build slack-mcp-client from source (public repo)
echo "Building slack-mcp-client from source..."
rm -rf /opt/slack-client
mkdir -p /opt/slack-client
cd /opt/slack-client
git clone https://github.com/tolemy-bio/slack-mcp-client.git .
export HOME=/root
export GOPATH=/root/go
export GOCACHE=/root/.cache/go-build
/usr/local/go/bin/go build -o /usr/local/bin/slack-mcp-client ./cmd
chmod +x /usr/local/bin/slack-mcp-client

# Create config directory
mkdir -p /etc/orby

# Create Slack client config file
echo "Creating Slack client configuration..."
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
      "mode": "sse",
      "url": "${MCP_SERVER_URL}",
      "initialize_timeout_seconds": 30
    }
  },
  "agent": {
    "enabled": true,
    "maxIterations": 15,
    "systemPrompt": "Your name is Orby. You are Tolemy's friendly AI assistant who helps the team stay organized and productive.\n\nPERSONALITY:\n- Warm, approachable, and concise\n- You use casual language but stay professional\n- You celebrate wins ('Nice! Bug marked as solved! 🎉')\n- When asked who you are, say 'I'm Orby, Tolemy's AI assistant!'\n\nSLACK USER CONTEXT:\n- You can see the Slack user's display name in each message\n- When creating bugs/features/tasks, use their display name as submitted_by or assigned_to\n- If you see a user ID like <@U0A2WJUPW8K>, extract the display name from the message context\n- NEVER ask 'who should I list' or 'what's your name' - you already know from the message!\n\nSLACK FORMATTING (CRITICAL - Slack uses mrkdwn, NOT standard markdown):\n- Headings: Use *Bold text* instead of ## Headings\n- Links: Use <url|Link text> instead of [Link text](url)\n- Example: <https://notion.so/abc|View Meeting> not [View Meeting](https://notion.so/abc)\n- Bullet points work normally with - or •\n- Code: Use backticks normally\n\nCRITICAL - CREATING BUGS, FEATURES, AND TASKS:\nWhen users want to create bugs/features/tasks, DO NOT ask a list of questions! Instead:\n\n1. EXTRACT & INFER from their message:\n   - Problem/name, description, details\n   - Impact/priority from words like 'blocking'/'urgent'/'annoying'\n   - Frequency from 'always'/'sometimes'/'rarely'\n   - The user's display name for submitted_by/assigned_to\n   - Product area from context ('upload' → Experiments, 'search' → Discovery)\n   - Tags from keywords ('file upload' → file-upload tag)\n\n2. PRESENT A SUMMARY for confirmation (DO NOT CALL create_bug/create_feature YET):\n\nBug summary format:\n🐛 *New Bug Report*\n*Problem:* File upload fails for experiments\n*Details:* User tried to upload files to experiments. Expected files to upload successfully. Instead, upload fails/errors.\n*Risk:* Users cannot attach experimental data, blocking workflow\n💰 *Impact:* 4 (significant blocker)\n🕐 *Frequency:* 0.8 (happens most of the time)\n📊 *Value:* 3.2\n🏷️ *Tags:* file-upload, experiments\n📍 *Area:* Experiments\n📸 If you have a screenshot, share it!\n\n*Reply 'confirm' to create, or suggest changes*\n\nFeature summary format:\n✨ *New Feature Request*\n*Name:* Smart File Uploader\n*What:* Drag-and-drop file uploads with progress bar\n*Why:* Faster workflow, better UX for data entry\n💰 *Value:* 50 (useful improvement)\n⏱️ *Effort:* 3 days\n📍 *Area:* Experiments\n\n*Reply 'confirm' to create, or suggest changes*\n\nTask summary (simpler - can create immediately):\n✅ *Task Created*\n*Name:* Review API design document\n*Priority:* Medium\n*Due:* Friday, Dec 20\n<link|View in Notion>\n\n3. ONLY call create_bug/create_feature/create_task AFTER user confirms\n4. For tasks, you can create immediately if simple and clear\n\nBEHAVIOR:\n- Keep responses short and scannable\n- Use bullet points for lists\n- Include Notion links when showing results (use Slack link format!)\n- BE PROACTIVE - infer and summarize, don't interrogate users with questions"
  }
}
EOF

chmod 600 /etc/orby/slack-client-config.json

# Create Slack client systemd service
echo "Creating Slack client systemd service..."
cat > /etc/systemd/system/slack-mcp-client.service << EOF
[Unit]
Description=Orby Slack MCP Client
After=network-online.target orby-mcp-server.service
Wants=network-online.target
Requires=orby-mcp-server.service

[Service]
Type=simple
ExecStart=/usr/local/bin/slack-mcp-client --config /etc/orby/slack-client-config.json --metrics-port 9090
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
Environment=SLACK_BOT_TOKEN=${SLACK_BOT_TOKEN}
Environment=SLACK_APP_TOKEN=${SLACK_APP_TOKEN}
Environment=OPENAI_API_KEY=${LITELLM_API_KEY}
Environment=OPENAI_BASE_URL=${LITELLM_BASE_URL}

[Install]
WantedBy=multi-user.target
EOF

echo ""
echo "=== Setting up Orby MCP Server (Docker) ==="

# Configure Docker to use GCR
echo "Configuring Docker for GCR..."
gcloud auth configure-docker --quiet

# Pull the MCP server image from GCR
MCP_IMAGE="gcr.io/${GCP_PROJECT_ID}/orby-mcp-server:latest"
echo "Pulling MCP server image: ${MCP_IMAGE}"
docker pull ${MCP_IMAGE}

# Create RAG persist directory
echo "Creating RAG persist directory..."
mkdir -p "${RAG_PERSIST_DIR}"
chmod 755 "${RAG_PERSIST_DIR}"

# Create MCP server systemd service (runs Docker container)
echo "Creating MCP server systemd service..."
cat > /etc/systemd/system/orby-mcp-server.service << EOF
[Unit]
Description=Orby MCP Server (Docker)
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=simple
Restart=always
RestartSec=10

ExecStartPre=-/usr/bin/docker stop orby-mcp-server
ExecStartPre=-/usr/bin/docker rm orby-mcp-server

ExecStart=/usr/bin/docker run --rm --name orby-mcp-server \
  -p 127.0.0.1:8080:8080 \
  -v ${RAG_PERSIST_DIR}:/tmp/orby_chroma \
  -e SLACK_BOT_TOKEN=${SLACK_BOT_TOKEN} \
  -e SLACK_SIGNING_SECRET=${SLACK_SIGNING_SECRET} \
  -e SLACK_APP_TOKEN=${SLACK_APP_TOKEN} \
  -e NOTION_API_KEY=${NOTION_API_KEY} \
  -e LITELLM_BASE_URL=${LITELLM_BASE_URL} \
  -e LITELLM_API_KEY=${LITELLM_API_KEY} \
  -e LITELLM_MODEL=${LITELLM_MODEL} \
  -e RAG_PERSIST_DIR=/tmp/orby_chroma \
  ${MCP_IMAGE}

ExecStop=/usr/bin/docker stop orby-mcp-server

[Install]
WantedBy=multi-user.target
EOF

# Enable and start services
echo "Starting services..."
systemctl daemon-reload
systemctl enable orby-mcp-server
systemctl enable slack-mcp-client
systemctl start orby-mcp-server

# Wait for MCP server to be ready
echo "Waiting for MCP server to start..."
sleep 10

# Start Slack client
systemctl start slack-mcp-client

echo ""
echo "=== Setting up Nginx HTTPS Reverse Proxy ==="

# Create Nginx config for MCP server
cat > /etc/nginx/sites-available/orby-mcp << 'NGINX_EOF'
# Orby MCP Server - HTTPS Reverse Proxy
# Proxies HTTPS requests to the local MCP server on port 8080

server {
    listen 80;
    server_name orby.tolemy.bio;
    
    # Let's Encrypt challenge location
    location /.well-known/acme-challenge/ {
        root /var/www/html;
    }
    
    # Redirect all other HTTP to HTTPS
    location / {
        return 301 https://$server_name$request_uri;
    }
}

server {
    listen 443 ssl http2;
    server_name orby.tolemy.bio;
    
    # SSL certificates (will be created by certbot)
    ssl_certificate /etc/letsencrypt/live/orby.tolemy.bio/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/orby.tolemy.bio/privkey.pem;
    
    # Modern SSL config
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers off;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384;
    
    # HSTS
    add_header Strict-Transport-Security "max-age=31536000" always;
    
    # Proxy to MCP server
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # SSE support (for /mcp/sse endpoint)
        proxy_set_header Connection '';
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 86400s;
        chunked_transfer_encoding off;
    }
}
NGINX_EOF

# Enable the site
ln -sf /etc/nginx/sites-available/orby-mcp /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default

# Create initial Nginx config without SSL for certbot
cat > /etc/nginx/sites-available/orby-mcp-initial << 'NGINX_INIT_EOF'
server {
    listen 80;
    server_name orby.tolemy.bio;
    
    location /.well-known/acme-challenge/ {
        root /var/www/html;
    }
    
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
NGINX_INIT_EOF

# Start with initial config (no SSL yet)
ln -sf /etc/nginx/sites-available/orby-mcp-initial /etc/nginx/sites-enabled/orby-mcp
mkdir -p /var/www/html
systemctl enable nginx
systemctl restart nginx

# Attempt to get SSL certificate (will fail if DNS not pointing here yet)
echo "Attempting to obtain SSL certificate..."
if certbot certonly --nginx -d orby.tolemy.bio --non-interactive --agree-tos --email caelan@tolemy.bio --keep-until-expiring 2>&1; then
    echo "SSL certificate obtained! Switching to HTTPS config..."
    ln -sf /etc/nginx/sites-available/orby-mcp /etc/nginx/sites-enabled/orby-mcp
    systemctl reload nginx
    echo "✅ HTTPS enabled for orby.tolemy.bio"
else
    echo "⚠️  SSL certificate not obtained (DNS may not be pointing here yet)"
    echo "   Once DNS is configured, run: sudo certbot --nginx -d orby.tolemy.bio"
    echo "   For now, HTTP is enabled on port 80"
fi

# Set up auto-renewal cron job
echo "0 3 * * * root certbot renew --quiet --post-hook 'systemctl reload nginx'" > /etc/cron.d/certbot-renew

echo ""
echo "=== Orby Services Setup Completed at $(date) ==="
echo ""
echo "Slack Client:"
echo "  Check status: systemctl status slack-mcp-client"
echo "  View logs: journalctl -u slack-mcp-client -f"
echo ""
echo "MCP Server (Docker):"
echo "  Check status: systemctl status orby-mcp-server"
echo "  View logs: journalctl -u orby-mcp-server -f"
echo "  Docker logs: docker logs orby-mcp-server -f"
echo ""
echo "Both services are running - Slack client connects to MCP at localhost:8080"
