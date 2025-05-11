.PHONY: test dev docker-build docker-build-multi docker-push docker-push-github itest itest-up ittest-down

# Default registry URL - can be overridden via REGISTRY_URL env var
REGISTRY_URL ?= docker.io
IMAGE_NAME ?= backmeup
TAG ?= latest
# GitHub username/org for GitHub Packages
GITHUB_USERNAME ?= thitiph0n

# Run all tests
test:
	go test -v ./...

# Run the development server
dev:
	go run cmd/backmeup/main.go

# Build Docker image for current architecture
docker-build:
	docker build -t $(IMAGE_NAME):$(TAG) .

# Build multi-architecture Docker images (amd64 and arm64)
docker-build-multi:
	docker buildx create --name multi-builder --use || true
	docker buildx build --platform linux/amd64,linux/arm64 -t $(REGISTRY_URL)/$(IMAGE_NAME):$(TAG) --push .

# Push Docker image to registry
docker-push:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(REGISTRY_URL)/$(IMAGE_NAME):$(TAG) --push .

# Push Docker image to GitHub Packages
docker-push-github:
	docker buildx build --platform linux/amd64,linux/arm64 -t ghcr.io/$(GITHUB_USERNAME)/$(IMAGE_NAME):$(TAG) --push .

# Start integration test environment
ittest-up:
	cd ittest && docker-compose up -d

# Shutdown integration test environment
ittest-down:
	cd ittest && docker-compose down