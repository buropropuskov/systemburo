FROM golang:1.26-bookworm AS dev

WORKDIR /app

RUN go install github.com/swaggo/swag/cmd/swag@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy && swag init -g cmd/server/main.go -o docs && go build -o server ./cmd/server

CMD ["sh", "-c", "swag init -g cmd/server/main.go -o docs && go run ./cmd/server"]

# --- Production ---
FROM golang:1.26-bookworm AS builder

WORKDIR /app

RUN go install github.com/swaggo/swag/cmd/swag@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy && \
    swag init -g cmd/server/main.go -o docs && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o server ./cmd/server && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o seed ./cmd/seed

FROM alpine:3.23 AS production
RUN apk add --no-cache ca-certificates
RUN addgroup -S -g 1001 appgroup && \
    adduser -S -u 1001 -G appgroup appuser
WORKDIR /app
COPY --from=builder --chown=appuser:appgroup /app/server /app/seed ./
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1
CMD ["./server"]
