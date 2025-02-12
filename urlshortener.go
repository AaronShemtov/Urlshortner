func handler(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    fmt.Println("===== Received Request =====")
    fmt.Printf("Full Request: %+v\n", req)

    if req.HTTPMethod == "" {
        fmt.Println("Error: HTTP Method is empty")
        return events.APIGatewayProxyResponse{
            StatusCode: http.StatusBadRequest,
            Body:       "HTTP Method is missing",
        }, nil
    }

    switch req.HTTPMethod {
    case "POST":
        return shortenURL(req)
    case "GET":
        return redirectURL(req)
    default:
        fmt.Println("Error: Unsupported HTTP Method ->", req.HTTPMethod)
        return events.APIGatewayProxyResponse{
            StatusCode: http.StatusMethodNotAllowed,
            Body:       "Method Not Allowed",
        }, nil
    }
}
