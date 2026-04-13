FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o analytics-service ./cmd/analytics

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/analytics-service .
COPY --from=builder /app/configs/analytics.yaml ./configs/
EXPOSE 8084 9094
CMD ["./analytics-service"]
