package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type CIDRRecord struct {
	Key  string `json:"key" dynamodbav:"key"`
	CIDR string `json:"cidr" dynamodbav:"cidr"`
}

type CIDRService struct {
	dynamoClient *dynamodb.Client
	tableName    string
}

type APIResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

func NewCIDRService(ctx context.Context) (*CIDRService, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS SDK config: %w", err)
	}

	tableName := os.Getenv("DYNAMODB_TABLE_NAME")
	if tableName == "" {
		return nil, fmt.Errorf("DYNAMODB_TABLE_NAME environment variable is required")
	}

	return &CIDRService{
		dynamoClient: dynamodb.NewFromConfig(cfg),
		tableName:    tableName,
	}, nil
}

func (c *CIDRService) GetAllCIDRs(ctx context.Context) ([]CIDRRecord, error) {
	input := &dynamodb.ScanInput{
		TableName: aws.String(c.tableName),
	}

	result, err := c.dynamoClient.Scan(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to scan DynamoDB table: %w", err)
	}

	var records []CIDRRecord
	for _, item := range result.Items {
		var record CIDRRecord
		if err := attributevalue.UnmarshalMap(item, &record); err != nil {
			return nil, fmt.Errorf("failed to unmarshal DynamoDB item: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}

func (c *CIDRService) RegisterCIDR(ctx context.Context, key, cidr string) error {
	if err := c.validateCIDR(cidr); err != nil {
		return fmt.Errorf("invalid CIDR: %w", err)
	}

	record := CIDRRecord{
		Key:  key,
		CIDR: cidr,
	}

	item, err := attributevalue.MarshalMap(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(c.tableName),
		Item:      item,
	}

	_, err = c.dynamoClient.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to put item in DynamoDB: %w", err)
	}

	return nil
}

func (c *CIDRService) DeleteCIDR(ctx context.Context, key string) error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"key": &types.AttributeValueMemberS{Value: key},
		},
	}

	_, err := c.dynamoClient.DeleteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete item from DynamoDB: %w", err)
	}

	return nil
}

func (c *CIDRService) GetNextAvailableCIDR(ctx context.Context) (string, error) {
	records, err := c.GetAllCIDRs(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get existing CIDRs: %w", err)
	}

	usedCIDRs := make(map[string]bool)
	for _, record := range records {
		if strings.HasPrefix(record.CIDR, "10.") {
			usedCIDRs[record.CIDR] = true
		}
	}

	// Generate all possible 10.x.0.0/16 CIDRs (10.0.0.0/16 to 10.255.0.0/16)
	for i := 0; i <= 255; i++ {
		cidr := fmt.Sprintf("10.%d.0.0/16", i)
		if !usedCIDRs[cidr] {
			return cidr, nil
		}
	}

	return "", fmt.Errorf("no available 10.x.0.0/16 CIDRs remaining")
}

func (c *CIDRService) validateCIDR(cidr string) error {
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR format: %w", err)
	}
	return nil
}

func createResponse(statusCode int, body interface{}) (events.APIGatewayProxyResponse, error) {
	var bodyStr string
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return events.APIGatewayProxyResponse{}, fmt.Errorf("failed to marshal response body: %w", err)
		}
		bodyStr = string(bodyBytes)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "GET, POST, DELETE, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type, Authorization",
		},
		Body: bodyStr,
	}, nil
}

func handleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	cidrService, err := NewCIDRService(ctx)
	if err != nil {
		return createResponse(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to initialize CIDR service: %v", err),
		})
	}

	switch request.HTTPMethod {
	case "GET":
		if request.Path == "/next" || (request.QueryStringParameters != nil && request.QueryStringParameters["action"] == "next") {
			nextCIDR, err := cidrService.GetNextAvailableCIDR(ctx)
			if err != nil {
				return createResponse(http.StatusInternalServerError, map[string]string{
					"error": fmt.Sprintf("failed to get next available CIDR: %v", err),
				})
			}
			return createResponse(http.StatusOK, map[string]string{
				"cidr": nextCIDR,
			})
		}

		// Get all CIDRs
		records, err := cidrService.GetAllCIDRs(ctx)
		if err != nil {
			return createResponse(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to get CIDRs: %v", err),
			})
		}

		// Sort records by key for consistent output
		sort.Slice(records, func(i, j int) bool {
			return records[i].Key < records[j].Key
		})

		return createResponse(http.StatusOK, map[string]interface{}{
			"records": records,
			"count":   len(records),
		})

	case "POST":
		var requestBody struct {
			Key  string `json:"key"`
			CIDR string `json:"cidr"`
		}

		if err := json.Unmarshal([]byte(request.Body), &requestBody); err != nil {
			return createResponse(http.StatusBadRequest, map[string]string{
				"error": "invalid JSON body",
			})
		}

		if requestBody.Key == "" || requestBody.CIDR == "" {
			return createResponse(http.StatusBadRequest, map[string]string{
				"error": "both key and cidr fields are required",
			})
		}

		if err := cidrService.RegisterCIDR(ctx, requestBody.Key, requestBody.CIDR); err != nil {
			return createResponse(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("failed to register CIDR: %v", err),
			})
		}

		return createResponse(http.StatusCreated, map[string]string{
			"message": "CIDR registered successfully",
			"key":     requestBody.Key,
			"cidr":    requestBody.CIDR,
		})

	case "DELETE":
		key := request.QueryStringParameters["key"]
		if key == "" {
			return createResponse(http.StatusBadRequest, map[string]string{
				"error": "key parameter is required",
			})
		}

		if err := cidrService.DeleteCIDR(ctx, key); err != nil {
			return createResponse(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to delete CIDR: %v", err),
			})
		}

		return createResponse(http.StatusOK, map[string]string{
			"message": "CIDR deleted successfully",
			"key":     key,
		})

	case "OPTIONS":
		return createResponse(http.StatusOK, nil)

	default:
		return createResponse(http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
	}
}

func main() {
	lambda.Start(handleRequest)
}
