# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the Golang binary
RUN go build -o urlshortener

# Final runtime image
FROM alpine:latest

WORKDIR /root/

# Copy compiled binary from builder
COPY --from=builder /app/urlshortener .

# Set environment variables
ENV DYNAMODB_TABLE=url_shortener

# Expose port (if running locally)
EXPOSE 8080

# Run the app
CMD ["./urlshortener"]
