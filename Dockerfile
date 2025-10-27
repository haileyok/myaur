# Build stage
FROM golang:1.25.3-bookworm AS builder

RUN apt-get update && apt-get install -y \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -o myaur-bin ./cmd/myaur

# Runtime stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    ca-certificates \
    git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /build/myaur-bin /app/myaur-bin

RUN mkdir -p /app/data

EXPOSE 8080 8081

# Set default command
CMD ["/app/myaur-bin", "serve", "--listen-addr", ":8080", "--metrics-listen-addr", ":8081", "--database-path", "/app/data/myaur.db", "--repo-path", "/app/aur-mirror"]
