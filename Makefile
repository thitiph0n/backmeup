.PHONY: test dev docker-build docker-build-multi docker-push itest itest-up itest-down

# Run all tests
test:
	go test -v ./...

# Run the development server
dev:
	go run cmd/backmeup/main.go

# Build Docker image for current architecture
docker-build:
	docker build -t backmeup:latest .

# Build multi-architecture Docker images (amd64 and arm64)
docker-build-multi:
	docker buildx create --name multi-builder --use || true
	docker buildx build --platform linux/amd64,linux/arm64 -t backmeup:latest --push .

# Push Docker image to registry (adjust registry URL as needed)
docker-push:
	docker buildx build --platform linux/amd64,linux/arm64 -t your-registry/backmeup:latest --push .

# Start integration test environment
ittest-up:
	cd ittest && docker-compose up -d

# Shutdown integration test environment
ittest-down:
	cd ittest && docker-compose down