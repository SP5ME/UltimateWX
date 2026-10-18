#!/bin/sh
set -eu

# Replace this value when publishing under the actual GitHub owner.
REPOSITORY=${ULTIMATEWX_REPOSITORY:-OWNER/UltimateWX}
if [ "$REPOSITORY" = "OWNER/UltimateWX" ]; then
  echo "Set ULTIMATEWX_REPOSITORY to the published OWNER/UltimateWX repository." >&2
  exit 1
fi
if [ "$(id -u)" -ne 0 ]; then
  echo "Run as root, for example: curl -fsSL <installer-url> | sudo sh" >&2
  exit 1
fi

if [ ! -r /etc/os-release ]; then echo "Cannot identify Linux distribution." >&2; exit 1; fi
. /etc/os-release
case "${ID:-}:${ID_LIKE:-}" in
  alpine:*|*:alpine*) DISTRO=alpine ;;
  ubuntu:*|*:ubuntu*) DISTRO=ubuntu ;;
  raspbian:*|*:raspbian*|debian:*|*:debian*) DISTRO=raspberry-pi-os-or-debian ;;
  *) echo "Unsupported Linux distribution: ${PRETTY_NAME:-unknown}. Supported: Raspberry Pi OS, Ubuntu, Alpine." >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "Unsupported architecture: $(uname -m). Supported: x86_64 and aarch64." >&2; exit 1 ;;
esac
if command -v systemctl >/dev/null 2>&1; then INIT=systemd
elif command -v rc-update >/dev/null 2>&1; then INIT=openrc
else echo "Neither systemd nor OpenRC was detected." >&2; exit 1; fi
for cmd in curl tar sha256sum; do command -v "$cmd" >/dev/null 2>&1 || { echo "Missing required command: $cmd" >&2; exit 1; }; done

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT HUP INT TERM
ASSET="ultimatewx_linux_$ARCH.tar.gz"
BASE="https://github.com/$REPOSITORY/releases/latest/download"
curl -fsSL "$BASE/$ASSET" -o "$TMP/$ASSET"
curl -fsSL "$BASE/SHA256SUMS" -o "$TMP/SHA256SUMS"
(cd "$TMP" && grep "  $ASSET\$" SHA256SUMS | sha256sum -c -)
tar -xzf "$TMP/$ASSET" -C "$TMP" ultimatewx

if ! id ultimatewx >/dev/null 2>&1; then
  if [ "$DISTRO" = alpine ]; then
    adduser -S -D -H -h /var/lib/ultimatewx -s /sbin/nologin ultimatewx
  else
    useradd --system --home-dir /var/lib/ultimatewx --no-create-home --shell /usr/sbin/nologin ultimatewx
  fi
fi
install -d -o ultimatewx -g ultimatewx -m 0750 /opt/ultimatewx /var/lib/ultimatewx
install -o ultimatewx -g ultimatewx -m 0750 "$TMP/ultimatewx" /opt/ultimatewx/ultimatewx
for group in audio i2c; do
  if getent group "$group" >/dev/null 2>&1; then
    if [ "$DISTRO" = alpine ]; then addgroup ultimatewx "$group" || true
    elif command -v usermod >/dev/null 2>&1; then usermod -a -G "$group" ultimatewx; fi
  fi
done

if [ "$INIT" = systemd ]; then
  cat >/etc/systemd/system/ultimatewx.service <<'UNIT'
[Unit]
Description=UltimateWX weather announcer
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=ultimatewx
Group=ultimatewx
WorkingDirectory=/var/lib/ultimatewx
ExecStart=/opt/ultimatewx/ultimatewx --config /var/lib/ultimatewx/config.json --data-dir /var/lib/ultimatewx --listen 0.0.0.0:8080
Restart=always
RestartSec=3
UMask=0077
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/ultimatewx /var/lib/ultimatewx

[Install]
WantedBy=multi-user.target
UNIT
  systemctl daemon-reload
  systemctl enable --now ultimatewx.service
else
  cat >/etc/init.d/ultimatewx <<'UNIT'
#!/sbin/openrc-run
description="UltimateWX weather announcer"
command="/opt/ultimatewx/ultimatewx"
command_args="--config /var/lib/ultimatewx/config.json --data-dir /var/lib/ultimatewx --listen 0.0.0.0:8080"
command_user="ultimatewx:ultimatewx"
command_background="yes"
pidfile="/run/ultimatewx.pid"
supervisor="supervise-daemon"
respawn_delay=3
respawn_max=0
depend() { need net; }
UNIT
  chmod 0755 /etc/init.d/ultimatewx
  rc-update add ultimatewx default
  rc-service ultimatewx start
fi

echo "UltimateWX installed for $DISTRO ($ARCH, $INIT)."
echo "Open http://<device-ip>:8080 and set the first-run password."
