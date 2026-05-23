FROM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run generate

FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/.output/public/ ./cmd/api/dist/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o scutum ./cmd/api

FROM golang:1.25-alpine AS sbom
RUN apk add --no-cache curl
RUN curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin
WORKDIR /app
COPY --from=builder /app/go.mod /app/go.sum ./
RUN syft dir:/app -o cyclonedx-json=/app/sbom.json

FROM alpine:3.21
RUN apk add --no-cache curl wireguard-tools iproute2
WORKDIR /app
COPY --from=builder /app/scutum /app/scutum
COPY --from=sbom /app/sbom.json /app/sbom.json
LABEL org.opencontainers.image.title="Scutum" \
      org.opencontainers.image.description="Sovereign P2P infrastructure orchestration" \
      com.scutum.sbom="/app/sbom.json"
EXPOSE 8080
CMD ["./scutum"]
