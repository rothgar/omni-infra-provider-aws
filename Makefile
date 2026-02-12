.PHONY: help build docker-build docker-push deploy-ec2 deploy-ec2-from-env clean run-local omni-cluster-list

# Variables
BINARY_NAME := omni-infra-provider-aws
VERSION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
IMAGE_TAG := rothgar/omni-infra-provider-aws:$(VERSION)

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

build: ## Build the Go binary locally
	@echo "Building $(BINARY_NAME)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="-w -s" \
		-o $(BINARY_NAME) \
		./cmd/$(BINARY_NAME)
	@echo "Binary built: $(BINARY_NAME)"

docker-build: ## Build the Docker image
	@echo "Building Docker image: $(IMAGE_TAG)"
	docker build --network=host -t $(IMAGE_TAG) .
	@echo "Image built successfully"

docker-push: docker-build ## Push the Docker image to Docker Hub
	@echo "Pushing image to Docker Hub..."
	docker push $(IMAGE_TAG)
	@echo "Image pushed: $(IMAGE_TAG)"

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	rm -f ec2-deployment.json
	@echo "Clean complete"
