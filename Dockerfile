FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /broker ./cmd/broker

FROM alpine:3.19
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /broker /broker
EXPOSE 8080
ENTRYPOINT ["/broker"]
