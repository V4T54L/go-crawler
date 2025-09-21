.PHONY: build run tidy

build:
	@echo "Building binary..."
	@go build -o ./bin/crawler-service ./cmd/api

run:
	@echo "Running application..."
	@go run ./cmd/api

tidy:
	@echo "Running go mod tidy..."
	@go mod tidy

