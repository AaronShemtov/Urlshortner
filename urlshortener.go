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

// Handle URL shortening
func shortenURL(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Processing shortenURL request...")

	var request ShortenRequest
	if err := json.Unmarshal([]byte(req.Body), &request); err != nil {
		log.Println("Error parsing JSON request body:", err)
		return createResponse(http.StatusBadRequest, "Invalid JSON"), nil
	}

	if request.URL == "" {
		log.Println("Missing URL in request")
		return createResponse(http.StatusBadRequest, "Missing URL"), nil
	}

	// Generate short code and ExecutionID
	code := generateShortCode(3)
	executionID := uuid.New().String()
	shortURL := ShortURL{
		ExecutionID: executionID,
		Code:        code,
		LongURL:     request.URL,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}

	log.Println("Generated short URL entry:", shortURL)

	// Store in DynamoDB
	_, err := db.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]*dynamodb.AttributeValue{
			"ExecutionID": {S: aws.String(shortURL.ExecutionID)},
			"Code":        {S: aws.String(shortURL.Code)},
			"LongURL":     {S: aws.String(shortURL.LongURL)},
			"CreatedAt":   {S: aws.String(shortURL.CreatedAt)},
		},
	})
	if err != nil {
		log.Println("Error saving item to DynamoDB:", err)
		return createResponse(http.StatusInternalServerError, "Database error"), nil
	}

	shortURLResponse := map[string]string{
		"short_url": fmt.Sprintf("https://1ms.my/%s", code),
	}
	respBody, _ := json.Marshal(shortURLResponse)

	log.Println("Successfully created short URL:", shortURLResponse)
	return createResponse(http.StatusOK, string(respBody)), nil
}

// Handle custom URL creation
func createCustomURL(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Processing createCustomURL request...")

	var request ShortenRequest
	if err := json.Unmarshal([]byte(req.Body), &request); err != nil {
		log.Println("Error parsing JSON request body:", err)
		return createResponse(http.StatusBadRequest, "Invalid JSON"), nil
	}

	if request.URL == "" || request.Code == "" {
		log.Println("Missing URL or custom code in request")
		return createResponse(http.StatusBadRequest, "Missing URL or custom code"), nil
	}

	if len(request.Code) < 8 {
		log.Println("Custom code too short")
		return createResponse(http.StatusBadRequest, "Custom code must be at least 8 characters"), nil
	}

	// Check if custom code is already in use
	result, err := db.Query(&dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("CodeIndexName"),
		KeyConditionExpression: aws.String("Code = :code"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":code": {S: aws.String(request.Code)},
		},
	})
	if err != nil {
		log.Println("Error querying DynamoDB:", err)
		return createResponse(http.StatusInternalServerError, "Database error"), nil
	}

	if len(result.Items) > 0 {
		log.Println("Custom code already exists:", request.Code)
		return createResponse(http.StatusConflict, "Custom code already in use"), nil
	}

	executionID := uuid.New().String()
	shortURL := ShortURL{
		ExecutionID: executionID,
		Code:        request.Code,
		LongURL:     request.URL,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}

	log.Println("Generated custom short URL entry:", shortURL)

	// Store in DynamoDB
	_, err = db.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]*dynamodb.AttributeValue{
			"ExecutionID": {S: aws.String(shortURL.ExecutionID)},
			"Code":        {S: aws.String(shortURL.Code)},
			"LongURL":     {S: aws.String(shortURL.LongURL)},
			"CreatedAt":   {S: aws.String(shortURL.CreatedAt)},
		},
	})
	if err != nil {
		log.Println("Error saving custom link to DynamoDB:", err)
		return createResponse(http.StatusInternalServerError, "Database error"), nil
	}

	customURLResponse := map[string]string{
		"short_url": fmt.Sprintf("https://1ms.my/%s", request.Code),
	}
	respBody, _ := json.Marshal(customURLResponse)

	log.Println("Successfully created custom short URL:", customURLResponse)
	return createResponse(http.StatusOK, string(respBody)), nil
}

// Route request handler
func handler(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	switch req.RequestContext.HTTP.Method {
	case "POST":
		if req.RawPath == "/createcustom" {
			return createCustomURL(req)
		}
		return shortenURL(req)
	case "OPTIONS":
		return createResponse(http.StatusOK, ""), nil
	default:
		return createResponse(http.StatusMethodNotAllowed, "Method Not Allowed"), nil
	}
}

func main() {
	lambda.Start(handler)
}
