# Variables for Orby Slack Client VM

variable "project_id" {
  description = "GCP Project ID"
  type        = string
  default     = "gen-lang-client-0335698828"
}

variable "region" {
  description = "GCP Region"
  type        = string
  default     = "europe-west1"
}

variable "zone" {
  description = "GCP Zone"
  type        = string
  default     = "europe-west1-b"
}

variable "machine_type" {
  description = "VM machine type (e2-small recommended for running both services)"
  type        = string
  default     = "e2-small"  # 2GB RAM for Slack client + MCP server + ChromaDB
}

variable "litellm_base_url" {
  description = "LiteLLM Base URL"
  type        = string
  default     = "https://litellm.tolemy.bio/v1"
}

variable "litellm_model" {
  description = "LLM Model to use"
  type        = string
  default     = "claude-sonnet-4-5"  # Upgraded from Haiku for better reasoning
}

variable "vision_model" {
  description = "LiteLLM model alias used to describe Slack image attachments (screenshots) before the agent runs"
  type        = string
  default     = "claude-sonnet-5"
}

# MCP Server (now co-located on VM)
variable "mcp_server_url" {
  description = "URL of the MCP server HTTP endpoint (co-located on localhost)"
  type        = string
  default     = "http://127.0.0.1:8081/mcp/orby" # via the local nginx proxy that sets X-Local-Request
}

variable "custom_prompt" {
  description = "Custom system prompt for the Slack bot (optional)"
  type        = string
  default     = ""
}

# Langfuse (the secret key is in Secret Manager)
variable "langfuse_public_key" {
  description = "Langfuse Public API Key"
  type        = string
  sensitive   = true
  default     = ""
}

variable "langfuse_host" {
  description = "Langfuse API Host URL"
  type        = string
  default     = "https://cloud.langfuse.com"
}

# Xero credentials (for orby-cash MCP)
variable "xero_client_id" {
  description = "Xero OAuth Client ID"
  type        = string
  sensitive   = true
  default     = ""
}

variable "xero_redirect_uri" {
  description = "Xero OAuth Redirect URI"
  type        = string
  default     = "http://localhost:8080/xero/callback"
}

# Orbit AWS region (the access keys are in Secret Manager)
variable "orbit_aws_region" {
  description = "AWS region for Orbit DynamoDB/S3"
  type        = string
  default     = "eu-west-2"
}

# RAG configuration
variable "rag_persist_dir" {
  description = "Directory for ChromaDB vector store persistence"
  type        = string
  default     = "/var/lib/orby/chroma"
}

variable "disable_rag" {
  description = "Disable RAG/semantic search (avoids embedding API calls)"
  type        = bool
  default     = true  # Disabled by default to avoid embedding rate limits
}

# Domain configuration
variable "domain_name" {
  description = "Domain name for HTTPS (e.g., orby.tolemy.bio)"
  type        = string
  default     = "orby.tolemy.bio"
}

variable "admin_email" {
  description = "Admin email for Let's Encrypt notifications"
  type        = string
  default     = "caelan@tolemy.bio"
}

# Linear
variable "linear_team_id" {
  description = "Linear team key that Orby files issues into"
  type        = string
  default     = "TOL"
}

# GitHub App Orby uses to start the coding agent (the private key is in Secret Manager)
variable "github_app_id" {
  description = "App ID of the Orby GitHub App; empty switches the start_agent_pr tool off"
  type        = string
  default     = "5296072"
}

# NewsWatch (the session secret and the OAuth client secret are in Secret Manager)
variable "newswatch_gcs_backup_bucket" {
  description = "Bucket NewsWatch backs its database up to"
  type        = string
  default     = "orby-newswatch-backups"
}

variable "newswatch_allowed_domains" {
  description = "Comma-separated email domains allowed into the NewsWatch admin UI"
  type        = string
  default     = "tolemy.bio"
}

variable "newswatch_google_client_id" {
  description = "Google OAuth client ID for the NewsWatch admin UI"
  type        = string
  default     = ""
}
