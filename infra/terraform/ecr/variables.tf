variable "aws_region" {
  description = "AWS region in which to create the ECR repositories."
  type        = string
  default     = "eu-west-1"
}

variable "project_name" {
  description = "Name used in repository names and cost-allocation tags."
  type        = string
  default     = "go-saas-learning-lab"
}

variable "environment" {
  description = "Environment name used in repository names and tags."
  type        = string
  default     = "lab"
}

variable "image_retention_count" {
  description = "Number of most recently pushed images retained in each repository."
  type        = number
  default     = 10

  validation {
    condition     = var.image_retention_count >= 1
    error_message = "image_retention_count must be at least 1."
  }
}

variable "untagged_retention_days" {
  description = "Number of days to retain untagged images before expiration."
  type        = number
  default     = 1

  validation {
    condition     = var.untagged_retention_days >= 1
    error_message = "untagged_retention_days must be at least 1."
  }
}

variable "force_delete" {
  description = "Allow Terraform to delete repositories that still contain images. Keep false outside deliberate lab cleanup."
  type        = bool
  default     = false
}

variable "additional_tags" {
  description = "Additional tags applied by the AWS provider to taggable resources."
  type        = map(string)
  default     = {}
}
