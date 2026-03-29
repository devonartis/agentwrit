# Stage 1: Build broker binary
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o broker ./cmd/broker

# Stage 2: Broker image
FROM alpine:3.21 AS broker

RUN apk --no-cache add ca-certificates sqlite curl
WORKDIR /root/
COPY --from=builder /app/broker .
EXPOSE 8080
ENTRYPOINT ["./broker"]
