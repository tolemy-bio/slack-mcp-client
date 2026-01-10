# Terraform Credentials Extraction

## Overview

The `extract-terraform-creds.sh` script automatically extracts credentials from your Terraform configuration and populates `.env.testing` for local testing.

## Usage

Simply run:

```bash
./extract-terraform-creds.sh
```

This will:
1. Read credentials from `terraform/terraform.tfvars`
2. Extract Slack bot token, app token, and API key
3. Create/update `.env.testing` with the extracted values
4. Verify all required variables are present

## What Gets Extracted

- **SLACK_BOT_TOKEN** - From `slack_bot_token` in terraform.tfvars
- **SLACK_APP_TOKEN** - From `slack_app_token` in terraform.tfvars  
- **OPENAI_API_KEY** - From `litellm_api_key` in terraform.tfvars (or `openai_api_key` if present)

## Notes

- The script uses **LiteLLM API key** as the `OPENAI_API_KEY` since that's what's configured in Terraform
- If you need a direct OpenAI API key instead, manually edit `.env.testing` after extraction
- The `.env.testing` file is in `.gitignore` and won't be committed
- You can re-run the script anytime to refresh credentials from Terraform

## After Extraction

Once credentials are extracted, you can run:

```bash
./test-multi-step.sh
```

The test script will automatically load the `.env.testing` file.

## Manual Override

If you want to override any value, you can either:
1. Edit `.env.testing` directly
2. Set environment variables in your shell (they'll override `.env.testing`)

Example:
```bash
export OPENAI_API_KEY="sk-your-direct-openai-key"
./test-multi-step.sh
```
