#!/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR"

DOCKER_USER="kaffannen"
IMAGE_NAME="cloud-controller"

docker build \
    --build-arg BINARY_NAME=${IMAGE_NAME} \
    -t ${DOCKER_USER}/${IMAGE_NAME}:latest \
    .

docker push \
    ${DOCKER_USER}/${IMAGE_NAME}:latest
