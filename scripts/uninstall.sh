#!/bin/sh
set -eu

if [ "$(id -u)" -ne 0 ]; then
  echo "Run as root, for example: sudo sh scripts/uninstall.sh" >&2
  exit 1
fi

PURGE_DATA=0
case "${1:-}" in
  "") ;;
  --purge-data) PURGE_DATA=1 ;;
  *) echo "Usage: $0 [--purge-data]" >&2; exit 2 ;;
esac
if [ "$#" -gt 1 ]; then
  echo "Usage: $0 [--purge-data]" >&2
  exit 2
fi

if [ -f /etc/systemd/system/ultimatewx.service ]; then
  if command -v systemctl >/dev/null 2>&1; then
    systemctl disable --now ultimatewx.service >/dev/null 2>&1 || true
  fi
  rm -f /etc/systemd/system/ultimatewx.service
  if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload >/dev/null 2>&1 || true
  fi
fi

if [ -f /etc/init.d/ultimatewx ]; then
  if command -v rc-service >/dev/null 2>&1; then
    rc-service ultimatewx stop >/dev/null 2>&1 || true
  fi
  if command -v rc-update >/dev/null 2>&1; then
    rc-update del ultimatewx default >/dev/null 2>&1 || true
  fi
  rm -f /etc/init.d/ultimatewx
fi

if [ -L /usr/local/bin/piper ]; then
  PIPER_TARGET=$(readlink -f /usr/local/bin/piper 2>/dev/null || true)
  case "$PIPER_TARGET" in
    /opt/ultimatewx/*) rm -f /usr/local/bin/piper ;;
  esac
fi
rm -rf /opt/ultimatewx

if [ "$PURGE_DATA" -eq 1 ]; then
  rm -rf /var/lib/ultimatewx
  if id ultimatewx >/dev/null 2>&1; then
    if command -v userdel >/dev/null 2>&1; then
      userdel ultimatewx
    elif command -v deluser >/dev/null 2>&1; then
      deluser ultimatewx
    fi
  fi
  echo "UltimateWX and its data were removed."
else
  echo "UltimateWX was removed. Data and the service account remain in /var/lib/ultimatewx."
  echo "To remove them too, run: sudo sh scripts/uninstall.sh --purge-data"
fi
