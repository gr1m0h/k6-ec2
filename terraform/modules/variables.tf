variable "name" {
  description = "Name prefix for all resources created by this module."
  type        = string
  default     = "k6-ec2"
}

variable "vpc_id" {
  description = "VPC ID where the security group will be created."
  type        = string
}

variable "subnet_ids" {
  description = "List of subnet IDs for k6 runner instances."
  type        = list(string)

  validation {
    condition     = length(var.subnet_ids) > 0
    error_message = "At least one subnet ID must be provided."
  }
}

variable "script_bucket_name" {
  description = "Custom S3 bucket name for k6 scripts. If empty, a name is auto-generated from account ID and region."
  type        = string
  default     = ""
}

variable "enable_spot" {
  description = "Whether to enable Spot Instances in the sample config output."
  type        = bool
  default     = true
}

variable "allowed_instance_types" {
  description = "EC2 instance types allowed for k6 runners."
  type        = list(string)
  default     = ["c5.xlarge", "c5.2xlarge", "c5.4xlarge", "m5.xlarge", "m5.2xlarge"]
}

variable "tags" {
  description = "Additional tags to apply to all resources."
  type        = map(string)
  default     = {}
}

variable "log_group_prefix" {
  description = "CloudWatch Logs group name. Must match CLI hardcoded value (/k6-ec2) for log streaming to work."
  type        = string
  default     = "/k6-ec2"
}

variable "log_retention_days" {
  description = "Number of days to retain CloudWatch log events."
  type        = number
  default     = 14
}

variable "s3_expiration_days" {
  description = "Number of days after whitch S3 objects under the k6-ec2/ prefix are automatically deleted."
  type        = number
  default     = 30
}

variable "eip_count" {
  description = "Number of Elastic IPs to pre-allocate for k6 runners. Required for WAF IP-based allowlisting. Set to match spec.runner.parallelism."
  type        = number
  default     = 0

  validation {
    condition     = var.eip_count >= 0
    error_message = "eip_count must be non-negative."
  }
}
