.PHONY: build run docker-up docker-down migrate-up migrate-down

build:
	go build -o bin/gator .

run: build
	./bin/gator

docker-up:
	docker compose -f docker/docker-compose.yml up -d

docker-down:
	docker compose -f docker/docker-compose.yml down

# Requires: go install github.com/pressly/goose/v3/cmd/goose@latest
migrate-up:
	cd sql/schema && goose postgres "$$GATOR_DB_URL" up

migrate-down:
	cd sql/schema && goose postgres "$$GATOR_DB_URL" down
