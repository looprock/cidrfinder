package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

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

	sort.Slice(records, func(i, j int) bool {
		return records[i].Key < records[j].Key
	})

	return records, nil
}

func (c *CIDRService) RegisterCIDR(ctx context.Context, key, cidr string) error {
	if err := c.validateCIDR(cidr); err != nil {
		return fmt.Errorf("invalid CIDR: %w", err)
	}

	if err := c.validateUniqueness(ctx, key, cidr); err != nil {
		return err
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

func (c *CIDRService) validateUniqueness(ctx context.Context, key, cidr string) error {
	records, err := c.GetAllCIDRs(ctx)
	if err != nil {
		return fmt.Errorf("failed to check existing records: %w", err)
	}

	for _, record := range records {
		if record.Key == key {
			return fmt.Errorf("key '%s' already exists", key)
		}
		if record.CIDR == cidr {
			return fmt.Errorf("CIDR '%s' already exists", cidr)
		}
	}

	return nil
}
