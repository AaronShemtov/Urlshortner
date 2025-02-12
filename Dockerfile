# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy Go module files first for efficient caching
COPY go.mod ./

# Download dependencies
RUN go mod tidy

# Copy the rest of the source code
COPY . .

# Build the Go binary
RUN go build -o urlshortener

# Ensure the binary has execution permissions
RUN chmod +x urlshortener

# Final runtime image
FROM alpine:latest

WORKDIR /root/

# Install necessary CA certificates (if needed for HTTPS requests)
RUN apk --no-cache add ca-certificates

# Copy compiled binary from builder
COPY --from=builder /app/urlshortener .

# Ensure the binary has execution permissions (just in case)
RUN chmod +x /root/urlshortener

# Set as entrypoint
ENTRYPOINT ["/root/urlshortener"]