BINARY_NAME=cmdban
BUILD_DIR=bin
GO=go

.PHONY: all build run test clean tidy lint install uninstall

all: build

build:
	$(GO) build -o $(BINARY_NAME) .

run: build
	./$(BINARY_NAME)

test:
	$(GO) test ./... -v

test-coverage:
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html

tidy:
	$(GO) mod tidy

lint:
	golangci-lint run

clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

install: build
	cp $(BINARY_NAME) /usr/local/bin/

uninstall:
	rm -f /usr/local/bin/$(BINARY_NAME)
