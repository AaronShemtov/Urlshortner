package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/google/uuid"
)

type ShortenRequest struct {
	URL  string `json:"url"`
	Code string `json:"code,omitempty"`
}

type ShortURL struct {
	ExecutionID string `json:"execution_id"`
	Code        string `json:"code"`
	LongURL     string `json:"long_url"`
	CreatedAt   string `json:"created_at"`
}

var db = dynamodb.New(session.Must(session.NewSession()))
var tableName = "LongShortLinks"

// Universal response function with CORS headers
func createResponse(statusCode int, body string) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":                "application/json",
			"Access-Control-Allow-Origin": "*",
			"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type",
		},
		Body: body,
	}
}

// Generate a random short code
func generateShortCode(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_~"
	rand.Seed(time.Now().UnixNano())
	code := make([]byte, length)
	for i := range code {
		code[i] = charset[rand.Intn(len(charset))]
	}
	log.Println("Generated short code:", string(code))
	return string(code)
}

// Handle URL redirection
func redirectURL(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Processing redirect request for path:", req.RawPath)

	parts := strings.Split(req.RawPath, "/")
	if len(parts) < 2 {
		return createResponse(http.StatusBadRequest, "Invalid short code"), nil
	}

	code := parts[len(parts)-1]
	log.Println("Extracted short code:", code)

	// Query DynamoDB
	result, err := db.Query(&dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("CodeIndexName"),
		KeyConditionExpression: aws.String("Code = :code"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":code": {S: aws.String(code)},
		},
	})
	if err != nil {
		log.Println("Error querying DynamoDB:", err)
		return createResponse(http.StatusInternalServerError, "Database error"), nil
	}

	if len(result.Items) == 0 {
		log.Println("Short code not found:", code)
		return createResponse(http.StatusNotFound, "URL not found"), nil
	}

	longURL := *result.Items[0]["LongURL"].S
	log.Println("Redirecting to:", longURL)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusMovedPermanently,
		Headers: map[string]string{
			"Location": longURL,
		},
	}, nil
}

// Route request handler
func handler(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	switch req.RequestContext.HTTP.Method {
	case "POST":
		if req.RawPath == "/createcustom" {
			return createCustomURL(req)
		}
		return shortenURL(req)
	case "GET":
		return redirectURL(req)
	case "OPTIONS":
		return createResponse(http.StatusOK, ""), nil
	default:
		return createResponse(http.StatusMethodNotAllowed, "Method Not Allowed"), nil
	}
}

func main() {
	lambda.Start(handler)
}
