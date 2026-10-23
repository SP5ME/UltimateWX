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
version_at_least() {
  awk -v current="$1" -v minimum="$2" 'BEGIN {
    split(current, installed, ".")
    split(minimum, required, ".")
    for (i = 1; i <= 2; i++) {
      have = installed[i] + 0
      need = required[i] + 0
      if (have > need) exit 0
      if (have < need) exit 1
    }
    exit 0
  }'
}
case "$DISTRO" in
  alpine) MINIMUM_VERSION=3.22 ;;
  ubuntu) MINIMUM_VERSION=24.04 ;;
  raspberry-pi-os-or-debian) MINIMUM_VERSION=12 ;;
esac
if ! version_at_least "${VERSION_ID:-0}" "$MINIMUM_VERSION"; then
  echo "Unsupported ${PRETTY_NAME:-Linux} version. Minimum supported versions: Raspberry Pi OS/Debian 12, Ubuntu 24.04 LTS, Alpine Linux 3.22." >&2
  exit 1
fi
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  armv7l|armv7) ARCH=armv7 ;;
  *) echo "Unsupported architecture: $(uname -m). Supported: x86_64, aarch64 and armv7l." >&2; exit 1 ;;
esac
if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then INIT=systemd
elif command -v rc-update >/dev/null 2>&1 && command -v rc-service >/dev/null 2>&1 && [ -x /sbin/openrc-run ]; then INIT=openrc
else echo "Neither systemd nor OpenRC was detected." >&2; exit 1; fi
for cmd in curl tar sha256sum awk sed; do command -v "$cmd" >/dev/null 2>&1 || { echo "Missing required command: $cmd" >&2; exit 1; }; done

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
trap 'exit 1' HUP INT TERM
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
verify_checksum() {
  asset=$1
  manifest=$2
  matches=$(awk -v asset="$asset" '$2 == asset { count++ } END { print count + 0 }' "$manifest")
  if [ "$matches" -ne 1 ]; then
    echo "Checksum manifest must contain exactly one entry for $asset." >&2
    return 1
  fi
  awk -v asset="$asset" '$2 == asset { print }' "$manifest" > "$TMP/checksum-entry"
  (cd "$TMP" && sha256sum -c checksum-entry)
}
verify_checksum "$ASSET" "$TMP/SHA256SUMS"
tar -xzf "$TMP/$ASSET" -C "$TMP" ultimatewx

if ! id ultimatewx >/dev/null 2>&1; then
  if [ "$DISTRO" = alpine ]; then
    if ! grep -q '^ultimatewx:' /etc/group; then addgroup -S ultimatewx; fi
    adduser -S -D -H -G ultimatewx -h /var/lib/ultimatewx -s /sbin/nologin ultimatewx
  else
    useradd --system --home-dir /var/lib/ultimatewx --no-create-home --shell /usr/sbin/nologin ultimatewx
  fi
elif [ "$DISTRO" = alpine ] && ! grep -q '^ultimatewx:' /etc/group; then
  addgroup -S ultimatewx
fi
install -d -o ultimatewx -g ultimatewx -m 0750 /opt/ultimatewx /var/lib/ultimatewx
install -o ultimatewx -g ultimatewx -m 0750 "$TMP/ultimatewx" /opt/ultimatewx/ultimatewx
install_alsa_tools() {
  if command -v aplay >/dev/null 2>&1 && command -v arecord >/dev/null 2>&1; then
    echo "ALSA playback and capture tools are already available."
    return
  fi

  if [ "$DISTRO" = alpine ]; then
    if command -v apk >/dev/null 2>&1 && apk add --no-cache alsa-utils; then
      echo "Installed ALSA playback and capture tools."
    else
      echo "ALSA tools were not installed automatically; audio playback and input testing may be unavailable."
    fi
  elif command -v apt-get >/dev/null 2>&1; then
    if apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y alsa-utils; then
      echo "Installed ALSA playback and capture tools."
    else
      echo "ALSA tools were not installed automatically; audio playback and input testing may be unavailable."
    fi
  else
    echo "ALSA tools were not installed automatically; no supported package manager was found."
  fi
}

install_piper() {
  PIPER_VERSION=1.8.0
  PIPER_DIR=/opt/ultimatewx/piper

  # Keep the app-managed engine in one predictable directory. A system Piper
  # remains untouched and can still be used as a fallback by the application.
  if [ -x "$PIPER_DIR/piper" ]; then
    echo "UltimateWX Piper is already installed under $PIPER_DIR."
    return
  fi
  if [ -x "$PIPER_DIR/bin/piper" ] && "$PIPER_DIR/bin/python" -m pip show piper-tts >/dev/null 2>&1; then
    echo "UltimateWX Piper is already installed under $PIPER_DIR."
    return
  fi

  case "$DISTRO:$ARCH" in
    alpine:*)
      echo "Piper was not installed automatically on Alpine (musl); install a Piper build compatible with musl libc and set its path in the application."
      return
      ;;
    *:armv7)
      PIPER_ASSET=piper_armv7.tar.gz
      if ! curl -fsSL "$BASE/$PIPER_ASSET" -o "$TMP/$PIPER_ASSET"; then
        echo "Piper was not installed automatically: this release has no experimental ARMv7 build."
        return
      fi
      if ! verify_checksum "$PIPER_ASSET" "$TMP/SHA256SUMS"; then
        echo "Piper ARMv7 package checksum verification failed; it was not installed."
        return
      fi
      if ! tar -tzf "$TMP/$PIPER_ASSET" | grep -q '^piper/piper$'; then
        echo "Piper ARMv7 package has an unexpected layout; it was not installed."
        return
      fi
      tar -xzf "$TMP/$PIPER_ASSET" -C /opt/ultimatewx
      if [ ! -x "$PIPER_DIR/piper" ]; then
        echo "Piper ARMv7 package was extracted, but its executable is missing or not executable."
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

  if ! install -d -o root -g root -m 0755 "$PIPER_DIR" || ! python3 -m venv "$PIPER_DIR"; then
    echo "Piper installation failed under $PIPER_DIR; UltimateWX will still start, but speech output needs Piper installed separately."
    return
  fi
  if [ -x "$PIPER_DIR/bin/piper" ]; then
    if "$PIPER_DIR/bin/python" -m pip install --disable-pip-version-check --only-binary=:all: --force-reinstall "piper-tts==$PIPER_VERSION"; then PIPER_INSTALL_OK=yes
    else PIPER_INSTALL_OK=no; fi
  else
    if "$PIPER_DIR/bin/python" -m pip install --disable-pip-version-check --only-binary=:all: "piper-tts==$PIPER_VERSION"; then PIPER_INSTALL_OK=yes
    else PIPER_INSTALL_OK=no; fi
  fi
  if [ "$PIPER_INSTALL_OK" != yes ] || [ ! -x "$PIPER_DIR/bin/piper" ]; then
    echo "Piper installation failed under $PIPER_DIR; UltimateWX will still start, but speech output needs Piper installed separately."
    return
  fi
  echo "Installed Piper TTS $PIPER_VERSION under $PIPER_DIR."
}

install_alsa_tools
for group in audio i2c; do
  if getent group "$group" >/dev/null 2>&1; then
    if [ "$DISTRO" = alpine ]; then addgroup ultimatewx "$group" || true
    elif command -v usermod >/dev/null 2>&1; then usermod -a -G "$group" ultimatewx; fi
  fi
done
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
  systemctl enable ultimatewx.service
  if systemctl is-active --quiet ultimatewx.service; then
    systemctl restart ultimatewx.service
  else
    systemctl start ultimatewx.service
  fi
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
  if rc-service ultimatewx status >/dev/null 2>&1; then
    rc-service ultimatewx restart
  else
    rc-service ultimatewx start
  fi
fi

echo "UltimateWX installed for $DISTRO ($ARCH, $INIT)."
echo "Open http://<device-ip>:8080. Read the one-time setup token with: sudo cat /var/lib/ultimatewx/setup-token"
