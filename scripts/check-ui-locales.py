#!/usr/bin/env python3
"""Ensure every built-in UI locale covers static translation references."""
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
HTML = (ROOT / "internal/app/index.html").read_text(encoding="utf-8")
LANGUAGES = ("en", "pl", "de", "fr", "es", "it")


def object_body(source: str, opening_brace: int) -> str:
    depth = 0
    quote = None
    escaped = False
    for index in range(opening_brace, len(source)):
        char = source[index]
        if quote:
            if escaped:
                escaped = False
            elif char == "\\":
                escaped = True
            elif char == quote:
                quote = None
            continue
        if char in "'\"`":
            quote = char
        elif char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                return source[opening_brace + 1:index]
    raise ValueError("Unclosed translation object")


def keys_in(body: str) -> set[str]:
    return set(re.findall(r"(?<![\w$])([A-Za-z_$][\w$]*):", body))


translations = {language: set() for language in LANGUAGES}
base_match = re.search(r"const\s+L\s*=\s*\{", HTML)
if not base_match:
    raise SystemExit("Could not find the built-in locale dictionary")
base_start = base_match.end() - 1
base = object_body(HTML, base_start)
for match in re.finditer(r"(?<![\w$])(en|pl|de|fr|es|it)\s*:\s*\{", base):
    translations[match.group(1)].update(keys_in(object_body(base, match.end() - 1)))

for match in re.finditer(r"Object\.assign\(L\.(en|pl|de|fr|es|it),\s*\{", HTML):
    translations[match.group(1)].update(keys_in(object_body(HTML, match.end() - 1)))

required = set(re.findall(r'data-i(?:-aria)?="([A-Za-z_$][\w$]*)"', HTML))
required.update(match[1] for match in re.findall(r"\btr\(\s*(['\"])([A-Za-z_$][\w$]*)\1\s*\)", HTML))
required.update(re.findall(r"['\"](aprsReason[A-Za-z_$][\w$]*)['\"]", HTML))
errors = []
for language in LANGUAGES:
    missing = sorted(required - translations[language])
    if missing:
        errors.append(f"{language}: missing UI translations: {', '.join(missing)}")
if errors:
    raise SystemExit("\n".join(errors))
print(f"UI translation coverage is complete ({len(required)} static keys, {len(LANGUAGES)} languages).")
