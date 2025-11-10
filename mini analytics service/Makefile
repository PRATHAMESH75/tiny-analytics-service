COMPOSE_FILE=deployments/docker-compose.yml

.PHONY: tidy infra-up infra-down run-ingest run-consumer run-loader run-query test test-e2e

tidy:
	go mod tidy

infra-up:
	docker compose -f $(COMPOSE_FILE) up -d --wait

infra-down:
	docker compose -f $(COMPOSE_FILE) down -v

run-ingest:
	go run ./cmd/ingest-api

run-consumer:
	go run ./cmd/consumer-enricher

run-loader:
	go run ./cmd/loader

run-query:
	go run ./cmd/query-api

test:
	go test ./...

test-e2e:
	go test -tags=e2e ./test/integration -v
