# Dockerfile for airgradient-timescaledb-inserter.
#
# Three stages so the base image (stage2) caches independently of code
# changes (stage1/stage3). Stage 1 builds natively on the BUILDPLATFORM
# and cross-compiles via GOARCH=$TARGETARCH — no QEMU emulation for the
# multi-arch build. Modules are resolved with go mod download under a
# BuildKit cache mount rather than a committed vendor/ tree. The runtime
# is distroless/static (CGO-free static binary): ~2 MB, nonroot, and it
# already ships ca-certificates + tzdata.

# STAGE 1 — build the executable.
FROM --platform=$BUILDPLATFORM golang:1-bookworm AS stage1

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
ARG VER_PKG=github.com/michaelpeterswa/airgradient-timescaledb-inserter/internal/version

WORKDIR /var/build/go

# Module download is arch-independent, so it sits above TARGETARCH and is
# shared by every platform in a multi-arch build.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
ARG TARGETARCH
ENV GOARCH=$TARGETARCH CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build,id=go-build-$TARGETARCH \
    go build -v -trimpath \
      -ldflags "-s -w \
        -X ${VER_PKG}.Version=${VERSION} \
        -X ${VER_PKG}.Commit=${COMMIT} \
        -X ${VER_PKG}.Date=${DATE}" \
      -o /var/build/bin/ ./cmd/...

# STAGE 2 — base image (cached by digest, independent of source).
# distroless/static already includes ca-certificates and tzdata, so there
# is no apt layer to run here.
FROM gcr.io/distroless/static-debian12:nonroot AS stage2

# STAGE 3 — construct the final image.
FROM stage2 AS stage3

COPY --from=stage1 /var/build/bin/* /usr/local/bin/

EXPOSE 8081
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/airgradient-timescaledb-inserter"]
