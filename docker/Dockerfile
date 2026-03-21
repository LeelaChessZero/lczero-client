# ==============================================================================
# STAGE 1: ENGINE BUILDER (C++ / CUDA)
# ==============================================================================
ARG CUDA_VERSION=12.9.1
FROM nvidia/cuda:${CUDA_VERSION}-devel-ubuntu22.04 AS engine_builder

ENV DEBIAN_FRONTEND=noninteractive

# 1. Install C++ Build Tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    git build-essential cmake ninja-build \
    libprotobuf-dev protobuf-compiler \
    libgtest-dev meson python3 python3-pip \
    curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 2. Upgrade Meson
RUN pip3 install --upgrade meson

WORKDIR /app

# 3. Fetch Lc0
ARG LC0_REPO=https://github.com/LeelaChessZero/lc0
ARG LC0_BRANCH=release/0.32
RUN echo "Fetching Lc0: ${LC0_BRANCH}" && \
    git clone -b ${LC0_BRANCH} ${LC0_REPO} lc0 && \
    cd lc0 && \
    git submodule update --init --recursive

# 4. Compile Lc0
WORKDIR /app/lc0
# -Dplain_cuda=true   : Native CUDA backend
# -Dnative_cuda=false : Allow running on different GPUs than the build machine
# --prefix /app/dist  : Install to a clean folder for copying
ARG LC0_BUILD_OPTS="-Dplain_cuda=true -Dnative_cuda=false -Dnative_arch=false -Dopencl=false -Dcudnn=false -Donnx=false"
RUN meson setup build \
    ${LC0_BUILD_OPTS} \
    --prefix /app/dist \
    --buildtype release && \
    ninja -C build install


# ==============================================================================
# STAGE 2: CLIENT BUILDER (Go)
# ==============================================================================
FROM golang:1.24-bookworm AS client_builder

WORKDIR /src

# 1. Fetch Client
ARG CLIENT_REPO=https://github.com/LeelaChessZero/lczero-client
ARG CLIENT_BRANCH=release
RUN echo "Fetching Client: ${CLIENT_BRANCH}" && \
    git clone -b ${CLIENT_BRANCH} ${CLIENT_REPO} client

WORKDIR /src/client

# 2. Build Client
# 'go mod tidy' fixes the missing dependency issues automatically
RUN go mod tidy && \
    go build -ldflags "-s -w" -o /src/client-binary .


# ==============================================================================
# STAGE 3: RUNTIME (The Final Image)
# ==============================================================================
ARG CUDA_VERSION=12.9.1
FROM nvidia/cuda:${CUDA_VERSION}-runtime-ubuntu22.04

ENV DEBIAN_FRONTEND=noninteractive

# 1. Install Runtime Dependencies
#    We need libprotobuf (matches dev version 3.12.4), OpenBLAS, and OpenMP
RUN apt-get update && apt-get install -y --no-install-recommends \
    libprotobuf23 \
    libopenblas-base \
    libgomp1 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 2. Copy Artifacts from Builders
COPY --from=engine_builder /app/dist/bin/lc0 ./lc0
COPY --from=client_builder /src/client-binary ./client

# 3. Setup Environment
#    Ensure Lc0 can find the client if needed, or vice-versa
ENV LC0_PATH=/app/lc0

# 4. Default Command
ENTRYPOINT ["./client"]