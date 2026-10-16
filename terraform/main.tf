# Orby Slack Client + MCP Server - GCE VM
# A small always-on VM to run both slack-mcp-client and orby-mcp-server

terraform {
  required_version = ">= 1.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

# Service account for the VM
resource "google_service_account" "slack_client" {
  account_id   = "orby-slack-client"
  display_name = "Orby Slack Client VM"
  description  = "Service account for the Slack MCP client VM"
}

# Secrets. Terraform owns the containers and who may read them, never the values: a
# value set here would sit in the state file. Add each one by hand, e.g.
#   printf %s "$VALUE" | gcloud secrets versions add orby-linear-api-key --data-file=-
# The first seven are required by startup.sh; the rest switch a feature on when present.
locals {
  orby_secrets = toset([
    "slack-bot-token",
    "slack-app-token",
    "slack-signing-secret",
    "litellm-api-key",
    "notion-api-key",
    "mcp-auth-token",
    "linear-api-key",
    "langfuse-secret-key",
    "linear-policy-github-token",
    "orbit-aws-access-key-id",
    "orbit-aws-secret-access-key",
    "newswatch-session-secret",
    "newswatch-google-client-secret",
    "github-app-private-key",
    "brave-api-key",
    "consensus-api-key",
  ])
}

resource "google_secret_manager_secret" "orby" {
  for_each  = local.orby_secrets
  secret_id = "orby-${each.key}"

  replication {
    auto {}
  }

  labels = {
    app = "orby"
  }
}

# The VM may read these secrets and nothing else in Secret Manager.
resource "google_secret_manager_secret_iam_member" "orby_vm" {
  for_each  = google_secret_manager_secret.orby
  secret_id = each.value.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.slack_client.email}"
}

# The VM instance
resource "google_compute_instance" "slack_client" {
  name         = "orby-slack-client"
  machine_type = var.machine_type
  zone         = var.zone

  tags = ["orby-slack-client", "orby-mcp-server"]

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-12"
      size  = 20  # GB; resized live from 10 on 2026-10-16. Each Orby deploy leaves an image of up to ~1 GB
      type  = "pd-standard"
    }
  }

  network_interface {
    network = "default"
    
    # Ephemeral public IP (needed to reach Slack and MCP server)
    access_config {}
  }

  # Plain configuration and the startup script. No secret belongs here: instance
  # metadata is readable by anyone with compute.instances.get on the project and by
  # every process on the VM. startup.sh reads secrets from Secret Manager instead.
  metadata = {
    litellm-base-url = var.litellm_base_url
    litellm-model    = var.litellm_model
    vision-model     = var.vision_model

    langfuse-public-key = var.langfuse_public_key
    langfuse-host       = var.langfuse_host

    xero-client-id    = var.xero_client_id
    xero-redirect-uri = var.xero_redirect_uri

    orbit-aws-region = var.orbit_aws_region
    linear-team-id   = var.linear_team_id
    github-app-id    = var.github_app_id

    newswatch-gcs-backup-bucket = var.newswatch_gcs_backup_bucket
    newswatch-allowed-domains   = var.newswatch_allowed_domains
    newswatch-google-client-id  = var.newswatch_google_client_id

    mcp-server-url  = var.mcp_server_url
    rag-persist-dir = var.rag_persist_dir
    disable-rag     = var.disable_rag ? "true" : "false"

    startup-script = file("${path.module}/startup.sh")
  }

  service_account {
    email  = google_service_account.slack_client.email
    scopes = ["cloud-platform"]
  }

  # Allow the VM to be stopped for cost savings during development
  allow_stopping_for_update = true

  # Scheduling options for cost optimization
  scheduling {
    automatic_restart   = true
    on_host_maintenance = "MIGRATE"
    preemptible         = false  # Set to true for ~80% cost savings (but VM can be preempted)
  }

  labels = {
    app     = "orby"
    purpose = "slack-client-and-mcp-server"
  }

  # Lifecycle: prefer updates over replacements for faster deploys
  # When metadata changes, update in-place and reboot instead of destroying VM
  lifecycle {
    create_before_destroy = false
    # Don't force replacement on metadata changes
    ignore_changes = []
  }
}

# Output the VM details
output "instance_name" {
  value = google_compute_instance.slack_client.name
}

output "instance_zone" {
  value = google_compute_instance.slack_client.zone
}

output "external_ip" {
  value = google_compute_instance.slack_client.network_interface[0].access_config[0].nat_ip
}

output "ssh_command" {
  value = "gcloud compute ssh ${google_compute_instance.slack_client.name} --zone=${google_compute_instance.slack_client.zone} --project=${var.project_id}"
}

output "logs_command_slack" {
  value = "gcloud compute ssh ${google_compute_instance.slack_client.name} --zone=${google_compute_instance.slack_client.zone} --project=${var.project_id} --command='sudo journalctl -u slack-mcp-client -f'"
}

output "logs_command_mcp" {
  value = "gcloud compute ssh ${google_compute_instance.slack_client.name} --zone=${google_compute_instance.slack_client.zone} --project=${var.project_id} --command='sudo journalctl -u orby-mcp-server -f'"
}

# Firewall rule for HTTPS (443) and HTTP (80 for Let's Encrypt validation)
resource "google_compute_firewall" "allow_mcp_server" {
  name    = "allow-orby-mcp-server"
  network = "default"

  allow {
    protocol = "tcp"
    ports    = ["80", "443"]
  }

  # Allow from anywhere (MCP clients can be anywhere)
  source_ranges = ["0.0.0.0/0"]
  
  # Only apply to VMs with the orby-mcp-server tag
  target_tags = ["orby-mcp-server"]
  
  description = "Allow HTTPS access to Orby MCP Server via Nginx"
}

output "mcp_server_url" {
  value = "https://orby.tolemy.bio"
  description = "MCP Server URL (HTTPS via Nginx)"
}

output "mcp_server_endpoint" {
  value = "https://orby.tolemy.bio/mcp/sse"
  description = "MCP Server SSE endpoint for clients (Cursor, Claude Desktop)"
}

