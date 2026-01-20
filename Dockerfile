# Build Stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy modules first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o quaycheck main.go

# Run Stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates just in case (though we talk to local socket/proxy mostly)
RUN apk --no-cache add ca-certificates

# Copy binary and static assets
COPY --from=builder /app/quaycheck .
COPY --from=builder /app/static ./static

# Expose port
EXPOSE 8080

# Environment variable for Docker Host (can be overridden)
ENV DOCKER_HOST="tcp://socket-proxy:2375"

CMD ["./quaycheck"]
