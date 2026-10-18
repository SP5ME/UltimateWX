#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
OUT=${1:-"$ROOT/dist/docs"}
mkdir -p "$OUT"
if [ "${REQUIRE_TRANSLATION_REVIEW:-0}" = "1" ]; then
  python3 "$ROOT/scripts/check-docs.py" --release
else
  python3 "$ROOT/scripts/check-docs.py"
fi
for lang in en pl de fr es it; do
  pandoc "$ROOT/docs/$lang/manual.md" \
    --from markdown --pdf-engine=xelatex \
    --metadata lang="$lang" --metadata title="UltimateWX Manual" \
    -V mainfont="Noto Sans" -V geometry:margin=22mm \
    -o "$OUT/UltimateWX-Manual-$lang.pdf"
done
