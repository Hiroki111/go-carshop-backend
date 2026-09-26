# ---- Build stage ----
FROM golang:1.27.1-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/car-service ./cmd/web

# ---- Run stage ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates && \
    addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=builder /out/car-service .
USER app
EXPOSE 8080
ENTRYPOINT ["./order-service"]