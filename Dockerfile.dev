# Development Dockerfile with hot reload
FROM golang:1.23-alpine

# Install development tools
RUN apk add --no-cache git make

# Install Air for hot reload (automatically recompiles on file changes)
# Pinning to v1.52.3 which works with Go 1.23 (latest has broken version constraint)
RUN go install github.com/air-verse/air@v1.52.3

# Install golang-migrate for database migrations
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Set working directory
WORKDIR /controller

# Copy go.mod first
COPY go.mod ./

# Initialize go.sum if it doesn't exist
RUN go mod download || touch go.sum

# Copy go.sum if it exists
COPY go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Expose application port
EXPOSE 8080

# Use Air for hot reload in development
CMD ["air", "-c", ".air.toml"]
