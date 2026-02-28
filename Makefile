.PHONY: run build test lint migrate migrate-down docker-up docker-down swag

BINARY=server
MAIN=./cmd/server

## Run locally (requires .env)
run:
	go run $(MAIN)

## Build binary
build:
	CGO_ENABLED=0 go build -o $(BINARY) $(MAIN)

## Run tests with race detector
test:
	go test -race -v ./...

## Lint (requires golangci-lint)
lint:
	golangci-lint run ./...

## Apply migrations
migrate:
	go run $(MAIN) migrate || true
	@echo "Use make run to start the server (migrations run automatically on startup)"

## Roll back one migration
migrate-down:
	@export $$(cat .env | grep -v '^#' | xargs) && migrate -path ./migrations -database "$$DATABASE_URL" down 1

## Start all services via Docker Compose
docker-up:
	docker compose up --build -d

## Stop and remove containers
docker-down:
	docker compose down -v

## Generate Swagger docs (requires swag CLI: go install github.com/swaggo/swag/cmd/swag@latest)
swag:
	swag init -g cmd/server/main.go -o docs

## Tidy modules
tidy:
	go mod tidy
