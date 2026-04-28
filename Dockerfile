# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o onvif-server ./cmd/onvif-server

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS support
RUN apk --no-cache add ca-certificates ffmpeg tzdata

# Copy the binary from builder
COPY --from=builder /build/onvif-server /app/onvif-server

# Make the binary executable
RUN chmod +x /app/onvif-server

# Expose ONVIF HTTP ports (8081-8086) and RTSP port (554)
EXPOSE 8081 8082 8083 8084 8085 8086 554

# Run the application
ENTRYPOINT ["/app/onvif-server"]
CMD ["config.yaml"]
