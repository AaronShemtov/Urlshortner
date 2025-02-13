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
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/google/uuid" // You can use any method to generate unique IDs
)

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortURL struct {
	ExecutionID string `json:"execution_id"` // Add ExecutionID to the struct
	Code        string `json:"code"`
	LongURL     string `json:"long_url"`
	CreatedAt   string `json:"created_at"`
}

var db = dynamodb.New(session.Must(session.NewSession()))
var tableName = "LongShortLinks"

// Generate a random short code of specified length
func generateShortCode(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	code := make([]byte, length)
	for i := range code {
		code[i] = charset[rand.Intn(len(charset))]
	}
	log.Println("Generated short code:", string(code))
	return string(code)
}

// Handle shortening of URL
func shortenURL(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Processing shortenURL request...")

	var request ShortenRequest
	if err := json.Unmarshal([]byte(req.Body), &request); err != nil {
		log.Println("Error parsing JSON request body:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Invalid JSON"}, err
	}

	if request.URL == "" {
		log.Println("Missing URL in request")
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Missing URL"}, nil
	}

	// Generate short code and set ExecutionID
	code := generateShortCode(6)
	executionID := uuid.New().String() // Generate unique ExecutionID
	shortURL := ShortURL{
		ExecutionID: executionID, // Set ExecutionID
		Code:        code,
		LongURL:     request.URL,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}

	log.Println("Generated short URL entry:", shortURL)

	// Put the item into DynamoDB with ExecutionID and Code as partition and sort keys
	_, err := db.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]*dynamodb.AttributeValue{
			"ExecutionID": {S: aws.String(shortURL.ExecutionID)}, // Partition key
			"Code":        {S: aws.String(shortURL.Code)},        // Sort key (for the GSI)
			"LongURL":     {S: aws.String(shortURL.LongURL)},     // Long URL
			"CreatedAt":   {S: aws.String(shortURL.CreatedAt)},   // Timestamp
		},
	})
	if err != nil {
		log.Println("Error saving item to DynamoDB:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Database error"}, err
	}

	// Prepare response with short URL
	response := map[string]string{"short_url": fmt.Sprintf("https://u.1ms.my/r/%s", code)}
	respBody, _ := json.Marshal(response)

	log.Println("Successfully created short URL:", response)

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(respBody)}, nil
}

// Handle redirection based on short code
func redirectURL(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	// Access path from RequestContext, e.g., /r/{code}
	log.Println("Processing redirect request for code:", req.RequestContext.HTTP.Path)

	code := req.RequestContext.HTTP.Path[3:] // Path might include `/r/`, so slicing it off

	// Fetch the item from DynamoDB using the short code as the key in GSI
	result, err := db.Query(&dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("CodeIndexName"), // Use GSI here
		KeyConditionExpression: aws.String("Code = :code"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":code": {S: aws.String(code)},
		},
	})
	if err != nil {
		log.Println("Error retrieving item from DynamoDB:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Database error"}, err
	}

	if len(result.Items) == 0 {
		log.Println("Short URL not found for code:", code)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusNotFound, Body: "URL not found"}, nil
	}

	// Unmarshal the result into the ShortURL struct
	var shortURL ShortURL
	err = dynamodbattribute.UnmarshalMap(result.Items[0], &shortURL)
	if err != nil {
		log.Println("Error unmarshaling DynamoDB response:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Internal error"}, err
	}

	log.Println("Redirecting to:", shortURL.LongURL)

	// Redirect to the long URL
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusMovedPermanently,
		Headers:    map[string]string{"Location": shortURL.LongURL},
	}, nil
}

func handler(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Received request:", req)

	switch req.RequestContext.HTTP.Method {
	case "POST":
		return shortenURL(req)
	case "GET":
		return redirectURL(req)
	default:
		log.Println("Unsupported method:", req.RequestContext.HTTP.Method)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusMethodNotAllowed,
			Body:       "Method Not Allowed",
		}, nil
	}
}

func main() {
	log.Println("Lambda function started...")
	lambda.Start(handler)
}
