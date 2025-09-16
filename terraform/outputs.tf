output "api_gateway_url" {
  description = "URL of the API Gateway endpoint"
  value       = aws_apigatewayv2_api.cidr_api.api_endpoint
}

output "lambda_function_name" {
  description = "Name of the Lambda function"
  value       = aws_lambda_function.cidr_finder.function_name
}

output "lambda_function_arn" {
  description = "ARN of the Lambda function"
  value       = aws_lambda_function.cidr_finder.arn
}

output "dynamodb_table_name" {
  description = "Name of the DynamoDB table"
  value       = aws_dynamodb_table.cidr_registry.name
}

output "dynamodb_table_arn" {
  description = "ARN of the DynamoDB table"
  value       = aws_dynamodb_table.cidr_registry.arn
}
