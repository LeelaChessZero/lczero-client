#!/usr/bin/env bash
# Autoupdate script for lc0 client. Use: "./client.sh" followed by any other
# client options. If you need to override system compiler you can run it
# setting CC and CXX, e.g. "CC=gcc-8 CXX=g++-8 ./client.sh".

trap "exit" INT

rm -f lc0
git clone --depth 1 --recurse-submodules https://github.com/LeelaChessZero/lc0.git
TAG=
ERR=
FIRST=true
while [ -d lc0 ]
do
  cd lc0

  git fetch --tags --depth 1
  NEW_TAG=$(git tag --list |grep -v rc |tail -1)
  if [ "$TAG" == "$NEW_TAG" ]
  then
    if [ $ERR -eq 5 ]
    then
      NEW_TAG=$(git tag --list |grep rc |tail -1)
    else
      NEW_TAG=$(git tag --list |grep -v rc |tail -2 |head -1)
   fi
  fi
  TAG=$NEW_TAG
  git checkout $TAG

  git submodule update --remote
  git submodule update --checkout
  rm -rf build
  meson build --buildtype release -Db_lto=true -Dgtest=false

  cd build
  ninja
  cd ../..

  rm -f lc0-training-client-linux
  curl -s -L https://github.com/LeelaChessZero/lczero-client/releases/latest | egrep -o '/LeelaChessZero/lczero-client/releases/download/.*/lc0-training-client-linux' | head -n 1 | wget --base=https://github.com/ -i -
  chmod +x lc0-training-client-linux

  PATH=lc0/build ./lc0-training-client-linux "$@"
  ERR=$?
  if [ $ERR -ne 5 ] && $FIRST
  then
    break
  fi
  FIRST=false
  echo Update needed, starting process shortly.
  sleep 60
done


