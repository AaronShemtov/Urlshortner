package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"time"
	"fmt"

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
	log.Println("Successfully created custom short URL:", responseBody)

	return createResponse(http.StatusOK, string(responseBody))
}

// Route request handler
func handler(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Handling request. Method:", req.RequestContext.HTTP.Method, "Path:", req.RawPath)

	switch req.RequestContext.HTTP.Method {
	case "POST":
		if req.RawPath == "/createcustom" {
			return createCustomURL(req)
		}
		return createResponse(http.StatusNotImplemented, "Shorten URL function not implemented")
	case "GET":
		return createResponse(http.StatusNotImplemented, "Redirect function not implemented")
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
