# Backend configuration for remote state storage
# This keeps Terraform state in Google Cloud Storage instead of local files
#
# Benefits:
# - Prevents secrets from being committed to git
# - Enables team collaboration with state locking
# - Provides state versioning and backup

terraform {
  backend "gcs" {
    bucket = "tolemy-terraform-state"
    prefix = "orby/slack-client"
  }
}


