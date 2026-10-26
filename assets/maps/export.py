#!/usr/bin/env python3
"""Export ship.tmx for the game.

usage: python3 export.py [map.tmx]      (default: ship.tmx next to this script)

Run it after autotile.py whenever the map changes. It writes:

  server/shipmap.json     what the server needs, in game units (1 unit = 2
                          art pixels): walls, furniture footprints, rooms,
                          task and sabotage-fix stations, spawn point and
                          waypoints for the bots to wander between.
  client/public/map/      what the client draws: the floor and walls baked
                          into a few big images (window glass left see-through
                          so space shows behind it), one sprite sheet with
                          every piece of furniture, ship.json describing
                          where everything goes, and space/ backdrop art.

Stations come from the furniture on the "props" layer: task-* and fix-*
objects are stations automatically (see STATIONS below). Any other object
becomes one if you give it a custom property "station" in Tiled, e.g.
station = meltdown or station = tap_targets.

The spawn point is a point object named "spawn" on an object layer called
"markers".
"""
import json
import os
import re
import shutil
import sys
import xml.etree.ElementTree as ET
from PIL import Image

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.normpath(os.path.join(HERE, "..", ".."))
OBJECTS = os.path.join(ROOT, "assets", "objects")
SERVER_OUT = os.path.join(ROOT, "server", "shipmap.json")
CLIENT_OUT = os.path.join(ROOT, "client", "public", "map")
GID_MASK = 0x0FFFFFFF
FLIP_X = 0x80000000
UNIT = 2              # art pixels per game unit
CHUNK = 1472          # art pixels per baked image side (keeps textures phone-sized)
PLAYER_RADIUS = 16    # game units, must match server/collision.go
STATION_GAP = 20      # game units between a station's front edge and where you stand

LABELS = {"medbay": "Medbay", "cafeteria": "Cafeteria", "navigation": "Navigation",
          "engine": "Engine", "storage": "Storage", "reactor": "Reactor"}
COLORS = {"medbay": "#2D6A4F", "cafeteria": "#4A5568", "navigation": "#1E40AF",
          "engine": "#92400E", "storage": "#5B21B6", "reactor": "#991B1B"}

# Furniture that is a station without needing a Tiled property
STATIONS = {
    "task-colors": "match_colors",
    "task-simon": "simon_says",
    "task-wires": "connect_wires",
    "task-targets": "tap_targets",
    "fix-comms": "comms_down",
    "fix-lights": "lights_out",
}
TASK_TYPES = {"match_colors", "simon_says", "connect_wires", "tap_targets"}
FIX_TYPES = {"comms_down", "lights_out", "meltdown"}


def main(tmx_path):
    tmx_dir = os.path.dirname(os.path.abspath(tmx_path))
    root = ET.parse(tmx_path).getroot()
    W, H, T = int(root.get("width")), int(root.get("height")), int(root.get("tilewidth"))

    tilesets = []
    for ts in root.findall("tileset"):
        path = os.path.normpath(os.path.join(tmx_dir, ts.get("source")))
        tilesets.append((int(ts.get("firstgid")), os.path.splitext(os.path.basename(path))[0], path))
    tilesets.sort()

    def owner(gid):
        found = None
        for t in tilesets:
            if gid >= t[0]:
                found = t
        return found

    def layer(name):
        el = next(l for l in root.findall("layer") if l.get("name") == name)
        vals = [int(v) for v in el.find("data").text.replace("\n", "").split(",") if v.strip()]
        return [vals[y * W:(y + 1) * W] for y in range(H)]

    def group(name):
        g = next((g for g in root.findall("objectgroup") if g.get("name") == name), None)
        return [] if g is None else g.findall("object")

    # ── Gameplay ──────────────────────────────────────────────────────
    walls, obstacles = [], []
    for o in group("walls"):
        r = [round(float(o.get(k)) / UNIT, 2) for k in ("x", "y", "width", "height")]
        (obstacles if o.get("type") == "prop" else walls).append(r)

    def walkable(x, y, radius=PLAYER_RADIUS):
        if x - radius < 0 or y - radius < 0 or x + radius > W * T / UNIT or y + radius > H * T / UNIT:
            return False
        for rx, ry, rw, rh in walls + obstacles:
            cx, cy = max(rx, min(x, rx + rw)), max(ry, min(y, ry + rh))
            if (x - cx) ** 2 + (y - cy) ** 2 <= radius ** 2:
                return False
        return True

    def nearest_walkable(x, y, margin=8):
        """The walkable spot closest to (x, y), with a little room to spare."""
        for step in range(0, 400, 4):
            for dx, dy in ((0, 0), (0, step), (0, -step), (step, 0), (-step, 0),
                           (step, step), (-step, step), (step, -step), (-step, -step)):
                if walkable(x + dx, y + dy, PLAYER_RADIUS + margin):
                    return [round(x + dx, 1), round(y + dy, 1)]
        return None

    # Rooms: the cells painted with each room's tileset (hallway is not a room)
    cells = {}
    for y, row in enumerate(layer("rooms")):
        for x, gid in enumerate(row):
            if gid & GID_MASK:
                cells.setdefault(owner(gid & GID_MASK)[1], []).append((x, y))
    tile = T / UNIT

    def bounds(cs):
        xs, ys = [c[0] for c in cs], [c[1] for c in cs]
        return {"x": min(xs) * tile, "y": min(ys) * tile,
                "w": (max(xs) + 1 - min(xs)) * tile, "h": (max(ys) + 1 - min(ys)) * tile}

    rooms = [{"id": n, "label": LABELS.get(n, n.capitalize()), "bounds": bounds(cs), "color": COLORS.get(n, "#4A5568")}
             for n, cs in sorted(cells.items()) if n != "hallway"]
    # Rooms' painted cells include their hull, so shrink to the floor the walls leave
    for r in rooms:
        inner = [(x, y) for x, y in cells[r["id"]] if walkable((x + 0.5) * tile, (y + 0.5) * tile, 1)]
        if inner:
            r["bounds"] = bounds(inner)

    def room_at(x, y):
        for r in rooms:
            b = r["bounds"]
            if b["x"] <= x <= b["x"] + b["w"] and b["y"] <= y <= b["y"] + b["h"]:
                return r["id"]
        return ""

    # Hallways: connected runs of walkable hallway cells
    hall = {c for c in cells.get("hallway", []) if walkable((c[0] + 0.5) * tile, (c[1] + 0.5) * tile, 1)}
    hallways = []
    while hall:
        stack, comp = [hall.pop()], []
        while stack:
            c = stack.pop()
            comp.append(c)
            for n in ((c[0] + 1, c[1]), (c[0] - 1, c[1]), (c[0], c[1] + 1), (c[0], c[1] - 1)):
                if n in hall:
                    hall.remove(n)
                    stack.append(n)
        hallways.append({"bounds": bounds(comp)})
    hallways.sort(key=lambda h: (h["bounds"]["y"], h["bounds"]["x"]))

    # Furniture
    tsx = open(os.path.join(OBJECTS, "props.tsx")).read()
    props_first = next(t[0] for t in tilesets if t[1] == "props")
    names = {props_first + int(i): os.path.splitext(s)[0]
             for i, s in re.findall(r'<tile id="(\d+)">\s*<image source="([^"]+)"', tsx)}
    info = json.load(open(os.path.join(OBJECTS, "objects.json")))
    props = []
    for o in group("props"):
        gid = int(o.get("gid"))
        name = names[gid & GID_MASK]
        p = {o2.get("name"): o2.get("value") for o2 in o.findall("properties/property")}
        props.append({"name": name, "x": float(o.get("x")), "y": float(o.get("y")),
                      "w": float(o.get("width")), "h": float(o.get("height")),
                      "flip": bool(gid & FLIP_X), "kind": info.get(name, {}).get("kind", "free"),
                      "station": p.get("station") or STATIONS.get(name)})

    tasks, fixes = [], []
    for p in props:
        if not p["station"]:
            continue
        # Stand just in front of the furniture
        spot = nearest_walkable(p["x"] / UNIT, p["y"] / UNIT + PLAYER_RADIUS + STATION_GAP / 2)
        if spot is None:
            sys.exit(f"no floor in front of {p['name']} at {p['x']},{p['y']}")
        room = room_at(*spot)
        if p["station"] in TASK_TYPES:
            tasks.append({"type": p["station"], "room": room, "x": spot[0], "y": spot[1]})
        elif p["station"] in FIX_TYPES:
            fixes.append({"sabotage": p["station"], "id": room, "room": room, "x": spot[0], "y": spot[1]})
        else:
            sys.exit(f"{p['name']}: unknown station {p['station']!r}")
    tasks.sort(key=lambda t: (t["y"], t["x"]))

    spawn = next(([float(o.get("x")) / UNIT, float(o.get("y")) / UNIT]
                  for o in group("markers") if o.get("name") == "spawn"), None)
    if spawn is None:
        sys.exit('add a point named "spawn" on an object layer called "markers"')

    # Places for bots to wander to: the middle and corners of each room, the middle of each hallway
    # (the first one listed for each room is its middle)
    waypoints = []
    for r in rooms:
        b = r["bounds"]
        for fx, fy in ((0.5, 0.55), (0.25, 0.35), (0.75, 0.35), (0.25, 0.8), (0.75, 0.8)):
            wp = nearest_walkable(b["x"] + b["w"] * fx, b["y"] + b["h"] * fy, 16)
            if wp and all(w["x"] != wp[0] or w["y"] != wp[1] for w in waypoints):
                waypoints.append({"x": wp[0], "y": wp[1], "room": r["id"]})
    for h in hallways:
        b = h["bounds"]
        wp = nearest_walkable(b["x"] + b["w"] / 2, b["y"] + b["h"] / 2, 16)
        if wp:
            waypoints.append({"x": wp[0], "y": wp[1], "room": ""})

    gameplay = {
        "width": W * tile, "height": H * tile,
        "rooms": rooms, "hallways": hallways,
        "walls": walls, "obstacles": obstacles,
        "spawn": spawn, "tasks": tasks, "fixes": fixes,
        "waypoints": waypoints,
    }
    with open(SERVER_OUT, "w") as f:
        json.dump(gameplay, f, indent=1)

    # ── Art ───────────────────────────────────────────────────────────
    os.makedirs(CLIENT_OUT, exist_ok=True)
    full = Image.new("RGBA", (W * T, H * T), (0, 0, 0, 0))
    sheets = {}
    for y, row in enumerate(layer("tiles")):
        for x, gid in enumerate(row):
            gid &= GID_MASK
            if not gid:
                continue
            first, name, path = owner(gid)
            if path not in sheets:
                src = re.search(r'<image source="([^"]+)"', open(path).read()).group(1)
                sheets[path] = Image.open(os.path.join(os.path.dirname(path), src)).convert("RGBA")
            sheet = sheets[path]
            cols = sheet.width // T
            lid = gid - first
            full.alpha_composite(sheet.crop(((lid % cols) * T, (lid // cols) * T, (lid % cols + 1) * T, (lid // cols + 1) * T)),
                                 (x * T, y * T))

    images = {}

    def picture(p):
        if p["name"] not in images:
            images[p["name"]] = Image.open(os.path.join(OBJECTS, p["name"] + ".png")).convert("RGBA")
        img = images[p["name"]]
        return img.transpose(Image.FLIP_LEFT_RIGHT) if p["flip"] else img

    # Wall-hung things never stand in front of anyone: bake them into the walls.
    # Glass replaces the wall behind it, so the space backdrop shows through.
    sprites = []
    for p in props:
        if p["kind"] != "mounted":
            sprites.append(p)
            continue
        img = picture(p)
        x0, y0 = int(round(p["x"] - img.width / 2)), int(round(p["y"] - img.height))
        layer_img = Image.new("RGBA", full.size, (0, 0, 0, 0))
        layer_img.paste(img, (x0, y0))
        full.alpha_composite(layer_img)
        if p["name"].startswith("window"):
            px, ux = img.load(), full.load()
            for yy in range(img.height):
                for xx in range(img.width):
                    if 0 < px[xx, yy][3] < 255:
                        ux[x0 + xx, y0 + yy] = px[xx, yy]

    for old in os.listdir(CLIENT_OUT):
        if old.startswith("chunk-") or old in ("props.png", "ship.json"):
            os.remove(os.path.join(CLIENT_OUT, old))
    chunks = []
    for cy in range(0, full.height, CHUNK):
        for cx in range(0, full.width, CHUNK):
            part = full.crop((cx, cy, min(cx + CHUNK, full.width), min(cy + CHUNK, full.height)))
            if not part.getbbox():
                continue
            name = f"chunk-{cx // CHUNK}-{cy // CHUNK}.png"
            part.save(os.path.join(CLIENT_OUT, name), optimize=True)
            chunks.append({"src": name, "x": cx / UNIT, "y": cy / UNIT,
                           "w": part.width / UNIT, "h": part.height / UNIT})

    # One sheet with every piece of free-standing furniture (shelf packing)
    used = sorted({p["name"] for p in sprites}, key=lambda n: -images.setdefault(
        n, Image.open(os.path.join(OBJECTS, n + ".png")).convert("RGBA")).height)
    sheet_w, frames, x, y, row_h = 1024, {}, 0, 0, 0
    for n in used:
        img = images[n]
        if x + img.width > sheet_w:
            x, y, row_h = 0, y + row_h + 1, 0
        frames[n] = [x, y, img.width, img.height]
        x, row_h = x + img.width + 1, max(row_h, img.height)
    atlas = Image.new("RGBA", (sheet_w, y + row_h), (0, 0, 0, 0))
    for n, (fx, fy, _, _) in frames.items():
        atlas.paste(images[n], (fx, fy))
    atlas.save(os.path.join(CLIENT_OUT, "props.png"), optimize=True)

    art = {
        "width": W * tile, "height": H * tile, "artScale": UNIT,
        "chunks": chunks,
        "atlas": "props.png", "frames": frames,
        # x, y: bottom-centre in game units (also the front edge, used for depth sorting)
        "props": [{"name": p["name"], "x": round(p["x"] / UNIT, 2), "y": round(p["y"] / UNIT, 2),
                   "flip": p["flip"]} for p in sprites],
    }
    with open(os.path.join(CLIENT_OUT, "ship.json"), "w") as f:
        json.dump(art, f, separators=(",", ":"))

    space_out = os.path.join(CLIENT_OUT, "space")
    os.makedirs(space_out, exist_ok=True)
    for f in os.listdir(os.path.join(OBJECTS, "space")):
        shutil.copy(os.path.join(OBJECTS, "space", f), space_out)

    print(f"server/shipmap.json: {len(rooms)} rooms, {len(hallways)} hallways, {len(walls)} walls, "
          f"{len(obstacles)} obstacles, {len(tasks)} tasks, {len(fixes)} fix stations, {len(waypoints)} waypoints")
    print(f"client/public/map: {len(chunks)} chunks, {len(sprites)} furniture sprites ({len(frames)} kinds)")


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else os.path.join(HERE, "ship.tmx"))
