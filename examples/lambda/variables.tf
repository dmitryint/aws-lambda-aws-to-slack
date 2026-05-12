variable "function_name" {
  description = "Name of the Lambda function that receives AWS events and posts to Slack."
  type        = string
}

variable "slack_hook_url" {
  description = "Slack incoming webhook URL. Plaintext or KMS ciphertext (auto-detected at cold start)."
  type        = string
  sensitive   = true
}

variable "slack_channel" {
  description = "Optional Slack channel override. Plaintext or KMS ciphertext."
  type        = string
  sensitive   = true
  nullable    = true
  default     = null
}

variable "chart_bucket_name" {
  description = "S3 bucket the CloudWatch parser writes generated chart images to. Null disables the chart parser permissions."
  type        = string
  nullable    = true
  default     = null
}

variable "chart_bucket_region" {
  description = "Region of the chart S3 bucket. May differ from the Lambda's region; the presigned-URL host must carry the bucket's region for SignatureV4 to validate."
  type        = string
  nullable    = true
  default     = null
}

variable "dedup_table_name" {
  description = "DynamoDB table used by the Inspector2 parser to dedupe noisy retries. Null disables the dedup parser permissions."
  type        = string
  nullable    = true
  default     = null
}

variable "dedup_ttl_days" {
  description = "TTL applied to dedup records, in days."
  type        = number
  default     = 14

  validation {
    condition     = var.dedup_ttl_days >= 1
    error_message = "dedup_ttl_days must be >= 1."
  }
}

variable "hide_aws_links" {
  description = "When true, suppresses AWS console deep links in Slack messages (display-only toggle)."
  type        = bool
  default     = false
}

variable "kms_decrypt_key_arn" {
  description = "ARN of the customer-managed KMS key used to encrypt env vars at rest and decrypt SLACK_HOOK_URL / SLACK_CHANNEL at cold start."
  type        = string
  sensitive   = true
  nullable    = true
  default     = null

  validation {
    condition     = var.kms_decrypt_key_arn == null || can(regex("^arn:aws[\\w-]*:kms:[a-z0-9-]+:\\d{12}:key/[a-f0-9-]+$", var.kms_decrypt_key_arn))
    error_message = "kms_decrypt_key_arn must be a valid KMS key ARN of the form arn:aws:kms:<region>:<account-id>:key/<uuid>."
  }
}

variable "memory_size" {
  description = "Lambda memory allocation in MB."
  type        = number
  default     = 256

  validation {
    condition     = var.memory_size >= 128 && var.memory_size <= 10240
    error_message = "memory_size must be between 128 and 10240 MB."
  }
}

variable "timeout_seconds" {
  description = "Lambda execution timeout in seconds."
  type        = number
  default     = 30

  validation {
    condition     = var.timeout_seconds >= 1 && var.timeout_seconds <= 900
    error_message = "timeout_seconds must be between 1 and 900."
  }
}

variable "log_retention_days" {
  description = "Retention period in days for the Lambda's CloudWatch log group."
  type        = number
  default     = 14

  validation {
    condition     = contains([1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653], var.log_retention_days)
    error_message = "log_retention_days must be one of the CloudWatch Logs retention values: 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653."
  }
}

variable "tags" {
  description = "Additional tags applied to all resources created by this example."
  type        = map(string)
  default     = {}
}
