.PHONY: run migrate up down test lint

GOLANGCI := ./bin/golangci-lint

run:
	go run ./cmd/bank

migrate:
	go run ./cmd/migrate

up:
	docker compose up -d

down:
	docker compose down

test:
	go test ./...

lint:
	$(GOLANGCI) run
