# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies if needed
RUN apk add --no-cache git

# Copy dependency definitions
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary (CGO_ENABLED=0 since we use pure-Go modernc.org/sqlite)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/nudge ./cmd/nudge

# Final stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/nudge /app/nudge

# Expose HTTP port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["/app/nudge"]
