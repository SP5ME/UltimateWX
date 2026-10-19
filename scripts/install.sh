#!/bin/sh
set -eu

REPOSITORY=${ULTIMATEWX_REPOSITORY:-SP5ME/UltimateWX}
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
  armv7l|armv7) ARCH=armv7 ;;
  *) echo "Unsupported architecture: $(uname -m). Supported: x86_64, aarch64 and armv7l." >&2; exit 1 ;;
esac
if command -v systemctl >/dev/null 2>&1; then INIT=systemd
elif command -v rc-update >/dev/null 2>&1; then INIT=openrc
else echo "Neither systemd nor OpenRC was detected." >&2; exit 1; fi
for cmd in curl tar sha256sum; do command -v "$cmd" >/dev/null 2>&1 || { echo "Missing required command: $cmd" >&2; exit 1; }; done

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT HUP INT TERM
ASSET="ultimatewx_linux_$ARCH.tar.gz"
VERSION=${ULTIMATEWX_VERSION:-}
if [ -z "$VERSION" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPOSITORY/releases?per_page=1" \
    | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | sed -n '1p')
fi
if [ -z "$VERSION" ]; then
  echo "No GitHub release was found for $REPOSITORY." >&2
  exit 1
fi
BASE="https://github.com/$REPOSITORY/releases/download/$VERSION"
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

install_piper() {
  PIPER_VERSION=1.8.0
  if command -v piper >/dev/null 2>&1; then
    echo "Piper is already available: $(command -v piper)."
    return
  fi

  case "$DISTRO:$ARCH" in
    alpine:*)
      echo "Piper was not installed automatically: the ARMv7 Piper build uses glibc and is not compatible with Alpine's musl libc."
      return
      ;;
    *:armv7)
      PIPER_ASSET=piper_armv7.tar.gz
      if ! curl -fsSL "$BASE/$PIPER_ASSET" -o "$TMP/$PIPER_ASSET"; then
        echo "Piper was not installed automatically: this release has no experimental ARMv7 build."
        return
      fi
      grep "  $PIPER_ASSET\$" "$TMP/SHA256SUMS" > "$TMP/PIPER-SHA256SUMS"
      if [ ! -s "$TMP/PIPER-SHA256SUMS" ] || ! (cd "$TMP" && sha256sum -c PIPER-SHA256SUMS); then
        echo "Piper ARMv7 package checksum verification failed; it was not installed."
        return
      fi
      if ! tar -tzf "$TMP/$PIPER_ASSET" | grep -q '^piper/piper$'; then
        echo "Piper ARMv7 package has an unexpected layout; it was not installed."
        return
      fi
      tar -xzf "$TMP/$PIPER_ASSET" -C /opt/ultimatewx
      if ! ln -sfn /opt/ultimatewx/piper/piper /usr/local/bin/piper; then
        echo "Piper was installed under /opt/ultimatewx/piper, but its command could not be added to /usr/local/bin."
        return
      fi
      echo "Installed experimental Piper 1.2.0 ARMv7 build from source. Test speech performance on this Raspberry Pi model."
      return
      ;;
  esac

  if ! command -v apt-get >/dev/null 2>&1; then
    echo "Piper was not installed automatically: apt-get is unavailable."
    return
  fi
  if ! apt-get update || ! DEBIAN_FRONTEND=noninteractive apt-get install -y python3 python3-venv; then
    echo "Piper was not installed automatically: Python 3 and venv could not be installed."
    return
  fi
  if ! python3 -c 'import sys; raise SystemExit(sys.version_info < (3, 9))'; then
    echo "Piper was not installed automatically: Python 3.9 or newer is required."
    return
  fi

  PIPER_VENV=/opt/ultimatewx/piper-venv
  if ! python3 -m venv "$PIPER_VENV" || ! "$PIPER_VENV/bin/python" -m pip install --disable-pip-version-check --only-binary=:all: "piper-tts==$PIPER_VERSION"; then
    echo "Piper installation failed; UltimateWX will still start, but speech output needs Piper installed separately."
    return
  fi
  if ! ln -sfn "$PIPER_VENV/bin/piper" /usr/local/bin/piper; then
    echo "Piper was installed at $PIPER_VENV/bin/piper, but its command could not be added to /usr/local/bin."
    return
  fi
  echo "Installed Piper TTS $PIPER_VERSION."
}

install_piper

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
