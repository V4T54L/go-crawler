# Stage 1: Build the Go binary in a dedicated build environment
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk --no-cache add git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the application, disabling CGO for a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o crawler ./cmd/crawler

# Stage 2: Create the final, minimal image
FROM alpine:latest

# Add Certificate Authorities for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/crawler .
# Copy the .env file if it exists, for configuration
COPY .env .

# Expose the application port
EXPOSE 8080

# The command to run when the container starts
CMD ["./crawler"]