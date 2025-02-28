package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
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
func createResponse(statusCode int, body string) (events.APIGatewayProxyResponse, error) {
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":                "application/json",
			"Access-Control-Allow-Origin": "*",
			"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type",
		},
		Body: body,
	}, nil
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

// Handle standard URL shortening
func shortenURL(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Processing shortenURL request...")

	var request ShortenRequest
	if err := json.Unmarshal([]byte(req.Body), &request); err != nil {
		log.Println("Error parsing JSON request body:", err)
		return createResponse(http.StatusBadRequest, "Invalid JSON")
	}

	if request.URL == "" {
		log.Println("Missing URL in request")
		return createResponse(http.StatusBadRequest, "Missing URL")
	}

	code := generateShortCode(3)
	shortURL := ShortURL{
		ExecutionID: code,
		Code:        code,
		LongURL:     request.URL,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}

	log.Println("Saving short URL to DynamoDB:", shortURL)
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
		return createResponse(http.StatusInternalServerError, "Database error")
	}

	responseBody, _ := json.Marshal(map[string]string{
		"short_url": fmt.Sprintf("https://1ms.my/%s", code),
	})
	log.Println("Successfully created short URL:", string(responseBody))

	return createResponse(http.StatusOK, string(responseBody))
}

// Handle custom URL creation
func createCustomURL(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Processing createCustomURL request...")

	var request ShortenRequest
	if err := json.Unmarshal([]byte(req.Body), &request); err != nil {
		log.Println("Error parsing JSON request body:", err)
		return createResponse(http.StatusBadRequest, "Invalid JSON")
	}

	if request.URL == "" || request.Code == "" {
		log.Println("Missing URL or custom code in request")
		return createResponse(http.StatusBadRequest, "Missing URL or custom code")
	}

	if len(request.Code) < 8 {
		log.Println("Custom code too short")
		return createResponse(http.StatusBadRequest, "Custom code must be at least 8 characters")
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
		return createResponse(http.StatusInternalServerError, "Database error")
	}

	if len(result.Items) > 0 {
		log.Println("Custom code already exists:", request.Code)
		return createResponse(http.StatusConflict, "Custom code already in use")
	}

	shortURL := ShortURL{
		ExecutionID: request.Code,
		Code:        request.Code,
		LongURL:     request.URL,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}

	log.Println("Saving custom short URL to DynamoDB:", shortURL)
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
		return createResponse(http.StatusInternalServerError, "Database error")
	}

	responseBody, _ := json.Marshal(map[string]string{
		"short_url": fmt.Sprintf("https://1ms.my/%s", request.Code),
	})
	log.Println("Successfully created custom short URL:", string(responseBody))

	return createResponse(http.StatusOK, string(responseBody))
}

// Handle redirection for GET requests
func redirectURL(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Processing redirect for GET request...")

	// Extract short code from path (e.g. /abc -> abc)
	parts := strings.Split(req.RawPath, "/")
	if len(parts) < 2 {
		log.Println("Invalid short code in path")
		return createResponse(http.StatusBadRequest, "Invalid short code")
	}
	code := parts[len(parts)-1]

	// Query DynamoDB by code
	result, err := db.Query(&dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("CodeIndexName"), // Must match a GSI with `Code` as partition key
		KeyConditionExpression: aws.String("Code = :code"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":code": {S: aws.String(code)},
		},
	})
	if err != nil {
		log.Println("Error querying DynamoDB for redirection:", err)
		return createResponse(http.StatusInternalServerError, "Database error")
	}
	if len(result.Items) == 0 {
		log.Println("Short code not found:", code)
		return createResponse(http.StatusNotFound, "URL not found")
	}

	longURL := *result.Items[0]["LongURL"].S
	log.Println("Redirecting to:", longURL)

	// Return a 301 redirect
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusMovedPermanently,
		Headers: map[string]string{
			"Location":                longURL,
			"Access-Control-Allow-Origin": "*",
			"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type",
		},
	}, nil
}

// Route request handler
func handler(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Handling request. Method:", req.RequestContext.HTTP.Method, "Path:", req.RawPath)

	switch req.RequestContext.HTTP.Method {
	case "GET":
		// This case is added so GET requests can do redirection
		return redirectURL(req)
	case "POST":
		if req.RawPath == "/createcustom" {
			return createCustomURL(req)
		}
		return shortenURL(req)
	case "OPTIONS":
		return createResponse(http.StatusOK, "")
	default:
		log.Println("Unsupported HTTP method received:", req.RequestContext.HTTP.Method)
		return createResponse(http.StatusMethodNotAllowed, "Method Not Allowed")
	}
}

func main() {
	log.Println("Lambda function started...")
	lambda.Start(handler)
}
