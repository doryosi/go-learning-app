variable "aws_region" {
  description = "AWS region in which to build the network."
  type        = string
  default     = "eu-west-1"
}

variable "project_name" {
  description = "Name used in resource names and cost-allocation tags."
  type        = string
  default     = "go-saas-learning-lab"
}

variable "environment" {
  description = "Environment name used in resource names and tags."
  type        = string
  default     = "lab"
}

variable "vpc_cidr" {
  description = "IPv4 CIDR block assigned to the VPC."
  type        = string
  default     = "10.0.0.0/16"

  validation {
    condition     = can(cidrnetmask(var.vpc_cidr))
    error_message = "vpc_cidr must be a valid IPv4 CIDR block."
  }
}

variable "availability_zone_count" {
  description = "Number of Availability Zones to use. The supplied subnet CIDR lists must contain at least this many entries."
  type        = number
  default     = 2

  validation {
    condition     = var.availability_zone_count >= 2 && var.availability_zone_count <= 3
    error_message = "availability_zone_count must be 2 or 3."
  }
}

variable "public_subnet_cidrs" {
  description = "CIDR blocks for public ALB and NAT subnets, one per Availability Zone."
  type        = list(string)
  default     = ["10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24"]

  validation {
    condition     = alltrue([for cidr in var.public_subnet_cidrs : can(cidrnetmask(cidr))])
    error_message = "Every public subnet value must be a valid IPv4 CIDR block."
  }
}

variable "private_subnet_cidrs" {
  description = "CIDR blocks for private compute and managed services, one per Availability Zone."
  type        = list(string)
  default     = ["10.0.10.0/24", "10.0.11.0/24", "10.0.12.0/24"]

  validation {
    condition     = alltrue([for cidr in var.private_subnet_cidrs : can(cidrnetmask(cidr))])
    error_message = "Every private subnet value must be a valid IPv4 CIDR block."
  }
}

variable "nat_gateway_mode" {
  description = "NAT strategy: none has no NAT cost, single is cheaper but not AZ-resilient, and per_az is resilient but creates one charged gateway per AZ."
  type        = string
  default     = "none"

  validation {
    condition     = contains(["none", "single", "per_az"], var.nat_gateway_mode)
    error_message = "nat_gateway_mode must be none, single, or per_az."
  }
}

variable "app_port" {
  description = "Port on which the ALB reaches the Go API."
  type        = number
  default     = 8080

  validation {
    condition     = var.app_port >= 1 && var.app_port <= 65535
    error_message = "app_port must be between 1 and 65535."
  }
}

variable "additional_tags" {
  description = "Additional tags applied by the AWS provider to taggable resources."
  type        = map(string)
  default     = {}
}
