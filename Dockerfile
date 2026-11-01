# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/youtubegir ./cmd/youtubegir

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates curl ffmpeg unzip \
  && rm -rf /var/lib/apt/lists/*

ARG TARGETARCH
RUN set -eux; \
  case "$TARGETARCH" in \
    amd64) YT=yt-dlp_linux; DENO=x86_64-unknown-linux-gnu ;; \
    arm64) YT=yt-dlp_linux_aarch64; DENO=aarch64-unknown-linux-gnu ;; \
    *) echo "unsupported architecture: $TARGETARCH" >&2; exit 1 ;; \
  esac; \
  curl -fsSL -o /usr/local/bin/yt-dlp "https://github.com/yt-dlp/yt-dlp/releases/latest/download/${YT}"; \
  chmod 0755 /usr/local/bin/yt-dlp; \
  curl -fsSL -o /tmp/deno.zip "https://github.com/denoland/deno/releases/latest/download/deno-${DENO}.zip"; \
  unzip -d /usr/local/bin /tmp/deno.zip; \
  chmod 0755 /usr/local/bin/deno; \
  rm -f /tmp/deno.zip; \
  yt-dlp --version; \
  deno --version; \
  ffmpeg -version | head -n 1

COPY --from=build /out/youtubegir /usr/local/bin/youtubegir

ENV ADDR=:6543 \
    DATA_DIR=/data \
    DB_PATH=/data/db/youtubegir.sqlite \
    DOWNLOAD_DIR=/data/downloads \
    TEMP_DIR=/data/tmp \
    YTDLP_PATH=/usr/local/bin/yt-dlp \
    JS_RUNTIME=deno:/usr/local/bin/deno \
    GIN_MODE=release

RUN mkdir -p /data/db /data/downloads /data/tmp

EXPOSE 6543
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD curl -fsS http://127.0.0.1:6543/health || exit 1

ENTRYPOINT ["/usr/local/bin/youtubegir"]
