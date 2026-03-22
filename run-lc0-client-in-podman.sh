#!/bin/bash
IMAGE="ghcr.io/leelachesszero/lczero-client:live"
NAME="lczero-client"
CHECK_PERIOD=600

trap 'podman rm -f $NAME 2>/dev/null; exit 0' INT TERM

podman image inspect $IMAGE >/dev/null 2>&1 || podman pull $IMAGE
[ -f lc0-training-client-config.json ] || echo '{}' > lc0-training-client-config.json

while true; do
    ID=$(podman image inspect --format='{{.Id}}' $IMAGE)
    echo "Starting $IMAGE (${ID:7:12})..."

    (
        while sleep $CHECK_PERIOD; do
            podman pull -q $IMAGE >/dev/null 2>&1
            NEW=$(podman image inspect --format='{{.Id}}' $IMAGE)
            if [ "$ID" != "$NEW" ]; then
                echo "New version found, restarting..."
                podman rm -f $NAME >/dev/null 2>&1
                break
            fi
        done
    ) </dev/null &
    CHECKER=$!

    podman run -it --rm --name $NAME --device nvidia.com/gpu=all \
        -v ./lc0-training-client-config.json:/app/lc0-training-client-config.json \
        $IMAGE "$@"

    kill $CHECKER 2>/dev/null
    wait $CHECKER 2>/dev/null
    sleep 1
done