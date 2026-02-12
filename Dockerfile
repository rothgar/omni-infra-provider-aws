# Multi-stage build for omni-infra-provider-aws

# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Allow Go to download required toolchain version
ENV GOTOOLCHAIN=auto

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o omni-infra-provider-aws \
    ./cmd/omni-infra-provider-aws

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 omni && \
    adduser -D -u 1000 -G omni omni

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/omni-infra-provider-aws .

# Set ownership
RUN chown -R omni:omni /app

USER omni

# Expose port if needed (adjust as necessary)
EXPOSE 8080

# Run the binary
ENTRYPOINT ["/app/omni-infra-provider-aws"]
