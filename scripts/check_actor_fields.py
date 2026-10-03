#!/usr/bin/env python3
"""Check live actor field presence without displaying response bodies or values."""
import argparse
import json
import os
import urllib.error
import urllib.parse
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("base_url")
    parser.add_argument("--provider", default="AV-LEAGUE")
    parser.add_argument("--ids", nargs="+", default=["8301", "14005", "36672"])
    args = parser.parse_args()
    headers = {"Accept": "application/json"}
    token = os.environ.get("METATUBE_TOKEN", "")
    if token:
        headers["Authorization"] = "Bearer " + token

    def get(path, query=None):
        url = args.base_url.rstrip("/") + path
        if query:
            url += "?" + urllib.parse.urlencode(query)
        try:
            with urllib.request.urlopen(urllib.request.Request(url, headers=headers), timeout=30) as response:
                return response.status, json.load(response)
        except urllib.error.HTTPError as error:
            return error.code, None
        except (urllib.error.URLError, TimeoutError, ValueError):
            return 0, None

    ok = True
    for actor_id in args.ids:
        path = "/v1/actors/" + urllib.parse.quote(args.provider, safe="") + "/" + urllib.parse.quote(actor_id, safe="")
        status, body = get(path, {"lazy": "false"})
        data = body.get("data") if isinstance(body, dict) else None
        valid = isinstance(data, dict) and all(data.get(key) for key in ("id", "name", "provider", "homepage"))
        result = {"provider": args.provider, "sample": actor_id, "status": status, "valid": bool(valid)}
        if valid:
            result["fields_present"] = {
                key: bool(data.get(key)) and not str(data[key]).startswith("0001-")
                for key in ("birthday", "height", "blood_type", "cup_size", "measurements", "aliases", "images", "debut_date")
            }
            # Reuse the name mechanically; do not display it or any response value.
            for name, query in (("provider_search", {"q": data["name"], "provider": args.provider}),
                                ("all_provider_search", {"q": data["name"]})):
                search_status, search_body = get("/v1/actors/search", query)
                entries = search_body.get("data", []) if isinstance(search_body, dict) else []
                found = isinstance(entries, list) and any(
                    isinstance(entry, dict) and entry.get("provider") == args.provider and entry.get("id") == data["id"]
                    for entry in entries
                )
                result[name] = {"status": search_status, "detail_found": found}
                ok &= search_status == 200 and found
            cached_status, cached = get(path)
            result["cached_fields_match"] = cached_status == 200 and isinstance(cached, dict) and all(
                cached.get("data", {}).get(key) == data.get(key)
                for key in ("birthday", "height", "blood_type", "cup_size", "measurements")
            )
            ok &= result["cached_fields_match"]
        ok &= status == 200 and bool(valid)
        print(json.dumps(result, ensure_ascii=False), flush=True)
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
