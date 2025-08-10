# Build stage
FROM golang:1.21 AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the applications with CGO enabled for Kafka
RUN CGO_ENABLED=1 GOOS=linux go build -o mtg-ingestor cmd/main.go
RUN CGO_ENABLED=1 GOOS=linux go build -o deck-ingester cmd/deck-ingester/main.go

# Final stage
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/mtg-ingestor .
COPY --from=builder /app/deck-ingester .

# Copy config files
COPY --from=builder /app/configs ./configs

# Create non-root user
RUN groupadd -g 1000 mtg && \
    useradd -u 1000 -g mtg -m mtg

# Change ownership
RUN chown -R mtg:mtg /app

USER mtg

# Run the application
ENTRYPOINT ["./mtg-ingestor"]