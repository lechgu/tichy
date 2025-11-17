.PHONY: build clean lint

BINARY_NAME=tichy
LDFLAGS=-ldflags="-s -w"

build:
	CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o $(BINARY_NAME) .

clean:
	@go clean ./...
	@rm -f $(BINARY_NAME)

lint:
	@golangci-lint run

