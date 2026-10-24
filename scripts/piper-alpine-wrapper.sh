#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
export LD_LIBRARY_PATH="$ROOT/usr/lib:$ROOT/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export PYTHONHOME="$ROOT/usr"
export PYTHONNOUSERSITE=1

exec "$ROOT/usr/bin/python3" "$ROOT/usr/bin/piper" "$@"
