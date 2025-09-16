# CIDR Finder Service

A Go-based AWS Lambda service that manages network CIDR allocations using DynamoDB for storage. The service provides REST API endpoints to register, delete, and retrieve the next available 10.0.0.0/16 CIDR blocks.

## Features

- **Register CIDR**: Associate a key with a specific CIDR block
- **Delete CIDR**: Remove a CIDR registration by key
- **Get all CIDRs**: Retrieve all registered CIDR blocks
- **Get next available**: Find the next unregistered 10.x.0.0/16 CIDR block

## API Endpoints

### GET /
Retrieve all registered CIDR blocks.

**Response:**
```json
{
  "records": [
    {"key": "vpc-prod", "cidr": "10.0.0.0/16"},
    {"key": "vpc-staging", "cidr": "10.1.0.0/16"}
  ],
  "count": 2
}
```

### GET /next
Get the next available 10.x.0.0/16 CIDR block.

**Response:**
```json
{
  "cidr": "10.2.0.0/16"
}
```

### POST /
Register a new CIDR block with a key.

**Request:**
```json
{
  "key": "vpc-dev",
  "cidr": "10.2.0.0/16"
}
```

**Response:**
```json
{
  "message": "CIDR registered successfully",
  "key": "vpc-dev",
  "cidr": "10.2.0.0/16"
}
```

### DELETE /?key=<key>
Delete a CIDR registration by key.

**Response:**
```json
{
  "message": "CIDR deleted successfully",
  "key": "vpc-dev"
}
```

## Development

### Prerequisites
- Go 1.22 or later
- AWS CLI configured
- Terraform (optional, for infrastructure deployment)

### Building

```bash
# Install dependencies
make install-deps

# Build the Lambda binary
make build

# Run tests
make test

# Create deployment package
make package
```

### Local Testing

```bash
# Build and test
go build -o cidrfinder .
go test -v ./...
```

## Deployment

### Using Terraform (Recommended)

1. Navigate to the terraform directory:
```bash
cd terraform
```

2. Initialize Terraform:
```bash
terraform init
```

3. Build the Lambda package:
```bash
cd ..
make package
cd terraform
```

4. Deploy the infrastructure:
```bash
terraform plan
terraform apply
```

5. Get the API Gateway URL:
```bash
terraform output api_gateway_url
```

### Manual Deployment

1. Create the DynamoDB table:
```bash
make create-table
```

2. Create the Lambda function through AWS Console or CLI with:
   - Runtime: `provided.al2023`
   - Handler: `bootstrap`
   - Environment variable: `DYNAMODB_TABLE_NAME=cidr-registry`
   - Appropriate IAM role with DynamoDB permissions

3. Upload the deployment package:
```bash
make deploy
```

## Usage Examples

```bash
# Get all registered CIDRs
curl https://your-api-gateway-url/

# Get next available CIDR
curl https://your-api-gateway-url/next

# Register a new CIDR
curl -X POST https://your-api-gateway-url/ \
  -H "Content-Type: application/json" \
  -d '{"key": "vpc-prod", "cidr": "10.0.0.0/16"}'

# Delete a CIDR registration
curl -X DELETE https://your-api-gateway-url/?key=vpc-prod
```

## Configuration

The service uses the following environment variables:

- `DYNAMODB_TABLE_NAME`: Name of the DynamoDB table (required)

## Architecture

- **Lambda Function**: Handles HTTP requests and business logic
- **DynamoDB**: Stores CIDR registrations with key-value pairs
- **API Gateway**: Provides HTTP REST interface
- **IAM Roles**: Manages permissions for Lambda to access DynamoDB

## CIDR Allocation Logic

The service manages 10.x.0.0/16 CIDR blocks where x ranges from 0-255, providing up to 256 unique /16 networks within the 10.0.0.0/8 private address space.