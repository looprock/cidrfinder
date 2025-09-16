terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = var.default_tags
  }
}

# DynamoDB table for CIDR registry
resource "aws_dynamodb_table" "cidr_registry" {
  name         = var.table_name
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "key"

  attribute {
    name = "key"
    type = "S"
  }

  tags = merge(var.default_tags, {
    Name = var.table_name
  })
}

# IAM role for Lambda
resource "aws_iam_role" "cidr_lambda_role" {
  name = "${var.function_name}-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })
}

# IAM policy for DynamoDB access
resource "aws_iam_policy" "dynamodb_policy" {
  name = "${var.function_name}-dynamodb-policy"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "dynamodb:PutItem",
          "dynamodb:GetItem",
          "dynamodb:UpdateItem",
          "dynamodb:DeleteItem",
          "dynamodb:Scan",
          "dynamodb:Query"
        ]
        Resource = aws_dynamodb_table.cidr_registry.arn
      }
    ]
  })
}

# Attach DynamoDB policy to Lambda role
resource "aws_iam_role_policy_attachment" "lambda_dynamodb_policy" {
  role       = aws_iam_role.cidr_lambda_role.name
  policy_arn = aws_iam_policy.dynamodb_policy.arn
}

# Attach basic execution role for Lambda
resource "aws_iam_role_policy_attachment" "lambda_basic_execution" {
  role       = aws_iam_role.cidr_lambda_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# Lambda function
resource "aws_lambda_function" "cidr_finder" {
  filename         = var.lambda_zip_path
  function_name    = var.function_name
  role            = aws_iam_role.cidr_lambda_role.arn
  handler         = "bootstrap"
  runtime         = "provided.al2023"
  timeout         = 30
  memory_size     = 128

  environment {
    variables = {
      DYNAMODB_TABLE_NAME = aws_dynamodb_table.cidr_registry.name
    }
  }

  depends_on = [
    aws_iam_role_policy_attachment.lambda_basic_execution,
    aws_iam_role_policy_attachment.lambda_dynamodb_policy
  ]
}

# API Gateway for Lambda
resource "aws_apigatewayv2_api" "cidr_api" {
  name          = "${var.function_name}-api"
  protocol_type = "HTTP"

  cors_configuration {
    allow_credentials = false
    allow_headers     = ["content-type", "authorization"]
    allow_methods     = ["GET", "POST", "DELETE", "OPTIONS"]
    allow_origins     = ["*"]
    max_age          = 86400
  }
}

# API Gateway integration
resource "aws_apigatewayv2_integration" "cidr_integration" {
  api_id             = aws_apigatewayv2_api.cidr_api.id
  integration_type   = "AWS_PROXY"
  integration_method = "POST"
  integration_uri    = aws_lambda_function.cidr_finder.invoke_arn
}

# API Gateway routes
resource "aws_apigatewayv2_route" "get_cidrs" {
  api_id    = aws_apigatewayv2_api.cidr_api.id
  route_key = "GET /"
  target    = "integrations/${aws_apigatewayv2_integration.cidr_integration.id}"
}

resource "aws_apigatewayv2_route" "get_next_cidr" {
  api_id    = aws_apigatewayv2_api.cidr_api.id
  route_key = "GET /next"
  target    = "integrations/${aws_apigatewayv2_integration.cidr_integration.id}"
}

resource "aws_apigatewayv2_route" "post_cidr" {
  api_id    = aws_apigatewayv2_api.cidr_api.id
  route_key = "POST /"
  target    = "integrations/${aws_apigatewayv2_integration.cidr_integration.id}"
}

resource "aws_apigatewayv2_route" "delete_cidr" {
  api_id    = aws_apigatewayv2_api.cidr_api.id
  route_key = "DELETE /"
  target    = "integrations/${aws_apigatewayv2_integration.cidr_integration.id}"
}

# API Gateway stage
resource "aws_apigatewayv2_stage" "default" {
  api_id      = aws_apigatewayv2_api.cidr_api.id
  name        = "$default"
  auto_deploy = true
}

# Lambda permission for API Gateway
resource "aws_lambda_permission" "api_gateway_invoke" {
  statement_id  = "AllowExecutionFromAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.cidr_finder.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.cidr_api.execution_arn}/*/*"
}
