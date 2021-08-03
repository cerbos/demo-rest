#! /usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTAINER_IMG=${CONTAINER_IMG:-"ghcr.io/cerbos/cerbos"}
CONTAINER_TAG=${CONTAINER_TAG:-"0.4.0"}

docker run -i -t -p 3592:3592 -p 3593:3593 \
  -v ${SCRIPT_DIR}/policies:/policies \
  "${CONTAINER_IMG}:${CONTAINER_TAG}"

