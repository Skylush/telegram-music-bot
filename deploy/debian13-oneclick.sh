#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   curl -fsSL <script-url> | sudo bash -s -- <repo-url>
# or:
#   sudo bash deploy/debian13-oneclick.sh <repo-url>

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root"
  exit 1
fi

REPO_URL="${1:-}"
SOURCE_DIR="/opt/music-bot-src"

echo "[1/7] Installing dependencies"
apt update
apt install -y git golang make ca-certificates

echo "[2/7] Preparing source code"
if [[ -f "./go.mod" && -f "./cmd/bot/main.go" ]]; then
  SOURCE_DIR="$(pwd)"
  echo "Using current directory as source: ${SOURCE_DIR}"
else
  if [[ -z "${REPO_URL}" ]]; then
    echo "Repository URL is required when not running inside project root"
    echo "Example: sudo bash deploy/debian13-oneclick.sh https://github.com/you/music-bot.git"
    exit 1
  fi

  if [[ -d "${SOURCE_DIR}/.git" ]]; then
    echo "Repository exists, pulling latest code"
    git -C "${SOURCE_DIR}" pull --ff-only
  else
    echo "Cloning repository into ${SOURCE_DIR}"
    git clone "${REPO_URL}" "${SOURCE_DIR}"
  fi
fi

echo "[3/7] Building binary"
cd "${SOURCE_DIR}"
go mod tidy
go test ./...
go build -o music-bot ./cmd/bot

echo "[4/7] Ensuring deploy assets"
if [[ ! -f "./deploy/debian13-bootstrap.sh" ]]; then
  echo "Missing deploy/debian13-bootstrap.sh"
  exit 1
fi

if [[ ! -x "./deploy/debian13-bootstrap.sh" ]]; then
  chmod +x ./deploy/debian13-bootstrap.sh
fi

echo "[5/7] Running bootstrap"
./deploy/debian13-bootstrap.sh

echo "[6/7] Final hints"
echo "Edit config if needed: /etc/music-bot/music-bot.env"
echo "Restart service: systemctl restart music-bot.service"

echo "[7/7] Done"