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

# MCP Server (now co-located on VM)
variable "mcp_server_url" {
  description = "URL of the MCP server SSE endpoint (co-located on localhost)"
  type        = string
  default     = "http://localhost:8080/mcp/sse"
}

variable "mcp_auth_token" {
  description = "MCP server authentication token (deprecated - not needed for localhost)"
  type        = string
  sensitive   = true
  default     = ""
}

variable "custom_prompt" {
  description = "Custom system prompt for the Slack bot (optional)"
  type        = string
  default     = ""
}

# Notion credentials (for MCP server)
variable "notion_api_key" {
  description = "Notion API Key"
  type        = string
  sensitive   = true
}

variable "slack_signing_secret" {
  description = "Slack Signing Secret (for MCP server)"
  type        = string
  sensitive   = true
}

# RAG configuration
variable "rag_persist_dir" {
  description = "Directory for ChromaDB vector store persistence"
  type        = string
  default     = "/var/lib/orby/chroma"
}

