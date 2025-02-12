# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy only go.mod (avoids missing go.sum error)
COPY go.mod ./

# Download dependencies (this will regenerate go.sum if needed)
RUN go mod tidy

# Copy the rest of the source code
COPY . .

# Build the Go binary
RUN go build -o urlshortener

# Final runtime image
FROM alpine:latest

WORKDIR /root/

# Copy compiled binary from builder
COPY --from=builder /app/urlshortener .

# Run the app
CMD ["./urlshortener"]