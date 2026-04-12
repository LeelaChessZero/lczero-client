#!/bin/bash
BASE="ghcr.io/leelachesszero/lczero-client"
CUDA=$(nvidia-smi 2>/dev/null | awk -F'CUDA Version: ' 'NF>1{split($2,a," ");print a[1]}')
VARIANTS="11.5:cuda11-live 12.9:cuda12-live"
TAG=""
for v in $VARIANTS; do
    [ "$(printf '%s\n%s' "${v%%:*}" "$CUDA" | sort -V | head -1)" = "${v%%:*}" ] && TAG=${v#*:}
done
[ -z "$TAG" ] && { echo "CUDA $CUDA too old (need ≥11.5). Update your NVIDIA driver." >&2; exit 1; }
IMAGE="$BASE:$TAG"

NAME="lczero-client"
CHECK_PERIOD=600

trap 'docker rm -f $NAME 2>/dev/null; exit 0' INT TERM

docker pull $IMAGE
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