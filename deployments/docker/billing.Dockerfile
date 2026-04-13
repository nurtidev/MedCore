FROM golang:1.25-bookworm AS builder
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    pkg-config \
    librdkafka-dev \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o billing-service ./cmd/billing

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    librdkafka1 \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/billing-service .
COPY --from=builder /app/configs/billing.yaml ./configs/
COPY --from=builder /app/migrations ./migrations
EXPOSE 8082 9092
CMD ["./billing-service"]
