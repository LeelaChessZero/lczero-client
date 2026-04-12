# ==============================================================================
# STAGE 1: ENGINE BUILDER (C++ / CUDA)
# ==============================================================================
ARG CUDA_VERSION=12.9.1
ARG UBUNTU_VERSION=22.04
FROM nvidia/cuda:${CUDA_VERSION}-devel-ubuntu${UBUNTU_VERSION} AS engine_builder

ENV DEBIAN_FRONTEND=noninteractive

# 1. Install C++ Build Tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    git build-essential cmake ninja-build \
    libprotobuf-dev protobuf-compiler \
    libgtest-dev meson python3 python3-pip \
    curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 2. Ensure C++20 support (ubuntu20.04 ships GCC 9, which lacks <span>)
ARG GCC_INSTALL_VERSION=""
RUN if [ -n "$GCC_INSTALL_VERSION" ]; then \
        apt-get update && apt-get install -y software-properties-common && \
        add-apt-repository -y ppa:ubuntu-toolchain-r/test && \
        apt-get update && \
        apt-get install -y gcc-$GCC_INSTALL_VERSION g++-$GCC_INSTALL_VERSION && \
        update-alternatives --install /usr/bin/gcc gcc /usr/bin/gcc-$GCC_INSTALL_VERSION 100 && \
        update-alternatives --install /usr/bin/g++ g++ /usr/bin/g++-$GCC_INSTALL_VERSION 100 && \
        rm -rf /var/lib/apt/lists/*; \
    fi

# 3. Upgrade Meson
RUN pip3 install --upgrade meson

WORKDIR /app

# 4. Fetch Lc0
ARG LC0_REPO=https://github.com/LeelaChessZero/lc0
ARG LC0_BRANCH
RUN echo "Fetching Lc0: ${LC0_BRANCH}" && \
    git clone -b ${LC0_BRANCH} ${LC0_REPO} lc0 && \
    cd lc0 && \
    git submodule update --init --recursive

# 5. Compile Lc0
WORKDIR /app/lc0
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
    CGO_ENABLED=0 go build -ldflags "-s -w" -o /src/client-binary .


# ==============================================================================
# STAGE 3: RUNTIME (The Final Image)
# ==============================================================================
ARG CUDA_VERSION=12.9.1
ARG UBUNTU_VERSION=22.04
FROM nvidia/cuda:${CUDA_VERSION}-base-ubuntu${UBUNTU_VERSION}

ENV DEBIAN_FRONTEND=noninteractive

# 1. Install Runtime Dependencies
ARG CUDA_VERSION
ARG LIBPROTOBUF_VERSION=23
RUN CUDA_PKG=$(echo ${CUDA_VERSION} | cut -d. -f1-2 | tr . -) && \
    apt-get update && apt-get install -y --no-install-recommends \
    libcublas-${CUDA_PKG} \
    libprotobuf${LIBPROTOBUF_VERSION} \
    libopenblas-base \
    libgomp1 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 2. Copy Artifacts from Builders
COPY --from=engine_builder /app/dist/bin/lc0 ./lc0
COPY --from=client_builder /src/client-binary ./client

# 3. Default Command
ENTRYPOINT ["./client"]