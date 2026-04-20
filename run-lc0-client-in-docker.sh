#!/bin/bash

GPU=0
ARGS=()
for arg in "$@"; do
    case $arg in
        --gpu=*) GPU=${arg#*=} ;;
        *) ARGS+=("$arg") ;;
    esac
done
set -- "${ARGS[@]}"

BASE="ghcr.io/leelachesszero/lczero-client"
CUDA=$(nvidia-smi 2>/dev/null | awk -F'CUDA Version: ' 'NF>1{split($2,a," ");print a[1]}')
VARIANTS="11.5:cuda11-live 12.9:cuda12-live"
TAG=""
for variant in $VARIANTS; do
    if [ "$(printf '%s\n%s' "${variant%%:*}" "$CUDA" | sort -V | head -1)" = "${variant%%:*}" ]; then
        TAG=${variant#*:}
    fi
done

if [ -z "$TAG" ]; then
    echo "CUDA $CUDA too old (need ≥11.5). Update your NVIDIA driver." >&2
    exit 1
fi

IMAGE="$BASE:$TAG"
NAME="lczero-client-gpu$GPU"
CHECK_PERIOD=600
CONFIG_PATH="$PWD/lc0-training-client-config.json"

# Keep restart state private to this script invocation.
STATE_DIR=$(mktemp -d)
RESTART_FLAG="$STATE_DIR/restart"

docker pull "$IMAGE"
[ -f "$CONFIG_PATH" ] || echo '{}' > "$CONFIG_PATH"

while true; do
    rm -f "$RESTART_FLAG"
    ID=$(docker image inspect --format='{{.Id}}' "$IMAGE")
    echo "Starting $IMAGE (${ID:7:12})..."

    # Restart only when a newer image is pulled and this flag is set.
    (
        while sleep "$CHECK_PERIOD"; do
            docker pull -q "$IMAGE" >/dev/null 2>&1 || continue
            NEW=$(docker image inspect --format='{{.Id}}' "$IMAGE") || continue
            [ "$ID" = "$NEW" ] && continue
            echo "New version found, restarting..."
            touch "$RESTART_FLAG"
            docker rm -f "$NAME" >/dev/null 2>&1
            exit 0
        done
    ) </dev/null &
    CHECKER=$!

    docker run -i --rm --name "$NAME" --gpus "device=$GPU" \
        -v "$CONFIG_PATH":/app/lc0-training-client-config.json \
        "$IMAGE" "$@"
    STATUS=$?

    kill "$CHECKER" 2>/dev/null
    wait "$CHECKER" 2>/dev/null

    [ -e "$RESTART_FLAG" ] || break
    sleep 1
done

# Remove per-run state before returning Docker's exit status.
rm -rf "$STATE_DIR"
exit "$STATUS"
