.PHONY: run test build docker cover fmt vet

run:
	go run ./cmd/server

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

cover:
	go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/humanix ./cmd/server

docker:
	docker build -t humanix .
	docker run --rm -p 8080:8080 humanix
