# Build stage
FROM golang:1.24-alpine AS builder

# Install ca-certificates and git
RUN apk add --no-cache ca-certificates git

WORKDIR /app

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source files
COPY . .

# Generate Prisma Go client
RUN go run github.com/steebchen/prisma-client-go generate

# Build application binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server .

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary and prisma schema
COPY --from=builder /app/server /app/server
COPY --from=builder /app/prisma /app/prisma

# Default port
EXPOSE 8080

CMD ["/app/server"]
