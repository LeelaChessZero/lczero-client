# Docker

Run the lc0 training client in a container with NVIDIA GPU support.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Volume Mounts](#volume-mounts)
- [Image Tags](#image-tags)
- [GPU Selection](#gpu-selection)
- [Auto-Updater](#auto-updater)
- [Exit Codes](#exit-codes)
- [Running as a Service](#running-as-a-service)
- [Building Locally](#building-locally)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### 1. NVIDIA Drivers

Verify drivers are installed:

```bash
nvidia-smi
```

### 2. Docker with NVIDIA Container Toolkit

One-time setup:

```bash
# Configure Docker runtime
sudo nvidia-ctk runtime configure --runtime=docker
sudo systemctl restart docker

# Verify GPU access in Docker
docker run --rm --gpus all nvidia/cuda:12.9.1-base-ubuntu22.04 nvidia-smi
```

See [NVIDIA Container Toolkit docs](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html) for full installation instructions.

## Quick Start

### Manual Run

```bash
# Create persistent data directory
mkdir -p ~/.lc0-training

# Run the client
docker run --gpus all \
  --user $(id -u):$(id -g) \
  -v ~/.lc0-training:/data \
  ghcr.io/leelachesszero/lc0-training-client:latest \
  --user=YOUR_USERNAME --password=YOUR_PASSWORD
```

### With Auto-Updater (Recommended)

```bash
# Download the script
curl -fLO https://raw.githubusercontent.com/LeelaChessZero/lczero-client/release/scripts/auto-update.sh
chmod +x auto-update.sh

# Run (handles updates automatically)
./auto-update.sh --user=YOUR_USERNAME --password=YOUR_PASSWORD
```

## Volume Mounts

Mount a single directory to `/data` for persistent storage:

```bash
-v ~/.lc0-training:/data
```

The container uses XDG conventions:

- `/data/config` → `XDG_CONFIG_HOME`
- `/data/cache` → `XDG_CACHE_HOME`

The client creates subdirectories as needed (config JSON, downloaded networks, opening books).

### File Ownership

Use `--user $(id -u):$(id -g)` to ensure files are owned by your host user, not root.

## Image Tags

| Tag | Description | Use Case |
|-----|-------------|----------|
| `:latest` | Current stable release | Recommended for most users |
| `:lc0-vX.Y.Z` | Pinned to engine version | Pin to specific lc0 version |
| `:client-vN` | Pinned to client version | Pin to specific client version |
| `:lc0-vX.Y.Z_client-vN` | Fully pinned | Reproducible builds |

Examples:

```bash
# Latest stable (auto-updates via script)
ghcr.io/leelachesszero/lc0-training-client:latest

# Pinned to lc0 v0.32.1
ghcr.io/leelachesszero/lc0-training-client:lc0-v0.32.1

# Fully pinned
ghcr.io/leelachesszero/lc0-training-client:lc0-v0.32.1_client-v34
```

## GPU Selection

Default is all GPUs:

```bash
docker run --gpus all ...
```

To restrict visible GPUs, use `NVIDIA_VISIBLE_DEVICES`:

```bash
# Single GPU
docker run --gpus all -e NVIDIA_VISIBLE_DEVICES=0 ...

# Multiple specific GPUs
docker run --gpus all -e NVIDIA_VISIBLE_DEVICES=0,2 ...
```

With the auto-updater, use `--gpu`:

```bash
./auto-update.sh --user=USER --password=PASS --gpu=0
./auto-update.sh --user=USER --password=PASS --gpu=0,2
```

Note: The `--gpu` option sets `NVIDIA_VISIBLE_DEVICES` while still passing `--gpus all` to Docker. This is the NVIDIA-recommended way to restrict GPU visibility.

For multi-GPU farms, run one container per GPU with different names:

```bash
./auto-update.sh --user=USER --password=PASS --gpu=0 --name=lc0-gpu0
./auto-update.sh --user=USER --password=PASS --gpu=1 --name=lc0-gpu1
```

## Auto-Updater

The `scripts/auto-update.sh` script manages the container lifecycle and handles updates automatically. It runs on the host (no `docker.sock` mount required).

### Features

- Polls container liveness every 10 seconds (configurable)
- Checks for new images hourly (configurable)
- Handles exit code 5 (server-requested update) with immediate pull
- Graceful shutdown on SIGINT/SIGTERM
- Logs to stdout and file

### Usage

```bash
./auto-update.sh --user=USERNAME --password=PASSWORD [OPTIONS] [-- <client-args...>]
```

### Options

| Option | Default | Description |
|--------|---------|-------------|
| `--user=USER` | (required) | lczero.org username |
| `--password=PASS` | (required) | lczero.org password |
| `--image=IMAGE` | `ghcr.io/.../lc0-training-client:latest` | Docker image |
| `--name=NAME` | `lc0-training` | Container name |
| `--data-dir=DIR` | `~/.lc0-training` | Data directory |
| `--check-interval=N` | `3600` | Seconds between image update checks |
| `--poll-interval=N` | `10` | Seconds between container liveness polls |
| `--stop-timeout=N` | `30` | Graceful stop timeout |
| `--gpu=IDLIST` | (all) | GPU IDs (e.g., `0` or `0,2`) |
| `--network=MODE` | bridge | `host` for host networking |
| `--skip-gpu-check` | false | Skip startup GPU smoke test |

### Environment Variables

All options can also be set via environment variables:

```bash
export IMAGE="ghcr.io/leelachesszero/lc0-training-client:latest"
export CONTAINER_NAME="lc0-training"
export DATA_DIR="$HOME/.lc0-training"
export CHECK_INTERVAL=3600
export POLL_INTERVAL=10
export STOP_TIMEOUT=30
export GPU_IDS="0"
export NETWORK_MODE="host"
export LOG_FILE="$DATA_DIR/updater.log"
```

### Passing Extra Client Flags

Use `--` to pass additional arguments to the client:

```bash
./auto-update.sh --user=USER --password=PASS -- --verbose
```

## Exit Codes

| Code | Meaning | Auto-Updater Behavior |
|------|---------|----------------------|
| 0 | Normal exit | Restart after short delay |
| 5 | Server requires update | Immediate pull + restart |
| Other | Error/crash | Restart after short delay |

Exit code 5 is sent by the training server when a new client or engine version is required. The auto-updater treats this as "pull now and restart."

## Running as a Service

### systemd Unit (Linux)

Create `/etc/systemd/system/lc0-training.service`:

```ini
[Unit]
Description=lc0-training-client auto-updater
After=docker.service
Requires=docker.service

[Service]
Type=simple
User=YOUR_USERNAME
WorkingDirectory=/home/YOUR_USERNAME
ExecStart=/home/YOUR_USERNAME/auto-update.sh --user=LC0_USER --password=LC0_PASS
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable lc0-training
sudo systemctl start lc0-training

# View logs
journalctl -u lc0-training -f
```

### Credentials via Environment File

To avoid credentials in the unit file, create `/etc/lc0-training.env` (root-owned, mode 600):

```
LC0_USER=your_username
LC0_PASS=your_password
```

Then reference it in the unit:

```ini
[Service]
EnvironmentFile=/etc/lc0-training.env
ExecStart=/home/YOUR_USERNAME/auto-update.sh --user=${LC0_USER} --password=${LC0_PASS}
```

### Multiple GPUs with systemd

Create a template unit `/etc/systemd/system/lc0-training@.service`:

```ini
[Unit]
Description=lc0-training-client (GPU %i)
After=docker.service
Requires=docker.service

[Service]
Type=simple
User=YOUR_USERNAME
WorkingDirectory=/home/YOUR_USERNAME
ExecStart=/home/YOUR_USERNAME/auto-update.sh --user=LC0_USER --password=LC0_PASS --gpu=%i --name=lc0-gpu%i
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable per GPU:

```bash
sudo systemctl enable lc0-training@0 lc0-training@1
sudo systemctl start lc0-training@0 lc0-training@1
```

## Building Locally

```bash
# Clone the repo
git clone https://github.com/LeelaChessZero/lczero-client.git
cd lczero-client

# Build with defaults
docker build -f docker/Dockerfile.cuda -t lc0-training-client .

# Build with specific versions
docker build -f docker/Dockerfile.cuda \
  --build-arg LC0_VERSION=v0.32.1 \
  --build-arg CLIENT_VERSION=v34 \
  -t lc0-training-client .
```

### Testing the Build

```bash
# Test client binary
docker run --rm lc0-training-client --help

# Test engine binary (requires entrypoint override)
docker run --rm --entrypoint /app/lc0 lc0-training-client --help
```

## Troubleshooting

### "could not select device driver "" with capabilities: [[gpu]]"

NVIDIA container runtime isn't configured:

```bash
sudo nvidia-ctk runtime configure --runtime=docker
sudo systemctl restart docker
docker run --rm --gpus all nvidia/cuda:12.9.1-base-ubuntu22.04 nvidia-smi
```

### Permission denied writing to ~/.lc0-training

Run with UID/GID mapping:

```bash
docker run --user $(id -u):$(id -g) ...
```

If files are already owned by root:

```bash
sudo chown -R $(id -u):$(id -g) ~/.lc0-training
```

### Client says "you must upgrade" / exits repeatedly

This is expected during forced upgrades. The auto-updater will pull the new image and restart.

If it loops endlessly:
1. Confirm you're running `:latest`
2. Verify `docker pull` is succeeding
3. Check logs: `~/.lc0-training/updater.log`

### Network issues

If experiencing DNS or proxy problems, try host networking:

```bash
# Manual
docker run --network host --gpus all ...

# Auto-updater
./auto-update.sh --user=USER --password=PASS --network=host
```

Note: Host networking grants additional privileges and is not recommended as the default.

### Image pull failures

```bash
# Check connectivity
curl -I https://ghcr.io

# Pull manually
docker pull ghcr.io/leelachesszero/lc0-training-client:latest
```

### "Unknown option gtest" during local build

Remove `-Dgtest=false` from the Dockerfile. This flag may not exist in all lc0 versions.

## Future Work

### Rebuild on lc0 Engine Releases

The workflow supports `repository_dispatch` with event type `lc0-release`. To enable automatic rebuilds when lc0 releases:

1. Add a workflow to `LeelaChessZero/lc0` that sends dispatch with payload `{"lc0_version": "vX.Y.Z"}`
2. Configure PAT/token in lc0 repo

### Edge Builds (lc0 master)

To add nightly `:edge` builds from lc0 `master`:

1. Add `schedule` trigger to workflow
2. Add logic to set `LC0_VERSION=master` and tag only `:edge`
3. Document `:edge` usage in this file
