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

# Grant minimal permissions (only needs to call the MCP server, which is public)
# No special GCP permissions needed

# The VM instance
resource "google_compute_instance" "slack_client" {
  name         = "orby-slack-client"
  machine_type = var.machine_type
  zone         = var.zone

  tags = ["orby-slack-client"]

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-12"
      size  = 10  # GB - minimal disk
      type  = "pd-standard"
    }
  }

  network_interface {
    network = "default"
    
    # Ephemeral public IP (needed to reach Slack and MCP server)
    access_config {}
  }

  # Pass secrets and startup script via metadata (encrypted at rest)
  metadata = {
    # Slack credentials (used by both Slack client and MCP server)
    slack-bot-token      = var.slack_bot_token
    slack-app-token      = var.slack_app_token
    slack-signing-secret = var.slack_signing_secret
    
    # LiteLLM credentials (used by both services)
    litellm-api-key  = var.litellm_api_key
    litellm-base-url = var.litellm_base_url
    litellm-model    = var.litellm_model
    
    # Notion credentials (used by MCP server)
    notion-api-key = var.notion_api_key
    
    # MCP server configuration
    mcp-server-url   = var.mcp_server_url
    mcp-auth-token   = var.mcp_auth_token
    rag-persist-dir  = var.rag_persist_dir
    
    startup-script   = file("${path.module}/startup.sh")
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



