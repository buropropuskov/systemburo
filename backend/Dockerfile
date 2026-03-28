# ──────────────────────────────────────────────
# Dev stage: hot-reload with cargo-watch
# ──────────────────────────────────────────────
FROM rust:1.82-bookworm AS dev

RUN cargo install cargo-watch

WORKDIR /app

# Copy manifests first for dependency caching
COPY Cargo.toml Cargo.lock ./

# Create a dummy main.rs so cargo can resolve and build dependencies
RUN mkdir src && echo "fn main() {}" > src/main.rs
RUN cargo build && rm -rf src

# Copy the real source code
COPY . .

ENV SQLX_OFFLINE=true

CMD ["cargo", "watch", "-x", "run"]

# ──────────────────────────────────────────────
# Builder stage: compile the release binary
# ──────────────────────────────────────────────
FROM rust:1.82-bookworm AS builder

WORKDIR /app

# Copy manifests first for dependency caching
COPY Cargo.toml Cargo.lock ./

# Create a dummy main.rs so cargo can resolve and build dependencies
RUN mkdir src && echo "fn main() {}" > src/main.rs
RUN cargo build --release && rm -rf src

# Copy the real source code
COPY . .

ENV SQLX_OFFLINE=true

RUN cargo build --release

# ──────────────────────────────────────────────
# Production stage: minimal runtime image
# ──────────────────────────────────────────────
FROM debian:bookworm-slim AS production

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libssl3 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/target/release/backend .

RUN mkdir -p uploads

EXPOSE 8080

ENV BIND_HOST=0.0.0.0

CMD ["./backend"]
