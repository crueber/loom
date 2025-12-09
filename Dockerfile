# Stage 1: Node.js builder for JavaScript bundling
FROM node:20-alpine AS js-builder

WORKDIR /app

# Copy package files
COPY cmd/server/package.json cmd/server/

# Install build dependencies
WORKDIR /app/cmd/server
RUN npm install

# Copy source files
COPY cmd/server/src/ src/
COPY cmd/server/build.js .

# Build JavaScript bundle
RUN node build.js

# Stage 2: Go builder
FROM golang:1.24-alpine AS builder

# Install ca-certificates, tzdata, and git (for version detection)
RUN apk add --no-cache ca-certificates tzdata git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built JavaScript from Stage 1 (includes version.txt)
COPY --from=js-builder /app/cmd/server/static/dist/ ./cmd/server/static/dist/

# Build binaries (version is read from embedded version.txt at runtime)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o user ./cmd/user

# Stage 3: Runtime
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
