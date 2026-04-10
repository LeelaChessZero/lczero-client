#!/bin/bash
IMAGE="ghcr.io/leelachesszero/lczero-client:live"
NAME="lczero-client"
CHECK_PERIOD=600

trap 'docker rm -f $NAME 2>/dev/null; exit 0' INT TERM

if ! docker info 2>/dev/null | grep -q "Runtimes:.*nvidia"; then
    echo "ERROR: NVIDIA Container Toolkit not found. Install it with:"
    echo "  sudo apt install nvidia-container-toolkit && sudo systemctl restart docker"
    exit 1
fi

docker image inspect $IMAGE >/dev/null 2>&1 || docker pull $IMAGE
[ -f lc0-training-client-config.json ] || echo '{}' > lc0-training-client-config.json

while true; do
    ID=$(docker image inspect --format='{{.Id}}' $IMAGE)
    echo "Starting $IMAGE (${ID:7:12})..."

    (
        while sleep $CHECK_PERIOD; do
            docker pull -q $IMAGE >/dev/null 2>&1
            NEW=$(docker image inspect --format='{{.Id}}' $IMAGE)
            if [ "$ID" != "$NEW" ]; then
                echo "New version found, restarting..."
                docker rm -f $NAME >/dev/null 2>&1
                break
            fi
        done
    ) </dev/null &
    CHECKER=$!

    docker run -it --rm --name $NAME --gpus all \
        -v "$(pwd)/lc0-training-client-config.json":/app/lc0-training-client-config.json \
        $IMAGE "$@"

    kill $CHECKER 2>/dev/null
    wait $CHECKER 2>/dev/null
    sleep 1
done