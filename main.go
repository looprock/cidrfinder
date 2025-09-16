package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

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
