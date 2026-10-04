-include .env
export

MIGRATIONS_DIR := migrations
IMAGE ?= trip-service:local
DOCKER_NETWORK ?= kind
DOCKER_DATABASE_URL ?= postgres://tripgo:tripgo@tripgo-local-control-plane:30103/tripgo?sslmode=disable

.PHONY: generate migrate migrate-down migrate-status run test docker-build docker-run

generate:
	go tool oapi-codegen \
		-generate types,chi-server \
		-package api \
		-include-operation-ids createTrip,getTrip,finishTrip,health,ready \
		-o internal/generated/api.gen.go \
		contracts/openapi/trip-service.openapi.yaml

migrate:
	go tool goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" up

migrate-down:
	go tool goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" down

migrate-status:
	go tool goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" status

run:
	go run ./cmd/trip-service

test:
	go test -race ./...

docker-build:
	docker build -t $(IMAGE) .

docker-run:
	docker run --rm -p 8080:8080 --network $(DOCKER_NETWORK) \
	  -e DATABASE_URL='$(DOCKER_DATABASE_URL)' \
	  $(IMAGE)