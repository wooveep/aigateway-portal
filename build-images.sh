#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${ROOT_DIR}"

IMAGE_NAME="${IMAGE_NAME:-aigateway/portal:0.2.0}"
DOCKERFILE="${DOCKERFILE:-backend/Dockerfile}"
CONTEXT_DIR="${CONTEXT_DIR:-.}"
PLATFORMS="${PLATFORMS:-}"
PUSH="${PUSH:-false}"

require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "[ERROR] Missing command: ${cmd}" >&2
    exit 1
  fi
}

require_cmd docker

if [[ ! -f "${DOCKERFILE}" ]]; then
  echo "[ERROR] Dockerfile not found: ${DOCKERFILE}" >&2
  exit 1
fi

if [[ -n "${PLATFORMS}" && "${PUSH}" != "true" && "${PLATFORMS}" == *,* ]]; then
  cat >&2 <<MSG
[ERROR] Multi-platform build without push is not supported by default.
        Set PUSH=true for multi-platform image publishing.
MSG
  exit 1
fi

if [[ -n "${PLATFORMS}" || "${PUSH}" == "true" ]]; then
  require_cmd docker
  buildx_args=(buildx build -f "${DOCKERFILE}" -t "${IMAGE_NAME}")
  if [[ -n "${PLATFORMS}" ]]; then
    buildx_args+=(--platform "${PLATFORMS}")
  fi
  if [[ "${PUSH}" == "true" ]]; then
    buildx_args+=(--push)
  else
    buildx_args+=(--load)
  fi
  buildx_args+=("${CONTEXT_DIR}")

  echo "[INFO] docker ${buildx_args[*]}"
  docker "${buildx_args[@]}"
else
  echo "[INFO] docker build -f ${DOCKERFILE} -t ${IMAGE_NAME} ${CONTEXT_DIR}"
  docker build -f "${DOCKERFILE}" -t "${IMAGE_NAME}" "${CONTEXT_DIR}"
fi

echo "[OK] Image build completed: ${IMAGE_NAME}"
