.PHONY: all setup build run stop logs lint test

# Install chrome
setup:
	wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
	sudo dpkg -i google-chrome-stable_current_amd64.deb

# Build the main crawler docker image
build:
	docker-compose build crawler

# Start all services (postgres, redis, crawler) in detached mode
run:
	docker-compose up -d --build

# Stop and remove all services and their volumes
stop:
	docker-compose down -v

# Follow the logs of the crawler service
logs:
	docker-compose logs -f crawler

# Run database migrations using a temporary container
migrate-up:
	docker run --rm -v ${PWD}/migrations:/migrations --network host migrate/migrate \
	-path /migrations -database "postgres://user:password@localhost:5432/crawler_dev?sslmode=disable" up

# Lint the project
lint:
	golangci-lint run

# Run tests
test:
	go test ./...