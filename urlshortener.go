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
	"github.com/google/uuid"
)

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortURL struct {
	ExecutionID string `json:"execution_id"`
	Code        string `json:"code"`
	LongURL     string `json:"long_url"`
	CreatedAt   string `json:"created_at"`
}

var db = dynamodb.New(session.Must(session.NewSession()))
var tableName = "LongShortLinks"

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
	code := generateShortCode(4)
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

// Handle redirection
func redirectURL(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Processing redirect request...")
	log.Println("Received RawPath:", req.RawPath) // Debugging

	// Extract short code from URL path using RawPath
	parts := strings.Split(req.RawPath, "/")
	if len(parts) <= 1 {
		log.Println("No valid short code found in path")
		return createResponse(http.StatusBadRequest, "Invalid short code"), nil
	}

	code := parts[len(parts)-1]
	log.Println("Extracted code (case-sensitive):", code)

	if code == "" {
		log.Println("Short code is empty")
		return createResponse(http.StatusBadRequest, "Invalid short code"), nil
	}

	// Query DynamoDB using the exact code
	log.Println("Querying DynamoDB for code:", code)
	result, err := db.Query(&dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("CodeIndexName"),
		KeyConditionExpression: aws.String("Code = :code"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":code": {S: aws.String(code)},
		},
	})
	if err != nil {
		log.Println("Error retrieving item from DynamoDB:", err)
		return createResponse(http.StatusInternalServerError, "Database error"), nil
	}

	log.Println("DynamoDB query result count:", len(result.Items))
	if len(result.Items) == 0 {
		log.Println("Short URL not found for code:", code)
		return createResponse(http.StatusNotFound, "URL not found"), nil
	}

	// Extract LongURL
	longURLAttr, exists := result.Items[0]["LongURL"]
	if !exists || longURLAttr.S == nil {
		log.Println("LongURL attribute missing in DynamoDB entry")
		return createResponse(http.StatusInternalServerError, "Invalid data in database"), nil
	}
	longURL := *longURLAttr.S
	log.Println("Retrieved LongURL:", longURL)

	if longURL == "" {
		log.Println("Retrieved LongURL is empty")
		return createResponse(http.StatusNotFound, "URL not found"), nil
	}

	// Redirect user
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusMovedPermanently,
		Headers: map[string]string{
			"Location": longURL,
			"Access-Control-Allow-Origin": "*",
			"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type",
		},
	}, nil
}

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

// Route request d
func handler(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("======== NEW REQUEST RECEIVED ========")
	log.Println("Received RawPath:", req.RawPath)
	log.Println("HTTP Method:", req.RequestContext.HTTP.Method)

	switch req.RequestContext.HTTP.Method {
	case "POST":
		return shortenURL(req)
	case "GET":
		return redirectURL(req)
	case "OPTIONS":
		return createResponse(http.StatusOK, ""), nil
	default:
		log.Println("Unsupported method:", req.RequestContext.HTTP.Method)
		return createResponse(http.StatusMethodNotAllowed, "Method Not Allowed"), nil
	}
}


func main() {
	log.Println("Lambda function started...")
	lambda.Start(handler)
}
