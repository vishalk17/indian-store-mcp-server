# ---- Build stage ----
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy entire source tree (including internal/)
COPY . ./

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o indian-store-server

# ---- Runtime stage ----
FROM alpine:3.20

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/indian-store-server /app/indian-store-server

# Expose HTTP port
EXPOSE 8080

# Run the server
CMD ["/app/indian-store-server"]
