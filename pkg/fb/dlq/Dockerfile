FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the function block
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/fb-dlq ./cmd/fb/dlq

# Create minimal runtime image
FROM alpine:3.18

RUN apk --no-cache add ca-certificates

WORKDIR /

# Copy binary from builder stage
COPY --from=builder /bin/fb-dlq /bin/fb-dlq

# Set executable
ENTRYPOINT ["/bin/fb-dlq"]
