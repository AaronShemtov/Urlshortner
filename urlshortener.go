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
var tableName = "LongShortLinks"

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

	code := generateShortCode(6)
	shortURL := ShortURL{
		Code:      code,
		LongURL:   request.URL,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	log.Println("Generated short URL entry:", shortURL)

	item, err := dynamodbattribute.MarshalMap(shortURL)
	if err != nil {
		log.Println("Error marshaling item for DynamoDB:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Internal error"}, err
	}

	_, err = db.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	if err != nil {
		log.Println("Error saving item to DynamoDB:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Database error"}, err
	}

	response := map[string]string{"short_url": fmt.Sprintf("https://u.1ms.my/r/%s", code)}
	respBody, _ := json.Marshal(response)

	log.Println("Successfully created short URL:", response)

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK, Body: string(respBody)}, nil
}

func redirectURL(req events.LambdaFunctionURLRequest) (events.APIGatewayProxyResponse, error) {
	// Access path from RequestContext, e.g., /r/{code}
	log.Println("Processing redirect request for code:", req.RequestContext.HTTP.Path)

	code := req.RequestContext.HTTP.Path

	result, err := db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       map[string]*dynamodb.AttributeValue{"Code": {S: aws.String(code)}},
	})
	if err != nil {
		log.Println("Error retrieving item from DynamoDB:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Database error"}, err
	}

	if result.Item == nil {
		log.Println("Short URL not found for code:", code)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusNotFound, Body: "URL not found"}, nil
	}

	var shortURL ShortURL
	err = dynamodbattribute.UnmarshalMap(result.Item, &shortURL)
	if err != nil {
		log.Println("Error unmarshaling DynamoDB response:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Internal error"}, err
	}

	log.Println("Redirecting to:", shortURL.LongURL)

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
