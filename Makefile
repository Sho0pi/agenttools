.PHONY: all fmt vet lint test test-race build ci tidy

all: ci

fmt:
	gofmt -w .

vet:
	go vet ./...

lint:
	golangci-lint run

test:
	go test ./...

test-race:
	go test -race ./...

build:
	go build ./...

tidy:
	go mod tidy

# Everything CI runs.
ci: vet test-race lint build
