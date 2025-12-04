# Stage 1: Build
FROM golang:1.24-alpine AS builder

# Install ca-certificates and tzdata
RUN apk add --no-cache ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build both binaries with static linking
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o user ./cmd/user

# Stage 2: Runtime
FROM scratch

# Copy CA certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binaries
COPY --from=builder /app/server /server
COPY --from=builder /app/user /user

# Expose port
EXPOSE 8080

# Create volume for data
VOLUME ["/data"]

# Set environment variables
ENV DATABASE_PATH=/data/bookmarks.db \
    PORT=8080

# Run server
ENTRYPOINT ["/server"]
