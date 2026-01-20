# ==============================================================================
# lc0-training-client Docker image
#
# - Builds lc0 (engine) from source against CUDA
# - Downloads lc0-training-client (Go binary) from GitHub releases
# - Uses /data as a single persistence mount (XDG config + cache)
# - Runs as non-root by default (UID 1000), but supports --user override
# ==============================================================================

# ==============================================================================
# STAGE 1: ENGINE BUILDER
# ==============================================================================
FROM nvidia/cuda:12.9.1-devel-ubuntu22.04 AS engine_builder

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    git build-essential ninja-build python3 python3-pip pkg-config \
    libprotobuf-dev protobuf-compiler \
    libopenblas-dev \
    curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN python3 -m pip install --no-cache-dir --upgrade pip meson

WORKDIR /build

# Use a real release tag, not a branch name.
ARG LC0_VERSION=v0.32.1

RUN git clone --branch "${LC0_VERSION}" --depth 1 \
      https://github.com/LeelaChessZero/lc0.git && \
    cd lc0 && \
    git submodule update --init --recursive

WORKDIR /build/lc0

# Build flags per spec (portable across GPU/CPU arch; no cuDNN/OpenCL/ONNX)
# NOTE: If meson errors with "unknown option gtest", just remove -Dgtest=false.
RUN meson setup build \
      --buildtype=release \
      --prefix=/app \
      -Dplain_cuda=true \
      -Dnative_cuda=false \
      -Dnative_arch=false \
      -Dcudnn=false \
      -Dopencl=false \
      -Donnx=false \
      -Dgtest=false \
    && meson compile -C build \
    && meson install -C build

# ==============================================================================
# STAGE 2: CLIENT DOWNLOADER
# ==============================================================================
FROM ubuntu:22.04 AS client_downloader

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ARG CLIENT_VERSION=v34

WORKDIR /download

# Download client binary
# NOTE: Add checksum verification later if/when upstream publishes checksums.
RUN curl -fSL --retry 3 --retry-delay 2 -o client \
      "https://github.com/LeelaChessZero/lczero-client/releases/download/${CLIENT_VERSION}/lc0-training-client-linux" \
    && chmod +x client

# ==============================================================================
# STAGE 3: RUNTIME
# ==============================================================================
FROM nvidia/cuda:12.9.1-runtime-ubuntu22.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    libprotobuf23 libopenblas-base libgomp1 ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -m -s /bin/bash -u 1000 lc0user

WORKDIR /app

COPY --from=engine_builder     /app/bin/lc0          ./lc0
COPY --from=client_downloader  /download/client     ./lc0-training-client

RUN chown -R lc0user:lc0user /app

# Create data dirs (will typically be overridden by a bind mount)
RUN mkdir -p /data/config /data/cache && chown -R lc0user:lc0user /data

# Environment
ENV PATH="/app:${PATH}"
ENV LC0_PATH="/app/lc0"

# XDG locations (client will create /data/config/lc0 and /data/cache/lc0 subdirs)
ENV XDG_CONFIG_HOME="/data/config"
ENV XDG_CACHE_HOME="/data/cache"

USER lc0user

ENTRYPOINT ["/app/lc0-training-client"]
CMD []

# Metadata labels
ARG LC0_VERSION
ARG CLIENT_VERSION
LABEL org.opencontainers.image.title="lc0-training-client"
LABEL org.opencontainers.image.description="Leela Chess Zero selfplay training client (bundled with lc0 engine)"
LABEL org.opencontainers.image.source="https://github.com/LeelaChessZero/lczero-client"
LABEL lc0.engine.version="${LC0_VERSION}"
LABEL lc0.client.version="${CLIENT_VERSION}"
