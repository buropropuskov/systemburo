# Replace only gosu; PostgreSQL, entrypoint, user and data paths stay inherited.
FROM golang:1.27.3-bookworm AS gosu-build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOBIN=/out
RUN go install -v -trimpath -ldflags="-d -w" github.com/tianon/gosu@29b939aa5bd4dc588af765b8248f6e39150d2565

FROM postgres:16-alpine@sha256:0cefaabd882e42bcb2a2f3dc56d3ce4770560ba62fd7f197f1a2e008fd5731ab AS production
ENV GOSU_VERSION=1.20
COPY --from=gosu-build /out/gosu /usr/local/bin/gosu
