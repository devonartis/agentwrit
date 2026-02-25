# Stage 1: Build both binaries
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o broker ./cmd/broker
RUN CGO_ENABLED=0 GOOS=linux go build -o sidecar ./cmd/sidecar

# Stage 2: Broker image
FROM alpine:3.18 AS broker

RUN apk --no-cache add ca-certificates sqlite curl
WORKDIR /root/
COPY --from=builder /app/broker .
EXPOSE 8080
ENTRYPOINT ["./broker"]

# Stage 3: Sidecar image
FROM alpine:3.18 AS sidecar

RUN apk --no-cache add ca-certificates curl
WORKDIR /root/
COPY --from=builder /app/sidecar .
EXPOSE 8081
ENTRYPOINT ["./sidecar"]
