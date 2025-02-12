package main

import (
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
	URL string `json:"url"`
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
	shortURL := ShortURL{
		Code:      code,
		LongURL:   request.URL,
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

	response := map[string]string{"short_url": fmt.Sprintf("https://u.1ms.my/r/%s", code)}
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
