variable "aws_region" {
  description = "AWS region for resources"
  type        = string
  default     = "us-east-1"
}

variable "function_name" {
  description = "Name of the Lambda function"
  type        = string
  default     = "cidr-finder"
}

variable "table_name" {
  description = "Name of the DynamoDB table"
  type        = string
  default     = "cidr-registry"
}

variable "lambda_zip_path" {
  description = "Path to the Lambda deployment package"
  type        = string
  default     = "../function.zip"
}

variable "default_tags" {
  description = "Default tags to apply to all resources"
  type        = map(string)
  default = {
    Project     = "CIDRFinder"
    Environment = "production"
    ManagedBy   = "Terraform"
  }
}
