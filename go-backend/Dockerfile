FROM golang:1.25-bookworm AS dev

WORKDIR /app

RUN go install github.com/swaggo/swag/cmd/swag@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy && swag init -g cmd/server/main.go -o docs && go build -o server ./cmd/server

CMD ["sh", "-c", "swag init -g cmd/server/main.go -o docs && go run ./cmd/server"]

# --- Production ---
FROM golang:1.25-bookworm AS builder

WORKDIR /app

RUN go install github.com/swaggo/swag/cmd/swag@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy && swag init -g cmd/server/main.go -o docs && CGO_ENABLED=0 go build -ldflags="-s -w" -o server ./cmd/server

FROM debian:bookworm-slim AS production
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 8090
CMD ["./server"]
