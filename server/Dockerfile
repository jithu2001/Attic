# syntax=docker/dockerfile:1

# ---- build ----------------------------------------------------------------
FROM golang:1.22-bookworm AS build

WORKDIR /src

# Dependencies first so the module cache survives source edits.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build \
      -ldflags "-s -w -X github.com/perleybrook/attic/server/internal/api.Version=${VERSION}" \
      -o /out/attic ./cmd/Attic

# ---- runtime --------------------------------------------------------------
# FFmpeg with Intel QSV / VAAPI support for hardware transcoding. The runtime
# image stays Debian-based because the VAAPI drivers are not available on Alpine.
FROM debian:bookworm-slim AS runtime

RUN apt-get update && apt-get install -y --no-install-recommends \
      ffmpeg \
      intel-media-va-driver-non-free \
      i965-va-driver \
      vainfo \
      libvips42 \
      ca-certificates \
      tzdata \
    && rm -rf /var/lib/apt/lists/*

RUN useradd --system --uid 10001 --create-home --home-dir /home/attic attic \
    && mkdir -p /data/originals /data/derived \
    && chown -R attic:attic /data

COPY --from=build /out/attic /usr/local/bin/attic

USER attic
WORKDIR /home/attic
EXPOSE 8080
ENV ATTIC_LISTEN_ADDR=:8080 \
    ATTIC_DATA_DIR=/data

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD ["/usr/local/bin/attic", "-healthcheck"]

ENTRYPOINT ["/usr/local/bin/attic"]
