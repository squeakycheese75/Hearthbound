# syntax=docker/dockerfile:1

# 1. The browser UI (with the art pack) -> web/dist
FROM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# 2. The server, with the UI embedded. Pure Go (no cgo), so it runs anywhere.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/tabletop ./cmd/tabletop
RUN mkdir -p /out/data/backups

# 3. A minimal image that runs as a non-root user.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/tabletop /tabletop
COPY --from=build --chown=65532:65532 /out/data /data
ENV TABLETOP_ADDR=0.0.0.0:8080 \
    TABLETOP_DB=/data/tabletop.db \
    TABLETOP_BACKUP_DIR=/data/backups \
    TABLETOP_LOG_FORMAT=json
VOLUME /data
EXPOSE 8080
USER nonroot
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s CMD ["/tabletop", "-healthcheck"]
ENTRYPOINT ["/tabletop"]
