#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
PACKAGE_DIR="${DIST_DIR}/qdl-server-linux-amd64"

cd "${ROOT_DIR}"
"${ROOT_DIR}/scripts/sync-admin-web.sh"

rm -rf "${PACKAGE_DIR}"
mkdir -p "${PACKAGE_DIR}/config"

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "${PACKAGE_DIR}/qdl-server" ./cmd/qdl-server
cp "${ROOT_DIR}/config/config.example.yaml" "${PACKAGE_DIR}/config/config.yaml"
cp "${ROOT_DIR}/README.md" "${PACKAGE_DIR}/README.md"

cd "${DIST_DIR}"
tar -czf qdl-server-linux-amd64.tar.gz qdl-server-linux-amd64
echo "${DIST_DIR}/qdl-server-linux-amd64.tar.gz"
