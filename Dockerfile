# Build stage
FROM golang:alpine AS builder

# Set Go toolchain to auto download any required go version
ENV GOTOOLCHAIN=auto

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
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata openssl

WORKDIR /app

# Copy binary and prisma schema
COPY --from=builder /app/server /app/server
COPY --from=builder /app/prisma /app/prisma

# Default ports (both 8080 and 10000 for local & Render container scanning)
EXPOSE 8080
EXPOSE 10000

CMD ["/app/server"]
