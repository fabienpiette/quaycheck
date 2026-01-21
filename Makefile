# Load environment variables if .env file exists
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

# Variables
BIN_DIR ?= bin
BINARY_NAME ?= quaycheck
INSTALL_PATH ?= /usr/local/bin
DOCKER_IMAGE ?= $(BINARY_NAME):latest
IMAGE_TAG ?= latest

# Registry Configuration (now loaded from .env)
# If REGISTRY_DOMAIN is docker.io or empty, we handle the tag differently
ifeq ($(REGISTRY_DOMAIN),docker.io)
    REGISTRY_IMAGE := $(REGISTRY_NAMESPACE)/$(BINARY_NAME):$(IMAGE_TAG)
else ifeq ($(REGISTRY_DOMAIN),)
    REGISTRY_IMAGE := $(REGISTRY_NAMESPACE)/$(BINARY_NAME):$(IMAGE_TAG)
else
    REGISTRY_IMAGE := $(REGISTRY_DOMAIN)/$(REGISTRY_NAMESPACE)/$(BINARY_NAME):$(IMAGE_TAG)
endif

REGISTRY_REPO ?= $(REGISTRY_NAMESPACE)/$(BINARY_NAME)
PUSH_TAGS ?= latest

.PHONY: build test clean run install lint fmt install-binary docker-build docker-tag docker-push docker-verify docker-pull docker-push-tags docker-release up down

# Build the binary
build:
	go build -o $(BINARY_NAME) main.go

# Run tests
test:
	go test -v -cover ./...

# Run tests with coverage report
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	rm -rf $(BIN_DIR)

# Run the application locally (requires DOCKER_HOST if not using local socket)
run:
	go run main.go

# Install dependencies
install:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Build for multiple platforms
build-all:
	mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 main.go
	GOOS=darwin GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 main.go
	GOOS=darwin GOARCH=arm64 go build -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 main.go
	GOOS=windows GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe main.go

# Docker Compose helpers
up:
	docker compose up -d --build

down:
	docker compose down

# Build the Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE) .

# Tag the Docker image for the private registry
docker-tag:
	docker tag $(DOCKER_IMAGE) $(REGISTRY_IMAGE)

# Push the Docker image to the private registry
docker-push:
	docker push $(REGISTRY_IMAGE)

# Tag and push multiple versions of the image
docker-push-tags:
	@if [ -z "$(PUSH_TAGS)" ]; then \
		echo "PUSH_TAGS is required. Invoke as PUSH_TAGS=\"v1.0.0 v1.1.0\" make docker-push-tags"; \
		exit 1; \
	fi; \
	for tag in $(PUSH_TAGS); do \
		target_image=$(REGISTRY_DOMAIN)/$(REGISTRY_NAMESPACE)/$(BINARY_NAME):$$tag; \
		echo "Tagging $(DOCKER_IMAGE) as $$target_image"; \
		docker tag $(DOCKER_IMAGE) $$target_image; \
		echo "Pushing $$target_image"; \
		docker push $$target_image; \
	done

# Verify the image is stored in the private registry
docker-verify:
	@if [ -z "$(REGISTRY_PASSWORD)" ] && [ -n "$(REGISTRY_USER)" ]; then \
		echo "REGISTRY_PASSWORD is not set but REGISTRY_USER is. Skipping verify."; \
	elif [ -n "$(REGISTRY_USER)" ]; then \
		curl -u $(REGISTRY_USER):$(REGISTRY_PASSWORD) https://$(REGISTRY_DOMAIN)/v2/_catalog; \
		curl -u $(REGISTRY_USER):$(REGISTRY_PASSWORD) https://$(REGISTRY_DOMAIN)/v2/$(REGISTRY_REPO)/tags/list; \
	else \
		echo "Skipping verify for public/anonymous registry."; \
	fi

# Pull the Docker image from the private registry
docker-pull:
	docker pull $(REGISTRY_IMAGE)

# Build, tag, push, and verify the Docker image
docker-release: docker-build docker-tag docker-push docker-verify
