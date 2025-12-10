# Base supervisor Dockerfile
# This builds the supervisor binary that can be used in game-specific images

FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the supervisor binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o supervisor ./cmd/supervisor

# Final stage - minimal image
FROM alpine:3.20

RUN apk --no-cache add ca-certificates

COPY --from=builder /build/supervisor /usr/local/bin/supervisor

ENTRYPOINT ["/usr/local/bin/supervisor"]
