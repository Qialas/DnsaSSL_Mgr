#!/usr/bin/env bash
set -euo pipefail

SERVER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROOT_DIR="$(cd "${SERVER_DIR}/.." && pwd)"
ADMIN_DIR="${ROOT_DIR}/admin-web"
EMBED_DIR="${SERVER_DIR}/internal/web/dist"

if [ ! -d "${ADMIN_DIR}" ]; then
  echo "admin-web directory not found: ${ADMIN_DIR}" >&2
  exit 1
fi

cd "${ADMIN_DIR}"
npm run build

rm -rf "${EMBED_DIR}"
mkdir -p "${EMBED_DIR}"
cp -R "${ADMIN_DIR}/dist/." "${EMBED_DIR}/"

echo "synced admin web to ${EMBED_DIR}"
