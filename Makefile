.PHONY: run test fmt vet check

run:
	go run ./cmd/api

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

check: test vet
