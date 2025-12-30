.PHONY: build run clean test

# Build the application
build:
	go build -o bin/cloudflare-speedtest ./cmd

# Run the application
run: build
	./bin/cloudflare-speedtest

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Run tests
test:
	go test -v ./...

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run ./...

# Download dependencies
deps:
	go mod download
	go mod tidy

# Build for multiple platforms
build-all:
	GOOS=windows GOARCH=amd64 go build -o bin/cloudflare-speedtest-windows-amd64.exe ./cmd
	GOOS=darwin GOARCH=amd64 go build -o bin/cloudflare-speedtest-darwin-amd64 ./cmd
	GOOS=linux GOARCH=amd64 go build -o bin/cloudflare-speedtest-linux-amd64 ./cmd
