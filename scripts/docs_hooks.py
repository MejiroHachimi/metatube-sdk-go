"""Publish the canonical OpenAPI contract and redirect former API tutorial URLs."""

import json
from pathlib import Path

from mkdocs.structure.files import File

SPEC = Path(__file__).resolve().parents[1] / "internal/service/openapi.json"


def resolve(spec, obj):
    """Resolve local OpenAPI references; fail the build for broken references."""
    seen = set()
    while "$ref" in obj:
        ref = obj["$ref"]
        if not ref.startswith("#/") or ref in seen:
            raise ValueError(f"Unsupported or cyclic reference: {ref}")
        seen.add(ref)
        obj = spec
        for part in ref[2:].split("/"):
            obj = obj[part.replace("~1", "/").replace("~0", "~")]
    return obj


def validate_refs(spec, node):
    if isinstance(node, dict):
        if "$ref" in node:
            resolve(spec, node)
        for value in node.values():
            validate_refs(spec, value)
    elif isinstance(node, list):
        for value in node:
            validate_refs(spec, value)


def on_files(files, config):
    source = SPEC.read_text(encoding="utf-8")
    spec = json.loads(source)
    validate_refs(spec, spec)
    files.append(File.generated(config, "api/openapi.json", content=source))
    # Keep already shared links useful without maintaining duplicate API pages.
    redirect = '<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta http-equiv="refresh" content="0;url=../"><title>API 参考</title><a href="../">打开 API 参考</a></html>'
    for old_page in ("reference", "arbb014"):
        files.append(File.generated(config, f"api/{old_page}/index.html", content=redirect))
    return files
