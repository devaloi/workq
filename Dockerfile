# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o bin/workq ./cmd/workq/

# Runtime stage
FROM alpine:3.21

COPY --from=builder /app/bin/workq /usr/local/bin/workq

ENTRYPOINT ["workq"]
CMD ["--help"]
