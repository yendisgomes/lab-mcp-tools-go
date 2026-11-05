.PHONY: build test lint run-http docker
VERSION ?= dev

build:
	go build -trimpath -ldflags "-X main.version=$(VERSION)" -o bin/mcp-frete .

test:
	go test -race -cover ./...

lint:
	go vet ./...
	golangci-lint run ./...

run-http: build
	./bin/mcp-frete -http :8080

docker:
	docker compose -f deploy/docker-compose.yml up --build
