FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY internal/ internal/
COPY pkg/ pkg/

# Build the FB-RX component
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/fb-rx \
    -ldflags "-X main.Version=2.1.2 -X main.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%SZ') -X main.CommitSHA=$(git rev-parse HEAD)" \
    ./cmd/fb/rx/main.go

# Create final lightweight image
FROM alpine:3.18

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /bin/fb-rx /app/fb-rx

# Expose metrics port
EXPOSE 2112

# Expose OTLP gRPC and HTTP ports
EXPOSE 4317 4318

# Expose Prometheus remote-write port
EXPOSE 9009

# Set gRPC port for communication with other FBs
EXPOSE 5000

# Health check
HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O- http://localhost:2112/health || exit 1

# Run the service
ENTRYPOINT ["/app/fb-rx"]
