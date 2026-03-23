#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"
FRONTEND_DIR="${ROOT_DIR}/frontend"
LOG_DIR="${ROOT_DIR}/.logs"

mkdir -p "${LOG_DIR}"
BACKEND_LOG="${LOG_DIR}/backend-dev.log"
FRONTEND_LOG="${LOG_DIR}/frontend-dev.log"

BACKEND_PORT="${PORTAL_BACKEND_PORT:-8081}"
export PORTAL_LISTEN_ADDR="${PORTAL_LISTEN_ADDR:-:${BACKEND_PORT}}"
if [[ "${PORTAL_LISTEN_ADDR}" =~ ([0-9]{2,5})$ ]]; then
  BACKEND_PORT="${BASH_REMATCH[1]}"
fi

FRONTEND_PORT="${PORTAL_FRONTEND_PORT:-5173}"
HEALTH_URL="${PORTAL_HEALTH_URL:-http://127.0.0.1:${BACKEND_PORT}/api/health}"
FRONTEND_URL="${PORTAL_FRONTEND_URL:-http://127.0.0.1:${FRONTEND_PORT}}"
STARTUP_TIMEOUT="${PORTAL_STARTUP_TIMEOUT_SECONDS:-45}"
export PORTAL_DEV_API_TARGET="${PORTAL_DEV_API_TARGET:-http://127.0.0.1:${BACKEND_PORT}}"

# Local startup does not launch external dependencies.
# Disable periodic usage sync by default to avoid hard dependency on core metrics service.
export PORTAL_USAGE_SYNC_ENABLED="${PORTAL_USAGE_SYNC_ENABLED:-false}"

BACKEND_PID=""
FRONTEND_PID=""

require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "[ERROR] Missing command: ${cmd}"
    exit 1
  fi
}

port_in_use() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1; then
    lsof -iTCP:"${port}" -sTCP:LISTEN -t >/dev/null 2>&1
    return $?
  fi
  if command -v ss >/dev/null 2>&1; then
    ss -ltn "( sport = :${port} )" 2>/dev/null | tail -n +2 | grep -q .
    return $?
  fi
  return 1
}

wait_http_ready() {
  local name="$1"
  local url="$2"
  local timeout="$3"
  local i
  for ((i=1; i<=timeout; i++)); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      echo "[OK] ${name} is ready: ${url}"
      return 0
    fi
    sleep 1
  done
  echo "[ERROR] ${name} startup timeout: ${url}"
  return 1
}

cleanup() {
  set +e
  if [[ -n "${BACKEND_PID}" ]] && kill -0 "${BACKEND_PID}" >/dev/null 2>&1; then
    kill "${BACKEND_PID}" >/dev/null 2>&1
    wait "${BACKEND_PID}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${FRONTEND_PID}" ]] && kill -0 "${FRONTEND_PID}" >/dev/null 2>&1; then
    kill "${FRONTEND_PID}" >/dev/null 2>&1
    wait "${FRONTEND_PID}" >/dev/null 2>&1 || true
  fi
}

trap cleanup EXIT INT TERM

require_cmd go
require_cmd npm
require_cmd curl

if [[ ! -d "${BACKEND_DIR}" ]]; then
  echo "[ERROR] backend directory not found: ${BACKEND_DIR}"
  exit 1
fi
if [[ ! -d "${FRONTEND_DIR}" ]]; then
  echo "[ERROR] frontend directory not found: ${FRONTEND_DIR}"
  exit 1
fi

if port_in_use "${BACKEND_PORT}"; then
  echo "[ERROR] Backend port is in use: ${BACKEND_PORT}"
  echo "        Adjust with PORTAL_BACKEND_PORT / PORTAL_LISTEN_ADDR."
  exit 1
fi
if port_in_use "${FRONTEND_PORT}"; then
  echo "[ERROR] Frontend port is in use: ${FRONTEND_PORT}"
  echo "        Adjust with PORTAL_FRONTEND_PORT."
  exit 1
fi

echo "[INFO] Starting backend..."
: > "${BACKEND_LOG}"
(
  cd "${BACKEND_DIR}"
  go run main.go
) >"${BACKEND_LOG}" 2>&1 &
BACKEND_PID=$!

if ! wait_http_ready "backend" "${HEALTH_URL}" "${STARTUP_TIMEOUT}"; then
  echo "----- backend log (tail) -----"
  tail -n 120 "${BACKEND_LOG}" || true
  exit 1
fi

if [[ ! -d "${FRONTEND_DIR}/node_modules" ]]; then
  if [[ "${NO_NPM_INSTALL:-0}" == "1" ]]; then
    echo "[ERROR] frontend/node_modules not found and NO_NPM_INSTALL=1"
    exit 1
  fi
  echo "[INFO] Installing frontend dependencies..."
  (
    cd "${FRONTEND_DIR}"
    npm install
  )
fi

echo "[INFO] Starting frontend..."
: > "${FRONTEND_LOG}"
(
  cd "${FRONTEND_DIR}"
  npm run dev -- --host 0.0.0.0 --port "${FRONTEND_PORT}"
) >"${FRONTEND_LOG}" 2>&1 &
FRONTEND_PID=$!

if ! wait_http_ready "frontend" "${FRONTEND_URL}" "${STARTUP_TIMEOUT}"; then
  echo "----- frontend log (tail) -----"
  tail -n 120 "${FRONTEND_LOG}" || true
  exit 1
fi

cat <<EOF
[READY] Portal local dev started.
- Backend health: ${HEALTH_URL}
- Frontend:       ${FRONTEND_URL}
- Backend log:    ${BACKEND_LOG}
- Frontend log:   ${FRONTEND_LOG}

Press Ctrl+C to stop both processes.
EOF

while true; do
  if ! kill -0 "${BACKEND_PID}" >/dev/null 2>&1; then
    echo "[ERROR] backend exited unexpectedly. See: ${BACKEND_LOG}"
    exit 1
  fi
  if ! kill -0 "${FRONTEND_PID}" >/dev/null 2>&1; then
    echo "[ERROR] frontend exited unexpectedly. See: ${FRONTEND_LOG}"
    exit 1
  fi
  sleep 1
done
