# Build Stage 1: Compile React 19 Frontend
FROM node:22-alpine AS web-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Build Stage 2: Compile Go Binary with Embedded Assets
FROM golang:1.24-alpine AS go-builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
# Copy compiled frontend into web/dist
COPY --from=web-builder /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /previewforge ./cmd/previewforge

# Stage 3: Minimal Production Container
FROM alpine:3.21
RUN apk add --no-cache docker-cli docker-cli-compose git ca-certificates tzdata

WORKDIR /data
VOLUME /data

COPY --from=go-builder /previewforge /usr/local/bin/previewforge

EXPOSE 9850

ENTRYPOINT ["/usr/local/bin/previewforge"]
CMD ["serve", "--domain", "localhost", "--port", "9850", "--data-dir", "/data"]
