"""Generate API reference and downloadable contract from the service's OpenAPI file."""

import html
import json
from pathlib import Path

from mkdocs.structure.files import File

ROOT = Path(__file__).resolve().parents[1]
SPEC = ROOT / "internal/service/openapi.json"


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


def cell(value):
    return html.escape(str(value)).replace("|", "&#124;").replace("\n", "<br>")


def code(value):
    return "```json\n" + json.dumps(value, ensure_ascii=False, indent=2) + "\n```\n"


def validate_refs(spec, node):
    if isinstance(node, dict):
        if "$ref" in node:
            resolve(spec, node)
        for value in node.values():
            validate_refs(spec, value)
    elif isinstance(node, list):
        for value in node:
            validate_refs(spec, value)


def reference(spec):
    lines = [
        "# 完整接口与模型\n",
        "本页在构建时从服务的 `internal/service/openapi.json` 自动生成。"
        "修改接口时维护该契约，不直接编辑生成页。\n",
        "[下载 OpenAPI 3.0 JSON](openapi.json) · [调用与认证指南](index.md)\n",
        "这里是静态参考。交互式调用请打开你部署的服务地址下的 `/docs`；"
        "JSON 中的相对服务器地址 `/` 指服务根地址，不是 GitHub Pages。\n",
    ]
    for path, item in spec["paths"].items():
        for method, operation in item.items():
            if method not in {"get", "head", "post", "put", "patch", "delete", "options"}:
                continue
            lines += [f"## {method.upper()} `{path}`\n", operation.get("summary", "") + "\n"]
            if operation.get("description"):
                lines.append(operation["description"] + "\n")
            security = operation.get("security", spec.get("security", []))
            lines.append("认证：配置 `TOKEN` 后需要 Bearer Token。\n" if security else "认证：公开接口。\n")
            params = item.get("parameters", []) + operation.get("parameters", [])
            if params:
                lines += ["### 参数\n", "| 参数 | 位置 | 必填 | 类型 | 默认值 | 说明 |", "| --- | --- | --- | --- | --- | --- |"]
                for raw in params:
                    p = resolve(spec, raw)
                    schema = resolve(spec, p.get("schema", {}))
                    default = json.dumps(schema["default"], ensure_ascii=False) if "default" in schema else "—"
                    description = p.get("description", "")
                    constraints = {k: v for k, v in schema.items() if k not in {"type", "default", "description"}}
                    if constraints:
                        description += " " + json.dumps(constraints, ensure_ascii=False)
                    values = [p["name"], p["in"], "是" if p.get("required") else "否", schema.get("type", "—"), default, description]
                    lines.append("| " + " | ".join(cell(v) for v in values) + " |")
                lines.append("")
            lines.append("### 响应\n")
            for status, raw in operation["responses"].items():
                response = resolve(spec, raw)
                lines.append(f"#### {status}\n\n{response.get('description', '')}\n")
                for media, content in response.get("content", {}).items():
                    lines.append(f"Content-Type: `{media}`\n")
                    if "schema" in content:
                        lines.append(code(content["schema"]))
                    if "example" in content:
                        lines += ["示例：\n", code(content["example"])]
                if response.get("headers"):
                    lines += ["响应头：\n", code(response["headers"])]
    lines.append("## 数据模型\n\n响应中的 `#/components/schemas/名称` 对应以下模型；完整引用可在下载的契约中解析。\n")
    for name, schema in spec.get("components", {}).get("schemas", {}).items():
        lines += [f"### {name}\n", code(schema)]
    return "\n".join(lines)


def on_files(files, config):
    source = SPEC.read_text(encoding="utf-8")
    spec = json.loads(source)
    validate_refs(spec, spec)
    files.append(File.generated(config, "api/reference.md", content=reference(spec)))
    files.append(File.generated(config, "api/openapi.json", content=source))
    return files
