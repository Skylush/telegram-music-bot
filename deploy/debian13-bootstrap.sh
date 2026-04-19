#!/usr/bin/env bash
set -euo pipefail

APP_NAME="music-bot"
APP_USER="musicbot"
APP_GROUP="musicbot"
APP_DIR="/opt/music-bot"
ENV_DIR="/etc/music-bot"
DATA_DIR="/var/lib/music-bot/downloads"
SERVICE_FILE="/etc/systemd/system/music-bot.service"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root"
  exit 1
fi

echo "[1/6] Creating system user and group"
if ! id -u "${APP_USER}" >/dev/null 2>&1; then
  useradd --system --home "${APP_DIR}" --shell /usr/sbin/nologin "${APP_USER}"
fi

echo "[2/6] Creating directories"
install -d -m 0755 -o "${APP_USER}" -g "${APP_GROUP}" "${APP_DIR}"
install -d -m 0755 -o root -g root "${ENV_DIR}"
install -d -m 0755 -o "${APP_USER}" -g "${APP_GROUP}" "${DATA_DIR}"

echo "[3/6] Installing binary"
if [[ ! -f ./music-bot ]]; then
  echo "Binary ./music-bot not found in current directory"
  echo "Build first with: go build -o music-bot ./cmd/bot"
  exit 1
fi
install -m 0755 -o root -g root ./music-bot "${APP_DIR}/music-bot"

echo "[4/6] Installing service template"
if [[ ! -f ./deploy/systemd/music-bot.service ]]; then
  echo "Missing deploy/systemd/music-bot.service"
  exit 1
fi
install -m 0644 -o root -g root ./deploy/systemd/music-bot.service "${SERVICE_FILE}"

echo "[5/6] Writing environment file"
if [[ ! -f "${ENV_DIR}/music-bot.env" ]]; then
  cat > "${ENV_DIR}/music-bot.env" <<EOF
TELEGRAM_BOT_TOKEN=replace_with_your_bot_token
DOWNLOAD_DIR=${DATA_DIR}
MAX_RESULTS=5
HTTP_TIMEOUT_SECONDS=20
HTTP_MAX_RETRIES=2
LOG_LEVEL=info
EOF
  chmod 0640 "${ENV_DIR}/music-bot.env"
fi

echo "[6/6] Reloading systemd and enabling service"
systemctl daemon-reload
systemctl enable --now music-bot.service
systemctl status --no-pager music-bot.service || true

echo "Done. Edit ${ENV_DIR}/music-bot.env if needed, then restart:"
echo "  systemctl restart music-bot.service"