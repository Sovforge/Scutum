# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /workspace

COPY go.mod go.sum* ./
RUN go mod download

COPY api/ api/
COPY internal/ internal/
COPY cmd/ cmd/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /manager ./cmd/main.go

# Final stage
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
