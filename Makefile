COMPOSE ?= docker-compose

.PHONY: run worker test fmt vet check docker-build docker-up docker-down

run:
	go run ./cmd/api

worker:
	go run ./cmd/worker

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

check: test vet

docker-build:
	$(COMPOSE) build

docker-up:
	$(COMPOSE) up --build

docker-down:
	$(COMPOSE) down
