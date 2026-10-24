#!/usr/bin/env python3
"""Turn the rooms painted in Tiled into tall walls + wall collision.

usage: python3 autotile.py [map.tmx]      (default: ship.tmx next to this script)

You paint the ship on a tile layer named "rooms" with the simple 16-tile
room tilesets (Tiled's Terrain brush: Floor and Wall). Leave cells empty
where the ship should show open space. This script then (re)writes two
layers in the same map, leaving all your other layers alone:

  "tiles"  the tall-wall version the game draws: top walls get a front
           face. Tall-wall tiles depend on their neighbours, which Tiled's
           terrain brush can't do, so they're picked here by PixelLab's
           pattern_4x4 rules.
  "walls"  rectangles where players can't walk: walls, wall faces and the
           footprints of the furniture on your "props" object layer.

It also lines up furniture on the "props" layer (see objects.json):
objects that stand against a wall get their back snapped onto the nearest
wall, and windows/screens get hung on the top wall's face. Place them
roughly; the script makes them exact.

Run it after every edit to "rooms" or "props" (it's quick), then reload the
map in Tiled.
"""
import json
import os
import re
import sys
import xml.etree.ElementTree as ET

HERE = os.path.dirname(os.path.abspath(__file__))
TILESETS = os.path.normpath(os.path.join(HERE, "..", "tilesets"))
OBJECTS = os.path.normpath(os.path.join(HERE, "..", "objects"))
GID_MASK = 0x0FFFFFFF  # strip Tiled's flip flags
LOWER, UPPER, FACE = 0, 1, 2

# Where walls meet the floor inside a 64px tall-wall tile (measured from the
# cafeteria-tall sheet; every room shares its walls).
TOP_WALL_FLOOR = 50   # below the top edge of the cell under a wall face
FACE_TOP = 28         # wall face starts this far into the cell above
LEFT_WALL_FLOOR = 39  # into the cell whose left corners are wall
RIGHT_WALL_FLOOR = 26  # into the cell whose right corners are wall
SNAP = 48             # how far (px) a prop may be from a wall to get snapped


def load_tilesets(tmx_dir, root):
    """firstgid -> (name, tsx root) for every external tileset in the map."""
    out = []
    for ts in root.findall("tileset"):
        src = ts.get("source")
        if not src:
            continue
        path = os.path.normpath(os.path.join(tmx_dir, src))
        name = os.path.splitext(os.path.basename(path))[0]
        out.append((int(ts.get("firstgid")), name, path))
    return sorted(out)


def tileset_for(gid, tilesets):
    owner = None
    for first, name, path in tilesets:
        if gid >= first:
            owner = (first, name, path)
    return owner


def wang_corners(tsx_path):
    """local tile id -> (NW, NE, SE, SW) as LOWER/UPPER from a corner terrain set."""
    out = {}
    text = open(tsx_path).read()
    for tid, wid in re.findall(r'<wangtile tileid="(\d+)" wangid="([\d,]+)"', text):
        v = [int(x) for x in wid.split(",")]
        to = lambda c: LOWER if c == 1 else UPPER
        out[int(tid)] = (to(v[7]), to(v[1]), to(v[3]), to(v[5]))
    return out


def read_layer(layer, w, h):
    data = layer.find("data")
    if data.get("encoding") != "csv":
        sys.exit('Set the map to CSV layer format in Tiled (Map > Map Properties > Tile Layer Format: CSV).')
    vals = [int(x) for x in data.text.replace("\n", "").split(",") if x.strip()]
    return [vals[y * w:(y + 1) * w] for y in range(h)]


def write_layer(root, name, rows, w, h, after):
    layer = next((l for l in root.findall("layer") if l.get("name") == name), None)
    if layer is None:
        layer = ET.Element("layer", {"name": name})
        root.insert(list(root).index(after) + 1, layer)
    layer.set("width", str(w))
    layer.set("height", str(h))
    layer.set("id", layer.get("id") or next_id(root, "nextlayerid"))
    for child in list(layer):
        layer.remove(child)
    data = ET.SubElement(layer, "data", {"encoding": "csv"})
    data.text = "\n" + ",\n".join(",".join(str(g) for g in row) for row in rows) + "\n"
    return layer


def next_id(root, attr):
    n = int(root.get(attr, "1"))
    root.set(attr, str(n + 1))
    return str(n)


def main(tmx_path):
    tmx_dir = os.path.dirname(os.path.abspath(tmx_path))
    tree = ET.parse(tmx_path)
    root = tree.getroot()
    W, H = int(root.get("width")), int(root.get("height"))
    T = int(root.get("tilewidth"))

    rooms_layer = next((l for l in root.findall("layer") if l.get("name") == "rooms"), None)
    if rooms_layer is None:
        sys.exit('No tile layer named "rooms" in the map.')
    cells = read_layer(rooms_layer, W, H)
    tilesets = load_tilesets(tmx_dir, root)

    # Which room each cell belongs to, and the terrain at every cell corner
    corners_cache = {}
    room = [[None] * W for _ in range(H)]
    vert = [[None] * (W + 1) for _ in range(H + 1)]  # None = open space
    for y in range(H):
        for x in range(W):
            gid = cells[y][x] & GID_MASK
            if not gid:
                continue
            first, name, path = tileset_for(gid, tilesets)
            if name.endswith("-tall"):
                continue
            if path not in corners_cache:
                corners_cache[path] = wang_corners(path)
            c = corners_cache[path].get(gid - first)
            if c is None:
                continue
            room[y][x] = name
            for (dy, dx), v in zip(((0, 0), (0, 1), (1, 1), (1, 0)), c):
                # Floor wins if neighbouring cells disagree
                if vert[y + dy][x + dx] is None or v == LOWER:
                    vert[y + dy][x + dx] = v

    # A floor corner right below a wall corner is where the wall's front face goes
    def terrain(y, x):
        if 0 <= y <= H and 0 <= x <= W and vert[y][x] is not None:
            return vert[y][x]
        return UPPER

    t = [[terrain(y, x) for x in range(W + 1)] for y in range(H + 1)]
    for y in range(H + 1):
        for x in range(W + 1):
            if t[y][x] == LOWER and terrain(y - 1, x) == UPPER:
                t[y][x] = FACE

    def at(y, x):
        return t[y][x] if 0 <= y <= H and 0 <= x <= W else UPPER

    # Tall tilesets: make sure each room's is referenced by the map
    tall = {}
    for name in sorted({r for row in room for r in row if r}):
        tname = f"{name}-tall"
        existing = next((ts for ts in tilesets if ts[1] == tname), None)
        if existing is None:
            last = max((int(ts.get("firstgid")) + tile_count(os.path.join(tmx_dir, ts.get("source"))))
                       for ts in root.findall("tileset"))
            src = os.path.relpath(os.path.join(TILESETS, name, f"{tname}.tsx"), tmx_dir)
            el = ET.Element("tileset", {"firstgid": str(last), "source": src})
            root.insert(list(root).index(root.findall("tileset")[-1]) + 1, el)
            existing = (last, tname, os.path.join(tmx_dir, src))
            tilesets.append(existing)
        meta = json.load(open(os.path.join(TILESETS, name, f"{tname}.json")))
        cols = int(re.search(r'columns="(\d+)"', open(existing[2]).read()).group(1))
        pats = []
        for tl in meta["tiles"]:
            p = [tl["pattern_4x4"][f"row_{i}"] for i in range(4)]
            pats.append(((tl["y"] // T) * cols + tl["x"] // T, p))
        tall[name] = (existing[0], pats)

    out = [[0] * W for _ in range(H)]
    for y in range(H):
        for x in range(W):
            name = room[y][x]
            if not name:
                continue
            first, pats = tall[name]
            local = [[at(y - 1 + r, x - 1 + c) for c in range(4)] for r in range(4)]
            best, best_score = None, -1
            for tid, p in pats:
                score = 0
                for r in range(4):
                    for c in range(4):
                        if p[r][c] == 255:
                            continue
                        if p[r][c] != local[r][c]:
                            score = -1
                            break
                        score += 1
                    if score < 0:
                        break
                if score > best_score:
                    best, best_score = tid, score
            if best is not None:
                out[y][x] = first + best

    tiles_layer = write_layer(root, "tiles", out, W, H, rooms_layer)
    rooms_layer.set("visible", "0")  # you edit "rooms", the game shows "tiles"

    footprints, snapped = place_props(root, tmx_dir, tilesets, at, W, H, T)

    # Collision on a half-tile grid: a quarter-cell is solid if its corner is wall or wall face
    solid = [[False] * (2 * W) for _ in range(2 * H)]
    for y in range(H):
        for x in range(W):
            if not room[y][x]:
                continue
            for qy in (0, 1):
                for qx in (0, 1):
                    if at(y + qy, x + qx) != LOWER:
                        solid[2 * y + qy][2 * x + qx] = True
    rects = merge_rects(solid)

    group = next((g for g in root.findall("objectgroup") if g.get("name") == "walls"), None)
    if group is None:
        group = ET.Element("objectgroup", {"id": next_id(root, "nextlayerid"), "name": "walls", "visible": "0"})
        root.insert(list(root).index(tiles_layer) + 1, group)
    for child in list(group):
        group.remove(child)
    half = T // 2
    for (x, y, w, h) in rects:
        ET.SubElement(group, "object", {"id": next_id(root, "nextobjectid"),
                                        "x": str(x * half), "y": str(y * half),
                                        "width": str(w * half), "height": str(h * half)})
    for name, (x, y, w, h) in footprints:
        ET.SubElement(group, "object", {"id": next_id(root, "nextobjectid"), "name": name, "type": "prop",
                                        "x": fmt(x), "y": fmt(y), "width": fmt(w), "height": fmt(h)})

    ET.indent(tree, space=" ")
    tree.write(tmx_path, encoding="UTF-8", xml_declaration=True)
    print(f"{os.path.basename(tmx_path)}: {sum(1 for r in out for g in r if g)} tall-wall tiles, "
          f"{len(rects)} wall rectangles, {len(footprints)} prop footprints, {snapped} props lined up")


def fmt(v):
    return str(int(v)) if float(v).is_integer() else f"{v:.2f}"


def place_props(root, tmx_dir, tilesets, at, W, H, T):
    """Snap the "props" layer's furniture to walls and return their footprints.

    Tile objects use the props tileset's bottom alignment: (x, y) is the
    bottom-centre of the picture, which is also the front of its footprint.
    """
    group = next((g for g in root.findall("objectgroup") if g.get("name") == "props"), None)
    if group is None:
        return [], 0
    info = json.load(open(os.path.join(OBJECTS, "objects.json")))
    names = {}  # gid -> object name
    for first, name, path in tilesets:
        text = open(path).read() if os.path.exists(path) else ""
        for tid, src in re.findall(r'<tile id="(\d+)">\s*<image source="([^"]+)"', text):
            names[first + int(tid)] = os.path.splitext(os.path.basename(src))[0]

    footprints, snapped = [], 0
    for obj in group.findall("object"):
        gid = int(obj.get("gid", "0")) & GID_MASK
        name = names.get(gid)
        meta = info.get(name or "")
        if not meta:
            continue
        x, y = float(obj.get("x")), float(obj.get("y"))
        w, h = float(obj.get("width")), float(obj.get("height"))
        depth, kind = meta["depth"], meta["kind"]
        old = (x, y)

        if kind == "mounted":
            # Hang it on the face of the nearest top wall above: bottom just above the stripe
            v = nearest_face_row(at, x, y - h / 2, W, H, T)
            if v is not None:
                face_top, face_bottom = (v - 1) * T + FACE_TOP, v * T + TOP_WALL_FLOOR - 14
                y = face_bottom if h > face_bottom - face_top else (face_top + face_bottom + h) / 2
        elif kind == "wall":
            back = y - depth
            v = top_wall_row(at, x, back, W, H, T)
            if v is not None and abs(back - (v * T + TOP_WALL_FLOOR)) <= SNAP:
                y = v * T + TOP_WALL_FLOOR + depth
            # Side walls: line up its left/right edge
            row = int((y - depth / 2) // T)
            for u in range(W):
                left_line = u * T + LEFT_WALL_FLOOR
                if at(row, u) != LOWER and at(row, u + 1) == LOWER and abs((x - w / 2) - left_line) <= SNAP / 2:
                    x = left_line + w / 2
                right_line = u * T + RIGHT_WALL_FLOOR
                if at(row, u) == LOWER and at(row, u + 1) != LOWER and abs((x + w / 2) - right_line) <= SNAP / 2:
                    x = right_line - w / 2

        if (x, y) != old:
            obj.set("x", fmt(x))
            obj.set("y", fmt(y))
            snapped += 1
        if kind in ("wall", "free") and depth:
            footprints.append((name, (x - w / 2, y - depth, w, depth)))
    return footprints, snapped


def nearest_face_row(at, x, y, W, H, T):
    """Vertex row of the wall face nearest to art pixel (x, y), above or below."""
    vx = [int(x // T), int(x // T) + 1]
    rows = [v for v in range(H + 1) if any(0 <= c <= W and at(v, c) == FACE for c in vx)]
    rows = [v for v in rows if abs(v * T - y) <= 3 * T]
    return min(rows, key=lambda v: abs(v * T - y)) if rows else None


def top_wall_row(at, x, y, W, H, T):
    """Vertex row of the wall face closest above art pixel (x, y), or None."""
    vx = [int(x // T), int(x // T) + 1]
    best = None
    for v in range(H + 1):
        if any(0 <= c <= W and at(v, c) == FACE for c in vx):
            line = v * T + TOP_WALL_FLOOR
            if line <= y + SNAP and (best is None or line > best * T + TOP_WALL_FLOOR):
                best = v
    return best


def tile_count(tsx_path):
    return int(re.search(r'tilecount="(\d+)"', open(tsx_path).read()).group(1))


def merge_rects(grid):
    """Greedy merge of solid cells into rectangles (x, y, w, h) in grid units."""
    h, w = len(grid), len(grid[0])
    used = [[False] * w for _ in range(h)]
    rects = []
    for y in range(h):
        for x in range(w):
            if not grid[y][x] or used[y][x]:
                continue
            rw = 1
            while x + rw < w and grid[y][x + rw] and not used[y][x + rw]:
                rw += 1
            rh = 1
            while y + rh < h and all(grid[y + rh][x + i] and not used[y + rh][x + i] for i in range(rw)):
                rh += 1
            for yy in range(y, y + rh):
                for xx in range(x, x + rw):
                    used[yy][xx] = True
            rects.append((x, y, rw, rh))
    return rects


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else os.path.join(HERE, "ship.tmx"))
