#!/bin/sh
set -eu

INIT=${1:-}
FIXTURE_DIR=${FIXTURE_DIR:-/fixtures}
if [ "$(id -u)" -ne 0 ]; then
  echo "Installer smoke test must run as root inside a disposable container." >&2
  exit 1
fi
if [ "$INIT" != systemd ] && [ "$INIT" != openrc ]; then
  echo "Usage: $0 systemd|openrc" >&2
  exit 2
fi

MOCKS=$(mktemp -d)
trap 'rm -rf "$MOCKS"' EXIT
export FIXTURE_DIR
cat >"$MOCKS/curl" <<'MOCK'
#!/bin/sh
set -eu
url=
output=
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o) shift; output=${1:-} ;;
    https://*) url=$1 ;;
  esac
  shift
done
file=${url##*/}
if [ -z "$url" ] || [ -z "$output" ] || [ ! -r "$FIXTURE_DIR/$file" ]; then
  echo "Unexpected installer download: $url" >&2
  exit 1
fi
cp "$FIXTURE_DIR/$file" "$output"
MOCK
cat >"$MOCKS/aplay" <<'MOCK'
#!/bin/sh
exit 0
MOCK
cat >"$MOCKS/arecord" <<'MOCK'
#!/bin/sh
exit 0
MOCK
cat >"$MOCKS/piper" <<'MOCK'
#!/bin/sh
exit 0
MOCK
cat >"$MOCKS/getent" <<'MOCK'
#!/bin/sh
exit 2
MOCK
cat >"$MOCKS/systemctl" <<'MOCK'
#!/bin/sh
set -eu
printf '%s\n' "$*" >> /tmp/ultimatewx-systemctl.log
case "$*" in
  "is-active --quiet ultimatewx.service") test -f /tmp/ultimatewx-systemd-active ;;
  "start ultimatewx.service"|"restart ultimatewx.service") touch /tmp/ultimatewx-systemd-active ;;
  "disable --now ultimatewx.service") rm -f /tmp/ultimatewx-systemd-active ;;
esac
MOCK
cat >"$MOCKS/rc-update" <<'MOCK'
#!/bin/sh
printf '%s\n' "$*" >> /tmp/ultimatewx-openrc.log
MOCK
cat >"$MOCKS/rc-service" <<'MOCK'
#!/bin/sh
set -eu
printf '%s\n' "$*" >> /tmp/ultimatewx-openrc.log
case "${2:-}" in
  status) test -f /tmp/ultimatewx-openrc-active ;;
  start|restart) touch /tmp/ultimatewx-openrc-active ;;
  stop) rm -f /tmp/ultimatewx-openrc-active ;;
esac
MOCK
chmod 0755 "$MOCKS"/*
PATH="$MOCKS:$PATH"
export PATH

if [ "$INIT" = systemd ]; then
  mkdir -p /run/systemd/system
else
  mkdir -p /etc/init.d
  if [ ! -x /sbin/openrc-run ]; then
    printf '#!/bin/sh\n' >/sbin/openrc-run
    chmod 0755 /sbin/openrc-run
  fi
fi

sh /scripts/install.sh
test -x /opt/ultimatewx/ultimatewx
/opt/ultimatewx/ultimatewx --help >/dev/null
sh /scripts/install.sh
if [ "$INIT" = systemd ]; then
  test -s /etc/systemd/system/ultimatewx.service
  grep -q 'User=ultimatewx' /etc/systemd/system/ultimatewx.service
  grep -qx 'enable ultimatewx.service' /tmp/ultimatewx-systemctl.log
  grep -qx 'start ultimatewx.service' /tmp/ultimatewx-systemctl.log
  grep -qx 'restart ultimatewx.service' /tmp/ultimatewx-systemctl.log
else
  test -x /etc/init.d/ultimatewx
  grep -q 'supervise-daemon' /etc/init.d/ultimatewx
  grep -q 'add ultimatewx default' /tmp/ultimatewx-openrc.log
  grep -q 'ultimatewx start' /tmp/ultimatewx-openrc.log
  grep -q 'ultimatewx restart' /tmp/ultimatewx-openrc.log
fi

sh /scripts/uninstall.sh
test ! -e /opt/ultimatewx
test -d /var/lib/ultimatewx
sh /scripts/uninstall.sh --purge-data
test ! -e /var/lib/ultimatewx
if id ultimatewx >/dev/null 2>&1; then
  echo "The purge uninstall left the service account behind." >&2
  exit 1
fi
