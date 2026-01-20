#!/usr/bin/env bash
# lc0-training-client auto-updater (host-side)
# - No docker.sock mount
# - Pulls + compares image ID
# - Restarts container when image changes
# - If client exits with code 5, pulls immediately and restarts

set -euo pipefail

# ------------------------------------------------------------------------------
# Defaults (override via env or CLI)
# ------------------------------------------------------------------------------
IMAGE="${IMAGE:-ghcr.io/leelachesszero/lc0-training-client:latest}"
CONTAINER_NAME="${CONTAINER_NAME:-lc0-training}"
DATA_DIR="${DATA_DIR:-$HOME/.lc0-training}"

# How often to check for new images while the container is running (seconds)
CHECK_INTERVAL="${CHECK_INTERVAL:-3600}"
# How often to poll whether the container is still running (seconds)
POLL_INTERVAL="${POLL_INTERVAL:-10}"

# docker stop timeout (seconds)
STOP_TIMEOUT="${STOP_TIMEOUT:-30}"

# LOG_FILE default is set after arg parsing (so --data-dir takes effect)

# Optional knobs
GPU_IDS="${GPU_IDS:-}"           # e.g. "0" or "0,2" (sets NVIDIA_VISIBLE_DEVICES)
NETWORK_MODE="${NETWORK_MODE:-}" # "" (default) or "host"
SKIP_GPU_CHECK="${SKIP_GPU_CHECK:-false}"

# ------------------------------------------------------------------------------
# Logging (initialized after arg parsing)
# ------------------------------------------------------------------------------
init_logging() {
  # Set LOG_FILE default now that DATA_DIR is finalized
  LOG_FILE="${LOG_FILE:-$DATA_DIR/updater.log}"
  mkdir -p "$(dirname "$LOG_FILE")" >/dev/null 2>&1 || true
}

log() {
  local level="$1"; shift
  local msg="$*"
  local ts
  ts="$(date '+%Y-%m-%d %H:%M:%S')"
  # Output to both stdout and log file
  printf '[%s] %-5s %s\n' "$ts" "$level" "$msg" | tee -a "$LOG_FILE"
}

usage() {
  cat <<EOF
lc0-training-client auto-updater

Usage:
  $0 --user=USERNAME --password=PASSWORD [OPTIONS] [-- <client-args...>]

Required:
  --user=USER            lczero.org username
  --password=PASS        lczero.org password

Options:
  --image=IMAGE          Docker image (default: $IMAGE)
  --name=NAME            Container name (default: $CONTAINER_NAME)
  --data-dir=DIR         Data directory mounted to /data (default: $DATA_DIR)
  --check-interval=N     Seconds between update checks (default: $CHECK_INTERVAL)
  --poll-interval=N      Seconds between container liveness polls (default: $POLL_INTERVAL)
  --stop-timeout=N       docker stop timeout seconds (default: $STOP_TIMEOUT)
  --gpu=IDLIST           GPU ids like "0" or "0,2" (sets NVIDIA_VISIBLE_DEVICES)
  --network=MODE         "host" to use host networking (default: bridge)
  --skip-gpu-check       Skip docker GPU smoke test
  -h, --help             Show this help

Examples:
  $0 --user=myname --password=secret
  $0 --user=myname --password=secret --gpu=0 --check-interval=1800
  $0 --user=myname --password=secret -- --verbose
EOF
}

# ------------------------------------------------------------------------------
# Args
# ------------------------------------------------------------------------------
USER_ARG=""
PASSWORD_ARG=""
EXTRA_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --user=*) USER_ARG="${1#*=}"; shift ;;
    --password=*) PASSWORD_ARG="${1#*=}"; shift ;;
    --image=*) IMAGE="${1#*=}"; shift ;;
    --name=*) CONTAINER_NAME="${1#*=}"; shift ;;
    --data-dir=*) DATA_DIR="${1#*=}"; shift ;;
    --check-interval=*) CHECK_INTERVAL="${1#*=}"; shift ;;
    --poll-interval=*) POLL_INTERVAL="${1#*=}"; shift ;;
    --stop-timeout=*) STOP_TIMEOUT="${1#*=}"; shift ;;
    --gpu=*) GPU_IDS="${1#*=}"; shift ;;
    --network=*) NETWORK_MODE="${1#*=}"; shift ;;
    --skip-gpu-check) SKIP_GPU_CHECK="true"; shift ;;
    --) shift; EXTRA_ARGS+=("$@"); break ;;
    -h|--help) usage; exit 0 ;;
    *) EXTRA_ARGS+=("$1"); shift ;;
  esac
done

if [[ -z "$USER_ARG" || -z "$PASSWORD_ARG" ]]; then
  # Initialize logging with default DATA_DIR for error message
  init_logging
  log ERROR "Missing required args: --user and --password"
  usage
  exit 1
fi

# Initialize logging now that DATA_DIR is finalized
init_logging

# ------------------------------------------------------------------------------
# Docker helpers
# ------------------------------------------------------------------------------
check_prerequisites() {
  command -v docker >/dev/null 2>&1 || { log ERROR "Docker not found in PATH"; exit 1; }
  docker info >/dev/null 2>&1 || { log ERROR "Docker daemon not running or permission denied"; exit 1; }

  if [[ "$SKIP_GPU_CHECK" != "true" ]]; then
    log INFO "GPU smoke test: docker run --gpus all ... nvidia-smi"
    if ! docker run --rm --gpus all nvidia/cuda:12.9.1-base-ubuntu22.04 nvidia-smi >/dev/null 2>&1; then
      log ERROR "GPU access in Docker failed."
      log ERROR "If you haven't configured the NVIDIA container runtime, try:"
      log ERROR "  sudo nvidia-ctk runtime configure --runtime=docker"
      log ERROR "  sudo systemctl restart docker"
      exit 1
    fi
  fi
}

pull_image_required() {
  log INFO "Pulling image: $IMAGE"
  docker pull "$IMAGE" >/dev/null
}

pull_image_best_effort() {
  log INFO "Pulling image: $IMAGE"
  if ! docker pull "$IMAGE" >/dev/null 2>&1; then
    log WARN "docker pull failed (will retry later)"
    return 1
  fi
}

get_image_id() {
  docker image inspect --format '{{.Id}}' "$IMAGE" 2>/dev/null || true
}

container_exists() {
  docker ps -aq -f "name=^/${CONTAINER_NAME}$" | grep -q .
}

container_running() {
  docker ps -q -f "name=^/${CONTAINER_NAME}$" | grep -q .
}

stop_container() {
  if container_running; then
    log INFO "Stopping container (timeout ${STOP_TIMEOUT}s): $CONTAINER_NAME"
    docker stop --time="$STOP_TIMEOUT" "$CONTAINER_NAME" >/dev/null 2>&1 || true
  fi
  if container_exists; then
    docker rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
  fi
}

start_container() {
  mkdir -p "$DATA_DIR/config" "$DATA_DIR/cache"

  log INFO "Starting container: $CONTAINER_NAME"
  local args=(docker run -d
    --name "$CONTAINER_NAME"
    --restart "no"
    --user "$(id -u):$(id -g)"
    --gpus all
    -v "$DATA_DIR:/data"
  )

  if [[ -n "$GPU_IDS" ]]; then
    # NVIDIA container toolkit supports selecting visible GPUs via env var.
    args+=(-e "NVIDIA_VISIBLE_DEVICES=$GPU_IDS")
  fi

  if [[ "$NETWORK_MODE" == "host" ]]; then
    args+=(--network host)
  fi

  args+=("$IMAGE" --user="$USER_ARG" --password="$PASSWORD_ARG")
  args+=("${EXTRA_ARGS[@]}")

  "${args[@]}" >/dev/null
  log INFO "Container started."
}

get_exit_code() {
  docker inspect --format '{{.State.ExitCode}}' "$CONTAINER_NAME" 2>/dev/null || echo "unknown"
}

# ------------------------------------------------------------------------------
# Main
# ------------------------------------------------------------------------------
cleanup() {
  log INFO "Shutting down..."
  stop_container
  exit 0
}
trap cleanup SIGINT SIGTERM

main() {
  log INFO "=== lc0-training-client auto-updater ==="
  log INFO "Image: $IMAGE"
  log INFO "Container: $CONTAINER_NAME"
  log INFO "Data dir: $DATA_DIR"
  log INFO "Check interval: ${CHECK_INTERVAL}s"
  log INFO "Poll interval: ${POLL_INTERVAL}s"
  if [[ -n "$GPU_IDS" ]]; then
    log INFO "GPU IDs: $GPU_IDS (NVIDIA_VISIBLE_DEVICES)"
  fi
  if [[ "$NETWORK_MODE" == "host" ]]; then
    log INFO "Network: host"
  fi

  check_prerequisites

  pull_image_required
  local current_id
  current_id="$(get_image_id)"
  if [[ -z "$current_id" ]]; then
    log ERROR "Could not inspect image ID after pull: $IMAGE"
    exit 1
  fi
  log INFO "Current image ID: ${current_id:0:12}"

  local last_check_ts
  last_check_ts="$(date +%s)"

  while true; do
    stop_container
    start_container

    while true; do
      if ! container_running; then
        local exit_code
        exit_code="$(get_exit_code)"
        log WARN "Container exited with code: $exit_code"

        # Client uses exit code 5 to indicate "upgrade/update required"
        if [[ "$exit_code" == "5" ]]; then
          log INFO "Exit code 5: update required -> pulling immediately"
          pull_image_best_effort || true
          current_id="$(get_image_id)"
          if [[ -n "$current_id" ]]; then
            log INFO "Now using image ID: ${current_id:0:12}"
          fi
        else
          log INFO "Restarting shortly..."
          sleep 2
        fi
        break
      fi

      local now
      now="$(date +%s)"

      if (( now - last_check_ts >= CHECK_INTERVAL )); then
        last_check_ts="$now"
        log INFO "Checking for updates..."
        pull_image_best_effort || true

        local new_id
        new_id="$(get_image_id)"
        if [[ -n "$new_id" && "$new_id" != "$current_id" ]]; then
          log INFO "New image detected:"
          log INFO "  Old: ${current_id:0:12}"
          log INFO "  New: ${new_id:0:12}"
          current_id="$new_id"
          break
        else
          log INFO "No updates."
        fi
      fi

      sleep "$POLL_INTERVAL"
    done
  done
}

main
