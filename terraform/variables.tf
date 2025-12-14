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
  description = "VM machine type (e2-micro is free tier eligible)"
  type        = string
  default     = "e2-micro"  # Free tier: 1 e2-micro per month
}

# Slack credentials
variable "slack_bot_token" {
  description = "Slack Bot Token (xoxb-...)"
  type        = string
  sensitive   = true
}

variable "slack_app_token" {
  description = "Slack App Token for Socket Mode (xapp-...)"
  type        = string
  sensitive   = true
}

# LLM credentials (LiteLLM)
variable "litellm_api_key" {
  description = "LiteLLM API Key"
  type        = string
  sensitive   = true
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

# MCP Server
variable "mcp_server_url" {
  description = "URL of the MCP server"
  type        = string
  default     = "https://orby-mcp-server-228973215278.europe-west1.run.app/mcp"
}

variable "mcp_auth_token" {
  description = "MCP server authentication token"
  type        = string
  sensitive   = true
  default     = ""
}

variable "custom_prompt" {
  description = "Custom system prompt for the Slack bot (optional)"
  type        = string
  default     = ""
}

