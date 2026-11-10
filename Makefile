-include .env
export

MIGRATIONS_DIR := migrations
IMAGE ?= trip-service:local
VERSION ?= dev
LDFLAGS := -X github.com/maksimkasimovhse/TripGo/internal/infra/telemetry.Version=$(VERSION)
DOCKER_NETWORK ?= kind
DOCKER_DATABASE_URL ?= postgres://tripgo:tripgo@tripgo-local-control-plane:30203/tripgo?sslmode=disable
DOCKER_OTLP_ENDPOINT ?= http://tripgo-local-control-plane:30206

.PHONY: generate migrate migrate-down migrate-status run test test-integration docker-build docker-run generate-mocks

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
	go run -ldflags="$(LDFLAGS)" ./cmd/service

test:
	go test -race ./...

test-integration:
	go test -race -tags=integration -count=1 ./...

docker-build:
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE) .

docker-run:
	docker run --rm -p 8080:8080 --network $(DOCKER_NETWORK) \
	  -e DATABASE_URL='$(DOCKER_DATABASE_URL)' \
	  -e LOG_LEVEL='$(LOG_LEVEL)' \
	  -e OTEL_EXPORTER_OTLP_ENDPOINT='$(DOCKER_OTLP_ENDPOINT)' \
	  -e OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf \
	  -e OTEL_METRIC_EXPORT_INTERVAL='$(or $(OTEL_METRIC_EXPORT_INTERVAL),10000)' \
	  -e OTEL_METRIC_EXPORT_TIMEOUT='$(or $(OTEL_METRIC_EXPORT_TIMEOUT),2000)' \
	  -e OTEL_TRACES_SAMPLER='$(or $(OTEL_TRACES_SAMPLER),parentbased_always_on)' \
	  -e OTEL_TRACES_SAMPLER_ARG='$(OTEL_TRACES_SAMPLER_ARG)' \
	  $(IMAGE)

generate-mocks:
	go generate ./internal/...
