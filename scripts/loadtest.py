#!/usr/bin/env python3
"""HTTP load checks for the test-only Docker target, locally or on Heroku.

Usage: python3 scripts/loadtest.py https://your-test-app.herokuapp.com
Run against a disposable app built with --target loadtest, never user data.
"""
import concurrent.futures
import json
import sys
import time
import urllib.error
import urllib.request

base = sys.argv[1].rstrip("/")


def fetch(path, timeout=35):
    start = time.monotonic()
    try:
        response = urllib.request.urlopen(base + path, timeout=timeout)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        body = response.read()
        if response.status == 200 and "/images/" in path:
            assert response.headers["Content-Type"] == "image/webp"
            assert body[:4] == b"RIFF" and body[8:12] == b"WEBP"
        return {"status": response.status, "seconds": round(time.monotonic()-start, 3),
                "bytes": len(body), "cache": response.headers.get("X-Cache")}


deadline = time.monotonic() + 60
while time.monotonic() < deadline:
    try:
        if fetch("/readyz", timeout=2)["status"] == 200:
            break
    except (OSError, urllib.error.URLError):
        pass
    time.sleep(1)
else:
    raise RuntimeError("test app did not become ready")

for label, kinds, expected in [
    ("small-4", ["small"]*4, {200}),
    ("medium-4", ["medium"]*4, {200}),
    ("large-4", ["large"]*4, {200}),
    ("small-compressed-large-png", ["png"]*2, {200}),
    ("mixed-12", ["large","small","medium"]*4, {200,504}),
    ("bounded-overload-32", ["large"]*32, {200,503,504}),
]:
    # Each phase uses a different crop to avoid hits from a previous phase.
    position = {"small-4": 0, "medium-4": 0, "large-4": 0,
                "small-compressed-large-png": 0, "mixed-12": 0.5,
                "bounded-overload-32": 1}[label]
    paths = [f"/v1/images/primary/FANZA/{kind}{i:05d}?pos={position}" for i,kind in enumerate(kinds)]
    start = time.monotonic()
    with concurrent.futures.ThreadPoolExecutor(max_workers=len(paths)) as pool:
        results = list(pool.map(fetch, paths))
    assert all(item["status"] in expected for item in results), results
    assert any(item["status"] == 200 for item in results), results
    assert fetch("/readyz")["status"] == 200
    print(json.dumps({"case":label,"seconds":round(time.monotonic()-start,3),
                      "results":results}), flush=True)
