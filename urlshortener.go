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
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
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

func generateShortCode(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	code := make([]byte, length)
	for i := range code {
		code[i] = charset[rand.Intn(len(charset))]
	}
	return string(code)
}

func shortenURL(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	var request ShortenRequest
	if err := json.Unmarshal([]byte(req.Body), &request); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Invalid JSON"}, err
	}

	if request.URL == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Missing URL"}, nil
	}

	code := generateShortCode(6)
	executionID := uuid.New().String()
	shortURL := ShortURL{
		ExecutionID: executionID,
		Code:        code,
		LongURL:     request.URL,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}

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
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Database error"}, err
	}

	response := map[string]string{"short_url": fmt.Sprintf("https://q4qoiz3fsjtv4rkvvizhg7yaci0zwpro.lambda-url.eu-central-1.on.aws/%s", code)}
	respBody, _ := json.Marshal(response)

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(respBody)}, nil
}

func redirectURL(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	parts := strings.Split(req.RequestContext.HTTP.Path, "/")

	if len(parts) <= 1 {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       "Invalid short code in the path",
		}, nil
	}

	code := parts[len(parts)-1]

	if code == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Invalid short code"}, nil
	}

	result, err := db.Query(&dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("CodeIndexName"),
		KeyConditionExpression: aws.String("Code = :code"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":code": {S: aws.String(code)},
		},
	})
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Database error"}, err
	}

	if len(result.Items) == 0 {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusNotFound, Body: "URL not found"}, nil
	}

	var shortURL ShortURL
	err = dynamodbattribute.UnmarshalMap(result.Items[0], &shortURL)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Internal error"}, err
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusMovedPermanently,
		Headers:    map[string]string{"Location": shortURL.LongURL},
	}, nil
}

func handler(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	switch req.RequestContext.HTTP.Method {
	case "POST":
		return shortenURL(req)
	case "GET":
		return redirectURL(req)
	default:
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusMethodNotAllowed,
			Body:       "Method Not Allowed",
		}, nil
	}
}

func main() {
	lambda.Start(handler)
}
