.PHONY: build test lint clean install run

# Binary name
BINARY := cliverse

# Build the application
build:
	go build -o $(BINARY) .

# Run tests
test:
	go test -v ./...

# Run tests with coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -f $(BINARY)
	rm -f coverage.out coverage.html

# Install the binary
install:
	go install .

# Run the application (with arguments)
run:
	go run . $(ARGS)

# Run TUI mode
tui:
	go run . disk tui .

# Format code
fmt:
	go fmt ./...

# Check for vulnerabilities
vuln:
	govulncheck ./...
