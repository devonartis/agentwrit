# Stage 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o broker ./cmd/broker

# Stage 2: Final
FROM alpine:3.18

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/broker .

# Expose port 8080
EXPOSE 8080

# Run the binary
ENTRYPOINT ["./broker"]
