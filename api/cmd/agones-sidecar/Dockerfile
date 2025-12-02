# Build stage
FROM golang:1.25.0-alpine AS builder

WORKDIR /build

# Copy the entire api directory
COPY . .

# Build the sidecar binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o agones-sidecar ./cmd/agones-sidecar

# Runtime stage
FROM alpine:3.20

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/agones-sidecar .

# Agones SDK sidecar environment setup
# The Agones SDK sidecar will provide the gRPC endpoint at localhost:59357
ENV AGONES_SDK_GRPC_HOST=localhost
ENV AGONES_SDK_GRPC_PORT=59357

ENTRYPOINT ["./agones-sidecar"]
