#!/usr/bin/env python3
"""Check required localized manuals and release translation approvals."""
import argparse
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
LANGUAGES = ("en", "pl", "de", "fr", "es", "it")

parser = argparse.ArgumentParser()
parser.add_argument("--release", action="store_true", help="require human-approved translations")
args = parser.parse_args()
errors = []
for language in LANGUAGES:
    manual = ROOT / "docs" / language / "manual.md"
    if not manual.is_file():
        errors.append(f"missing {manual.relative_to(ROOT)}")
    elif len(manual.read_text(encoding="utf-8").strip()) < 500:
        errors.append(f"manual is unexpectedly short: {manual.relative_to(ROOT)}")
status_path = ROOT / "docs" / "translation-status.json"
try:
    status = json.loads(status_path.read_text(encoding="utf-8"))
except Exception as exc:
    status = {}
    errors.append(f"cannot read translation status: {exc}")
if args.release:
    for language in LANGUAGES:
        if status.get(language) != "reviewed":
            errors.append(f"{language}: translation must be human-reviewed before PDF release")
if errors:
    raise SystemExit("\n".join(errors))
print("Documentation structure is complete." + (" All translations are approved." if args.release else ""))
