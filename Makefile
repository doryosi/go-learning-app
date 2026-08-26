COMPOSE ?= docker-compose
TF_DIR ?= infra/terraform/network

.PHONY: run worker test fmt vet check docker-build docker-up docker-down terraform-fmt terraform-init terraform-validate terraform-plan

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

terraform-fmt:
	terraform -chdir=$(TF_DIR) fmt -recursive

terraform-init:
	terraform -chdir=$(TF_DIR) init

terraform-validate:
	terraform -chdir=$(TF_DIR) validate

terraform-plan:
	terraform -chdir=$(TF_DIR) plan
