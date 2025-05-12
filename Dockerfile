FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application for the platform of the build machine automatically
ARG TARGETPLATFORM
RUN if [ "$TARGETPLATFORM" = "linux/amd64" ]; then \
      CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o backmeup ./cmd/backmeup; \
    elif [ "$TARGETPLATFORM" = "linux/arm64" ]; then \
      CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -o backmeup ./cmd/backmeup; \
    else \
      CGO_ENABLED=0 go build -a -installsuffix cgo -o backmeup ./cmd/backmeup; \
    fi

# Create final image
FROM alpine:3.21
ARG TARGETPLATFORM

# Install runtime dependencies for database backup utilities
RUN apk add --no-cache \
    postgresql-client \
    mysql-client \
    bash \
    tzdata \
    ca-certificates \
    curl

# Install MinIO client with multi-architecture support
ARG TARGETPLATFORM
RUN if [ "$TARGETPLATFORM" = "linux/amd64" ] || [ -z "$TARGETPLATFORM" ]; then \
      curl -O https://dl.min.io/client/mc/release/linux-amd64/mc; \
    elif [ "$TARGETPLATFORM" = "linux/arm64" ]; then \
      curl -O https://dl.min.io/client/mc/release/linux-arm64/mc; \
    else \
      echo "Unsupported platform: $TARGETPLATFORM, falling back to amd64" && \
      curl -O https://dl.min.io/client/mc/release/linux-amd64/mc; \
    fi && \
    chmod +x mc && \
    mv mc /usr/local/bin/

# Set working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/backmeup /app/backmeup

# Copy example configuration
COPY example/config.example.yml /app/config.example.yml

# Create volume for backups
VOLUME ["/backups"]

# Set environment variables
ENV CONFIG_PATH=/app/config.yml

# Set entrypoint
ENTRYPOINT ["/app/backmeup"]
CMD ["-config", "/app/config.yml"]