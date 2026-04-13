FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o billing-service ./cmd/billing

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/billing-service .
COPY --from=builder /app/configs/billing.yaml ./configs/
EXPOSE 8082 9092
CMD ["./billing-service"]
