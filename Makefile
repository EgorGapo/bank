.PHONY: run migrate up down test lint

GOLANGCI := ./bin/golangci-lint

run:
	go run ./cmd/bank
runN:
	go run ./cmd/notifier

migrate:
	go run ./cmd/migrate
	
migrate-down: 
	go run ./cmd/migrate down
	
up:
	docker compose up -d --build   
api:
	docker compose up -d --build api

stop:
	docker compose stop 
logs:
	docker compose logs -f --no-log-prefix api | jq


down:
	docker compose down

test:
	go test ./...

lint:
	$(GOLANGCI) run






