# Orby Slack Client - GCE VM Deployment

This directory contains Terraform configuration for deploying the Slack client on a GCE VM.

## Why GCE VM and not Cloud Run?

The `slack-mcp-client` uses Slack's **Socket Mode**, which requires a persistent WebSocket connection. Cloud Run is designed for HTTP request/response workloads and doesn't fit this use case well.

A small always-on VM is the proper solution:
- ✅ Native fit for long-running processes
- ✅ Simple, no hacks needed
- ✅ Free tier eligible (e2-micro)
- ✅ Cheap (~$6/month if not on free tier)

## Quick Start

### 1. Set Environment Variables

```bash
export SLACK_BOT_TOKEN=xoxb-xxx
export SLACK_APP_TOKEN=xapp-xxx  # From Socket Mode setup
export LITELLM_API_KEY=xxx
```

### 2. Deploy

```bash
cd /path/to/orby
make deploy-slack
```

Or run the deployment script directly:

```bash
./deploy.sh
```

## Configuration

### Option A: Environment Variables (Recommended)

Set the required variables and run `./deploy.sh`. Terraform vars will be created automatically.

### Option B: terraform.tfvars

```bash
cd terraform
cp terraform.tfvars.template terraform.tfvars
# Edit terraform.tfvars with your values
terraform init
terraform plan
terraform apply
```

## Files

| File | Purpose |
|------|---------|
| `deploy.sh` | Main deployment script (checks env vars, runs Terraform) |
| `terraform/main.tf` | Terraform config for GCE VM |
| `terraform/variables.tf` | Variable definitions |
| `terraform/startup.sh` | VM startup script (installs Go, slack-mcp-client, creates systemd service) |
| `terraform/terraform.tfvars.template` | Template for configuration |

## What Gets Created

- **GCE Instance**: `orby-slack-client` in `europe-west1-b`
- **Machine Type**: `e2-micro` (free tier eligible)
- **Service Account**: Minimal permissions
- **Systemd Service**: `slack-mcp-client.service` that runs on boot
- **Config**: `/etc/orby/slack-client-config.json` (contains secrets, only readable by root)

## Management Commands

From the `orby/` directory:

```bash
# View logs
make logs-slack

# Follow logs live
make logs-slack-follow

# Check status
make status-slack

# SSH into VM
make ssh-slack

# Restart service
make restart-slack

# Destroy VM
make destroy-slack
```

## Troubleshooting

### Check if service is running

```bash
make ssh-slack
sudo systemctl status slack-mcp-client
```

### View logs

```bash
make logs-slack-follow
```

### Restart service

```bash
make restart-slack
```

### Check configuration

```bash
make ssh-slack
sudo cat /etc/orby/slack-client-config.json
```

### Manual Terraform Operations

```bash
cd terraform

# View current state
terraform show

# Plan changes
terraform plan

# Apply changes
terraform apply

# Destroy
terraform destroy
```

## Cost

- **e2-micro**: Free tier includes 1 instance per month
- **If not on free tier**: ~$6/month
- **Disk**: 10GB standard persistent disk (~$0.40/month)
- **Total**: ~$6-7/month (or free with GCP free tier)

## Security

Secrets are stored:
1. In Terraform state (encrypted if using remote backend)
2. In GCE instance metadata (encrypted at rest)
3. In `/etc/orby/slack-client-config.json` on the VM (chmod 600, root only)

**Best practice**: Use remote Terraform state with encryption and state locking.

## Updating the Service

### Update slack-mcp-client version

SSH into the VM and run:

```bash
sudo systemctl stop slack-mcp-client
sudo /usr/local/go/bin/go install github.com/tuannvm/slack-mcp-client/cmd@latest
sudo mv /root/go/bin/cmd /usr/local/bin/slack-mcp-client
sudo systemctl start slack-mcp-client
```

### Update configuration

Edit the Terraform variables and re-apply:

```bash
terraform apply
```

The startup script will regenerate the config on next boot. To apply immediately:

```bash
make ssh-slack
sudo systemctl restart slack-mcp-client
```



