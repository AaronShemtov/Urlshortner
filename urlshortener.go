package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

type ShortenRequest struct {
	URL  string `json:"url"`
	Code string `json:"code,omitempty"`
}

type ShortURL struct {
	Code      string `json:"code"`
	LongURL   string `json:"long_url"`
	CreatedAt string `json:"created_at"`
}

var db = dynamodb.New(session.Must(session.NewSession()))
var tableName = os.Getenv("DYNAMODB_TABLE")

func generateShortCode(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	code := make([]byte, length)
	for i := range code {
		code[i] = charset[rand.Intn(len(charset))]
	}
	return string(code)
}

func shortenURL(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var request ShortenRequest
	if err := json.Unmarshal([]byte(req.Body), &request); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest}, err
	}

	if request.URL == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Missing URL"}, nil
	}

	code := generateShortCode(6)
	return storeShortURL(request.URL, code)
}

func createCustomURL(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var request ShortenRequest
	if err := json.Unmarshal([]byte(req.Body), &request); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest}, err
	}

	if request.URL == "" || request.Code == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Missing URL or custom code"}, nil
	}

	if len(request.Code) < 8 {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Custom short code must be at least 8 characters long"}, nil
	}

	// Check if custom code already exists
	result, err := db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       map[string]*dynamodb.AttributeValue{"Code": {S: aws.String(request.Code)}},
	})
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Database error"}, nil
	}
	if result.Item != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusConflict, Body: "Custom short code already taken"}, nil
	}

	return storeShortURL(request.URL, request.Code)
}

func storeShortURL(url, code string) (events.APIGatewayProxyResponse, error) {
	shortURL := ShortURL{
		Code:      code,
		LongURL:   url,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	item, err := dynamodbattribute.MarshalMap(shortURL)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
	}

	_, err = db.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
	}

	response := map[string]string{"short_url": fmt.Sprintf("https://1ms.my/%s", code)}
	respBody, _ := json.Marshal(response)

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(respBody)}, nil
}

func redirectURL(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	code := req.PathParameters["code"]

	result, err := db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       map[string]*dynamodb.AttributeValue{"Code": {S: aws.String(code)}},
	})
	if err != nil || result.Item == nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusNotFound, Body: "URL not found"}, nil
	}

	var shortURL ShortURL
	dynamodbattribute.UnmarshalMap(result.Item, &shortURL)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusMovedPermanently,
		Headers:    map[string]string{"Location": shortURL.LongURL},
	}, nil
}

func handler(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	switch req.HTTPMethod {
	case "POST":
		if req.Path == "/createcustom" {
			return createCustomURL(req)
		}
		return shortenURL(req)
	case "GET":
		return redirectURL(req)
	default:
		return events.APIGatewayProxyResponse{StatusCode: http.StatusMethodNotAllowed}, nil
	}
}

func main() {
	lambda.Start(handler)
}
