#!/usr/bin/env python3
"""Seed the README demo board (OSS-167).

Builds the lived-in board pictured in README.md on a fresh server:
five columns (Today, Reading list, Projects, Kitchen, Ideas) with a
mix of markdown notes, link lists, todo checklists, and two uploaded
images. Only the Python standard library is used.

Usage:
    go run ./cmd/loom --addr 127.0.0.1:8080 --data /tmp/demo.db --web web &
    python3 scripts/seed-demo.py --base http://127.0.0.1:8080

The script targets the auto-seeded "Home" board on a fresh server and
refuses to run when that board already holds real content, so it never
wipes anyone's data. Image uploads need the SQLite backend (.db file).
"""

import argparse
import io
import json
import mimetypes
import os
import sys
import urllib.request

ASSETS = os.path.join(os.path.dirname(os.path.abspath(__file__)), "seed-demo-assets")

COLUMNS = [
    ("Today", "#4c8dff"),
    ("Reading list", "#9b7ede"),
    ("Projects", "#3fae7a"),
    ("Kitchen", "#e8933c"),
    ("Ideas", "#e06c9f"),
]


def api(base, method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(
        base + path, data=data, method=method,
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(req) as r:
        raw = r.read().decode()
        return r.status, json.loads(raw) if raw.strip() else None


def upload(base, filename):
    path = os.path.join(ASSETS, filename)
    with open(path, "rb") as f:
        blob = f.read()
    ctype, _ = mimetypes.guess_type(path)
    boundary = "seeddemo"
    buf = io.BytesIO()
    buf.write(("--%s\r\n" % boundary).encode())
    buf.write(('Content-Disposition: form-data; name="file"; filename="%s"\r\n' % filename).encode())
    buf.write(("Content-Type: %s\r\n\r\n" % (ctype or "application/octet-stream")).encode())
    buf.write(blob)
    buf.write(("\r\n--%s--\r\n" % boundary).encode())
    req = urllib.request.Request(
        base + "/api/images", data=buf.getvalue(), method="POST",
        headers={"Content-Type": "multipart/form-data; boundary=%s" % boundary},
    )
    with urllib.request.urlopen(req) as r:
        return json.loads(r.read().decode())


def main():
    ap = argparse.ArgumentParser(description="Seed the README demo board.")
    ap.add_argument("--base", default="http://127.0.0.1:8080", help="server base URL")
    args = ap.parse_args()
    base = args.base.rstrip("/")

    _, boards = api(base, "GET", "/api/boards")
    if not boards:
        sys.exit("no boards found; start the server with an empty store first")
    bid = boards[0]["id"]
    _, tree = api(base, "GET", "/api/boards/%s" % bid)
    titles = [c["title"] for c in tree["columns"]]
    if any(t != "Start here" for t in titles):
        sys.exit("board %r already has content %r; refusing to seed" % (tree["board"]["title"], titles))

    for c in tree["columns"]:
        req = urllib.request.Request(base + "/api/columns/%s" % c["id"], method="DELETE")
        urllib.request.urlopen(req).read()
    api(base, "PATCH", "/api/boards/%s" % bid, {"background": "honey"})

    cols = {}
    for title, color in COLUMNS:
        _, c = api(base, "POST", "/api/boards/%s/columns" % bid, {"title": title, "color": color})
        cols[title] = c["id"]

    hike = upload(base, "hike.png")
    recipe = upload(base, "recipe.png")

    cards = [
        (cols["Today"], [
            {"type": "note", "content": "# Friday\n**Dentist** at 9:40 \u2014 *bring the insurance card*. Parking code `4410`."},
            {"type": "todo", "items": [
                {"content": "Confirm dentist appointment", "checked": True},
                {"content": "Water the plants"},
                {"content": "Call mom back"},
            ]},
        ]),
        (cols["Today"], [
            {"type": "link", "url": "https://news.ycombinator.com/", "title": "Hacker News \u2014 morning scan"},
            {"type": "link", "url": "https://www.weather.gov/", "title": "Weather forecast"},
        ]),
        (cols["Reading list"], [
            {"type": "link", "url": "https://www.gutenberg.org/", "title": "Project Gutenberg \u2014 free ebooks"},
            {"type": "link", "url": "https://go.dev/doc/", "title": "Go documentation"},
            {"type": "link", "url": "https://www.ribbonfarm.com/", "title": "Ribbonfarm \u2014 long reads"},
        ]),
        (cols["Reading list"], [
            {"type": "note", "content": "## Reading notes\nFinished **ch. 4** \u2014 the bit on *checklists* applies to Loom. Re-read with `coffee`."},
        ]),
        (cols["Projects"], [
            {"type": "note", "content": "# Shed rebuild\n**This weekend:** posts + roof. Budget `~$400`."},
            {"type": "todo", "items": [
                {"content": "Measure and order lumber", "checked": True},
                {"content": "Rent post-hole auger", "checked": True},
                {"content": "Set posts Saturday"},
                {"content": "Roof panels Sunday"},
            ]},
        ]),
        (cols["Projects"], [
            {"type": "link", "url": "https://go.dev/", "title": "Go \u2014 docs and downloads"},
            {"type": "note", "content": "Loom runs as a *single binary* + SQLite. Deploy note: `docker run -p 8080:8080 -v loom-data:/data loom`."},
        ]),
        (cols["Kitchen"], [
            {"type": "image", "image_url": recipe["url"], "thumb_url": recipe["thumb_url"], "alt": "Skillet dinner"},
            {"type": "note", "content": "## Skillet shakshuka-ish\n**Serves 2.** Onion + pepper, *smoked paprika*, 4 eggs. `20 min` start to finish."},
        ]),
        (cols["Kitchen"], [
            {"type": "todo", "items": [
                {"content": "Eggs"},
                {"content": "Crushed tomatoes"},
                {"content": "Feta", "checked": True},
                {"content": "Bread"},
            ]},
        ]),
        (cols["Ideas"], [
            {"type": "image", "image_url": hike["url"], "thumb_url": hike["thumb_url"], "alt": "Ridge at sunrise"},
            {"type": "note", "content": "## Ridge loop\n**12 km**, *sunrise start*. Trailhead code `P7` \u2014 bring the thermos."},
        ]),
        (cols["Ideas"], [
            {"type": "note", "content": "# Cabin weekend\n## Pack list\n### Don't forget\n**Lantern**, *wool socks*, `firewood`."},
        ]),
    ]
    for col_id, blocks in cards:
        api(base, "POST", "/api/columns/%s/cards" % col_id, {"blocks": blocks})
    print("seeded demo board %r: %d columns, %d cards" % (tree["board"]["title"], len(cols), len(cards)))


if __name__ == "__main__":
    main()
