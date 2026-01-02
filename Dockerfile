# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git (needed for go mod download)
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /qrlocal ./cmd/qrlocal

# Runtime stage
FROM alpine:3.19

# Install openssh-client for tunnel functionality
RUN apk add --no-cache openssh-client ca-certificates

# Create non-root user
RUN adduser -D -g '' qrlocal
USER qrlocal

# Copy binary from builder
COPY --from=builder /qrlocal /usr/local/bin/qrlocal

# Default working directory for serving files
WORKDIR /data

# Expose common ports
EXPOSE 8080

# Default command
ENTRYPOINT ["qrlocal"]
CMD ["--help"]
