import * as aws from "@pulumi/aws";
import * as pulumi from "@pulumi/pulumi";

// Get configuration values
const config = new pulumi.Config();
const functionName = config.get("function-name") || "cidr-finder";
const tableName = config.get("table-name") || "cidr-registry";
const lambdaZipPath = config.get("lambda-zip-path") || "../function.zip";

// Default tags for all resources
const defaultTags = {
    Project: "CIDRFinder",
    Environment: "production",
    ManagedBy: "Pulumi"
};

// DynamoDB table for CIDR registry
const cidrRegistry = new aws.dynamodb.Table("cidr-registry", {
    name: tableName,
    billingMode: "PAY_PER_REQUEST",
    hashKey: "key",
    attributes: [{
        name: "key",
        type: "S"
    }],
    tags: {
        ...defaultTags,
        Name: tableName
    }
});

// IAM role for Lambda
const lambdaRole = new aws.iam.Role("cidr-lambda-role", {
    name: `${functionName}-role`,
    assumeRolePolicy: JSON.stringify({
        Version: "2012-10-17",
        Statement: [{
            Action: "sts:AssumeRole",
            Effect: "Allow",
            Principal: {
                Service: "lambda.amazonaws.com"
            }
        }]
    }),
    tags: defaultTags
});

// IAM policy for DynamoDB access
const dynamodbPolicy = new aws.iam.Policy("dynamodb-policy", {
    name: `${functionName}-dynamodb-policy`,
    policy: pulumi.all([cidrRegistry.arn]).apply(([tableArn]) =>
        JSON.stringify({
            Version: "2012-10-17",
            Statement: [{
                Effect: "Allow",
                Action: [
                    "dynamodb:PutItem",
                    "dynamodb:GetItem",
                    "dynamodb:UpdateItem",
                    "dynamodb:DeleteItem",
                    "dynamodb:Scan",
                    "dynamodb:Query"
                ],
                Resource: tableArn
            }]
        })
    )
});

// Attach DynamoDB policy to Lambda role
const lambdaDynamodbPolicyAttachment = new aws.iam.RolePolicyAttachment("lambda-dynamodb-policy", {
    role: lambdaRole.name,
    policyArn: dynamodbPolicy.arn
});

// Attach basic execution role for Lambda
const lambdaBasicExecutionAttachment = new aws.iam.RolePolicyAttachment("lambda-basic-execution", {
    role: lambdaRole.name,
    policyArn: "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
});

// Lambda function
const cidrFinderLambda = new aws.lambda.Function("cidr-finder", {
    code: new pulumi.asset.FileArchive(lambdaZipPath),
    name: functionName,
    role: lambdaRole.arn,
    handler: "bootstrap",
    runtime: "provided.al2023",
    timeout: 30,
    memorySize: 128,
    environment: {
        variables: {
            DYNAMODB_TABLE_NAME: cidrRegistry.name
        }
    },
    tags: {
        ...defaultTags,
        Name: functionName
    }
}, {
    dependsOn: [lambdaBasicExecutionAttachment, lambdaDynamodbPolicyAttachment]
});

// API Gateway for Lambda
const cidrApi = new aws.apigatewayv2.Api("cidr-api", {
    name: `${functionName}-api`,
    protocolType: "HTTP",
    corsConfiguration: {
        allowCredentials: false,
        allowHeaders: ["content-type", "authorization"],
        allowMethods: ["GET", "POST", "DELETE", "OPTIONS"],
        allowOrigins: ["*"],
        maxAge: 86400
    },
    tags: {
        ...defaultTags,
        Name: `${functionName}-api`
    }
});

// API Gateway integration
const cidrIntegration = new aws.apigatewayv2.Integration("cidr-integration", {
    apiId: cidrApi.id,
    integrationType: "AWS_PROXY",
    integrationMethod: "POST",
    integrationUri: cidrFinderLambda.invokeArn
});

// API Gateway routes
const getCidrsRoute = new aws.apigatewayv2.Route("get-cidrs", {
    apiId: cidrApi.id,
    routeKey: "GET /",
    target: pulumi.interpolate`integrations/${cidrIntegration.id}`
});

const getNextCidrRoute = new aws.apigatewayv2.Route("get-next-cidr", {
    apiId: cidrApi.id,
    routeKey: "GET /next",
    target: pulumi.interpolate`integrations/${cidrIntegration.id}`
});

const postCidrRoute = new aws.apigatewayv2.Route("post-cidr", {
    apiId: cidrApi.id,
    routeKey: "POST /",
    target: pulumi.interpolate`integrations/${cidrIntegration.id}`
});

const deleteCidrRoute = new aws.apigatewayv2.Route("delete-cidr", {
    apiId: cidrApi.id,
    routeKey: "DELETE /",
    target: pulumi.interpolate`integrations/${cidrIntegration.id}`
});

// API Gateway stage
const defaultStage = new aws.apigatewayv2.Stage("default", {
    apiId: cidrApi.id,
    name: "$default",
    autoDeploy: true,
    tags: {
        ...defaultTags,
        Name: `${functionName}-api-stage`
    }
});

// Lambda permission for API Gateway
const apiGatewayInvokePermission = new aws.lambda.Permission("api-gateway-invoke", {
    statementId: "AllowExecutionFromAPIGateway",
    action: "lambda:InvokeFunction",
    function: cidrFinderLambda.name,
    principal: "apigateway.amazonaws.com",
    sourceArn: pulumi.interpolate`${cidrApi.executionArn}/*/*`
});

// Exports
export const apiGatewayUrl = cidrApi.apiEndpoint;
export const lambdaFunctionName = cidrFinderLambda.name;
export const lambdaFunctionArn = cidrFinderLambda.arn;
export const dynamodbTableName = cidrRegistry.name;
export const dynamodbTableArn = cidrRegistry.arn;
