#!/usr/bin/env python3
"""Exercise a built server image over HTTP with SQLite and a local PNG fixture.

Usage: python3 scripts/smoke.py metatube:test
Requires a local Docker daemon; creates and removes its own container and volume.
No external metadata providers or credentials are used.
"""
import json
import os
import pathlib
import sqlite3
import struct
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
import uuid
import zlib


def docker(*args):
    return subprocess.check_output(["docker", *args], text=True).strip()


def main():
    image = sys.argv[1] if len(sys.argv) > 1 else "metatube:test"
    name = "metatube-smoke-" + uuid.uuid4().hex[:12]
    volume = name + "-data"
    token = uuid.uuid4().hex
    base = ""

    def http(path, method="GET", headers=None):
        req = urllib.request.Request(base + path, method=method, headers=headers or {})
        try:
            response = urllib.request.urlopen(req, timeout=10)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            return response.status, response.headers, response.read()

    def start():
        nonlocal base
        docker("run", "-d", "--name", name, "-p", "127.0.0.1::8080",
               "--mount", f"type=volume,src={volume},dst=/data",
               "-e", f"TOKEN={token}", image)
        base = "http://" + docker("port", name, "8080/tcp").splitlines()[0]
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            try:
                if http("/readyz")[0] == 200:
                    return
            except (OSError, urllib.error.URLError):
                pass
            time.sleep(0.1)
        raise RuntimeError("container never became ready: " + docker("logs", name))

    def png_chunk(kind, body):
        return (struct.pack(">I", len(body)) + kind + body
                + struct.pack(">I", zlib.crc32(kind + body)))

    docker("volume", "create", volume)
    try:
        with tempfile.TemporaryDirectory(prefix="metatube-smoke-") as temporary:
            tmp = pathlib.Path(temporary)
            # The production Alpine image deliberately has no fixture server.
            source = tmp / "fixture.go"
            source.write_text('package main\nimport ("log"; "net/http"; "os")\n'
                              'func main() { if len(os.Args)>1 { '
                              'r,e:=http.Get("http://127.0.0.1:18081/cover.png"); '
                              'if e!=nil { log.Fatal(e) }; defer r.Body.Close(); '
                              'if r.StatusCode!=200 { log.Fatal(r.Status) }; return }; '
                              'log.Fatal(http.ListenAndServe('
                              '"127.0.0.1:18081", http.FileServer(http.Dir("/tmp")))) }\n')
            server = tmp / "fixture-server"
            subprocess.run(["go", "build", "-o", str(server), str(source)],
                           env={**os.environ, "CGO_ENABLED": "0"}, check=True)
            # Obtain the real schema created by the production binary.
            start()
            assert http("/healthz")[0] == 200
            assert http("/v1/db/version")[0] == 401
            assert http("/v1/db/version", headers={"Authorization": "Bearer wrong"})[0] == 401
            auth = {"Authorization": "Bearer " + token}
            assert http("/v1/db/version", headers=auth)[0] == 200
            assert http("/docs")[0] == 200
            spec = json.loads(http("/openapi.json")[2])
            assert "image/webp" in spec["paths"]["/v1/images/primary/{provider}/{id}"]["get"]["responses"]["200"]["content"]
            docker("stop", name)
            database = tmp / "metadata.db"
            docker("cp", name + ":/data/metadata.db", str(database))
            with sqlite3.connect(database) as db:
                db.execute("""INSERT INTO movie_metadata
                    (id, provider, number, title, homepage, cover_url, genres)
                    VALUES (?, ?, ?, ?, ?, ?, ?)""",
                    ("smoke00001", "FANZA", "TEST-001", "<b>Smoke</b>",
                     "https://example.com/item", "http://127.0.0.1:18081/cover.png",
                     '{"1080P","Drama"}'))
            db.close()  # Checkpoint SQLite WAL before copying the database file.
            docker("cp", str(database), name + ":/data/metadata.db")
            docker("rm", name)
            docker("run", "--rm", "--user", "0", "--entrypoint", "chown",
                   "--mount", f"type=volume,src={volume},dst=/data", image,
                   "10001:10001", "/data/metadata.db")
            rows = b"".join(b"\0" + bytes(v for x in range(64)
                           for v in (x * 4, y * 5, x * y % 256)) for y in range(48))
            fixture = tmp / "cover.png"
            fixture.write_bytes(b"\x89PNG\r\n\x1a\n"
                + png_chunk(b"IHDR", struct.pack(">IIBBBBB", 64, 48, 8, 2, 0, 0, 0))
                + png_chunk(b"IDAT", zlib.compress(rows)) + png_chunk(b"IEND", b""))
            fixture.chmod(0o644)  # The image serves files as unprivileged UID 10001.
            # Recreate twice on the same volume: metadata persists, image cache resets.
            for cycle in range(2):
                start()
                docker("cp", str(fixture), name + ":/tmp/cover.png")
                docker("cp", str(server), name + ":/tmp/fixture-server")
                docker("exec", "-d", name, "/tmp/fixture-server")
                for attempt in range(30):
                    ready = subprocess.run(["docker", "exec", name, "/tmp/fixture-server", "probe"],
                                           capture_output=True, text=True)
                    if ready.returncode == 0:
                        break
                    time.sleep(0.1)
                else:
                    raise RuntimeError("local image fixture did not start: " + ready.stdout + ready.stderr)
                status, _, body = http("/v1/movies/FANZA/smoke00001?lazy=true", headers=auth)
                assert status == 200, (status, body)
                metadata = json.loads(body)["data"]
                assert metadata["title"] == "Smoke" and metadata["genres"] == ["Drama"]
                path = "/v1/images/primary/FANZA/smoke00001?pos=1"
                status, headers, body = http(path)
                assert status == 200, (status, body)
                assert headers["Content-Type"] == "image/webp"
                assert body[:4] == b"RIFF" and body[8:12] == b"WEBP"
                assert headers["X-Cache"] == "MISS"
                for quality in (1, 80, 100):
                    cached = http(path + "&quality=" + str(quality))
                    assert cached[0] == 200 and cached[1]["X-Cache"] == "HIT"
                    assert cached[2] == body
                conditional = http(path, headers={"If-None-Match": headers["ETag"]})
                assert conditional[0] == 304 and not conditional[2]
                head = http(path, method="HEAD")
                assert head[0] == 200 and not head[2]
                assert int(head[1]["Content-Length"]) == len(body)
                assert http(path + "&quality=101")[0] == 400
                assert http(path + "&url=http://127.0.0.1/private")[0] == 400
                print(f"PASS container cycle {cycle + 1}: SQLite persistence, authentication, "
                      f"metadata cleaning, WebP ({len(body)} bytes), cache, ETag, HEAD", flush=True)
                docker("stop", name)
                state = json.loads(docker("inspect", name))[0]["State"]
                assert state["ExitCode"] == 0, "graceful shutdown failed"
                docker("rm", name)
            print("PASS container recreation preserves SQLite data and resets image cache", flush=True)
    finally:
        subprocess.run(["docker", "rm", "-f", name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        subprocess.run(["docker", "volume", "rm", volume], check=True, stdout=subprocess.DEVNULL)


if __name__ == "__main__":
    main()
