#!/usr/bin/env python3
"""Normalize known swaggo output defects in generated OpenAPI files."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path


REQUEST_REF = "#/components/schemas/handler.SendNotificationRequestJSON"
ENDPOINT = "/groups/{id}/notifications"


def normalize_json(path: Path) -> None:
    document = json.loads(path.read_text(encoding="utf-8"))
    schema = document["paths"][ENDPOINT]["post"]["requestBody"]["content"][
        "application/json"
    ]["schema"]

    if schema == {"$ref": REQUEST_REF}:
        return

    alternatives = schema.get("oneOf")
    if not isinstance(alternatives, list) or not any(
        alternative.get("$ref") == REQUEST_REF
        for alternative in alternatives
        if isinstance(alternative, dict)
    ):
        raise RuntimeError(f"unexpected notification request schema in {path}")

    schema.clear()
    schema["$ref"] = REQUEST_REF
    serialized = json.dumps(document, ensure_ascii=False, indent=4)
    # Go's encoding/json escapes HTML-sensitive characters. Preserve that
    # representation so normalization changes only the targeted schema.
    serialized = (
        serialized.replace("&", r"\u0026")
        .replace("<", r"\u003c")
        .replace(">", r"\u003e")
    )
    path.write_text(serialized, encoding="utf-8")


def normalize_yaml(path: Path) -> None:
    source = path.read_text(encoding="utf-8")
    desired = f"$ref: '{REQUEST_REF}'"
    if desired in source and not re.search(
        rf"oneOf:\n\s+- type: object\n\s+- \$ref: ['\"]{re.escape(REQUEST_REF)}['\"]",
        source,
    ):
        return

    pattern = re.compile(
        rf"(?P<indent> +)schema:\n"
        rf"(?P=indent)  oneOf:\n"
        rf"(?P=indent)  - type: object\n"
        rf"(?P=indent)  - \$ref: ['\"]{re.escape(REQUEST_REF)}['\"]\n"
        rf"(?:(?P=indent)    (?:description|summary):[^\n]*\n){{2}}"
    )

    def replacement(match: re.Match[str]) -> str:
        indent = match.group("indent")
        return f"{indent}schema:\n{indent}  {desired}\n"

    normalized, count = pattern.subn(replacement, source)
    if count != 1:
        raise RuntimeError(f"unexpected notification request schema in {path}")
    path.write_text(normalized, encoding="utf-8")


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("usage: normalize-openapi-generated.py <openapi.json|openapi.yaml>")
    path = Path(sys.argv[1])
    if path.suffix == ".json":
        normalize_json(path)
    elif path.suffix in {".yaml", ".yml"}:
        normalize_yaml(path)
    else:
        raise SystemExit(f"unsupported OpenAPI file: {path}")
    print(f"✅ 生成OpenAPIを正規化しました: {path}")


if __name__ == "__main__":
    main()
