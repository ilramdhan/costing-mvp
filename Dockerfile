# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build API binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/api ./cmd/api

# Build Worker binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/worker ./cmd/worker

# Build Seeder binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/seeder ./cmd/seeder

# Build Migrate binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/migrate ./cmd/migrate

# API runtime stage
FROM alpine:3.20 AS api

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/api /app/api
COPY --from=builder /bin/seeder /app/seeder
COPY --from=builder /bin/migrate /app/migrate
COPY migrations /app/migrations

EXPOSE 8080

CMD ["/app/api"]

# Worker runtime stage
FROM alpine:3.20 AS worker

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/worker /app/worker

CMD ["/app/worker"]
