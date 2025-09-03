###################
# BUILD STAGE
###################
FROM golang:1.25-alpine3.22 AS builder

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go module files first for better layer caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimizations
# CGO_ENABLED=0 creates a statically linked binary
# -ldflags="-s -w" strips debug information to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/http-file-server .

###################
# RUNTIME STAGE
###################
FROM alpine:3.22 AS runtime

# # Add CA certificates for HTTPS
# RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/http-file-server /app/

# Create a non-root user to run the application
RUN addgroup -g 1000 -S appgroup && adduser -u 1000 -S appuser -G appgroup

# Create a directory to serve files from and set permissions
RUN mkdir /DirToServe && chown appuser:appgroup /DirToServe

# Use the non-root user
USER appuser

# Expose the port the server runs on
EXPOSE 8080

# Command to run the binary
ENTRYPOINT ["/app/http-file-server", "--listen-ip", "0.0.0.0", "--listen-port", "8080", "--dir-to-serve", "/DirToServe"]

