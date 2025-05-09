FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/

# Build the DLQ replay utility
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/dlq-replay \
    -ldflags "-X main.Version=2.1.2 -X main.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%SZ') -X main.CommitSHA=$(git rev-parse HEAD)" \
    ./cmd/dlq-replay/main.go

# Create final lightweight image
FROM alpine:3.18

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /bin/dlq-replay /app/dlq-replay

# Default entrypoint
ENTRYPOINT ["/app/dlq-replay"]
