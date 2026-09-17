terraform {
  required_version = ">= 1.9"
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 5.0" }
  }
  backend "s3" {} # state backend configured per environment, never inline
}

variable "project" { type = string }
variable "db_password" {
  type      = string
  sensitive = true # via env TF_VAR_db_password or vault, never committed
}

# Capability blocks bind here, at the edge: compute/db/auth/queue/CDN.
# Keep modules small; cost-estimate artifact required before apply.
# Gates: terraform validate + plan (read-only) before any human spend gate.
