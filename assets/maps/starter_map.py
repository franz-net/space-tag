#!/usr/bin/env python3
"""Write ship.tmx: a starting Tiled map with today's ship layout.

usage: python3 starter_map.py            (refuses to overwrite an existing ship.tmx)
       python3 starter_map.py --update   (only rewrite the "rooms" layer of ship.tmx,
                                          keeping props and everything else)

Rooms and hallways come from server/gamemap.go (1 game unit = 2 art pixels,
64px tiles), with hallways widened to HALL_TILES. The hull is two tiles thick around them; everything else is
left empty, which the game shows as open space. Edit the result freely in
Tiled, then run autotile.py.
"""
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
TILESETS = os.path.join("..", "tilesets")
ROOT = os.path.normpath(os.path.join(HERE, "..", ".."))
T = 64                 # art pixels per tile
UNIT = 2               # art pixels per game unit
W, H = 69, 41          # tiles (the 2200x1300 ship at double resolution)
HULL = 2               # wall thickness around rooms, in tiles
HALL_TILES = 4         # hallway width in tiles (the tall top wall's face takes ~1.5)
ROOMS = ["cafeteria", "medbay", "navigation", "engine", "storage", "reactor", "hallway"]


def layout():
    src = open(os.path.join(ROOT, "server", "gamemap.go")).read()
    rooms = [(rid, tuple(float(v) for v in b)) for rid, *b in
             re.findall(r'\{ID: "(\w+)".*?Bounds: Rect\{([\d.]+), ([\d.]+), ([\d.]+), ([\d.]+)\}', src)]
    halls = [widen(tuple(float(v) for v in b)) for b in
             re.findall(r'\{Bounds: Rect\{([\d.]+), ([\d.]+), ([\d.]+), ([\d.]+)\}\}', src)]
    return rooms, halls


def widen(r):
    """Make a hallway HALL_TILES wide across its short side, keeping its centre line."""
    x, y, w, h = r
    width = HALL_TILES * T / UNIT
    if w > h:
        return (x, y + h / 2 - width / 2, w, width)
    return (x + w / 2 - width / 2, y, width, h)


def inside(px, py, r):
    x, y, w, h = r
    return x <= px <= x + w and y <= py <= y + h


def main():
    out = os.path.join(HERE, "ship.tmx")
    update = "--update" in sys.argv
    if os.path.exists(out) and not update:
        sys.exit("ship.tmx already exists; use --update to only redo its rooms layer, "
                 "or delete it if you really want a fresh one.")
    rooms, halls = layout()
    tile_units = T / UNIT

    # Corner (vertex) grid: floor inside a room or hallway
    def area(px, py):
        for rid, r in rooms:
            if inside(px, py, r):
                return rid
        for r in halls:
            if inside(px, py, r):
                return "hallway"
        return None

    floor = [[area(x * tile_units, y * tile_units) is not None for x in range(W + 1)] for y in range(H + 1)]
    near = [[any(floor[yy][xx]
                 for yy in range(max(0, y - HULL), min(H, y + HULL) + 1)
                 for xx in range(max(0, x - HULL), min(W, x + HULL) + 1))
             for x in range(W + 1)] for y in range(H + 1)]

    # Tileset gids and their corner -> tile lookup
    firstgid, lookup, tsx_refs, gid = {}, {}, [], 1
    for name in ROOMS:
        path = os.path.join(HERE, TILESETS, name, f"{name}.tsx")
        text = open(path).read()
        firstgid[name] = gid
        lookup[name] = {}
        for tid, wid in re.findall(r'<wangtile tileid="(\d+)" wangid="([\d,]+)"', text):
            v = wid.split(",")
            lookup[name][(v[7], v[1], v[3], v[5])] = int(tid)  # NW NE SE SW
        tsx_refs.append(f' <tileset firstgid="{gid}" source="{TILESETS}/{name}/{name}.tsx"/>')
        gid += int(re.search(r'tilecount="(\d+)"', text).group(1))

    rows = []
    for y in range(H):
        row = []
        for x in range(W):
            vs = [(y, x), (y, x + 1), (y + 1, x + 1), (y + 1, x)]
            if not any(near[vy][vx] for vy, vx in vs):
                row.append(0)  # open space
                continue
            cx, cy = (x + 0.5) * tile_units, (y + 0.5) * tile_units
            name = area(cx, cy) or next((area(vx * tile_units, vy * tile_units) for vy, vx in vs
                                         if area(vx * tile_units, vy * tile_units)), "hallway")
            key = tuple("1" if floor[vy][vx] else "2" for vy, vx in vs)
            row.append(firstgid[name] + lookup[name][key])
        rows.append(",".join(map(str, row)))

    data = ",\n".join(rows)
    if update:
        # Keep everything else in the map; only the rooms layer's tiles change.
        # (Tileset firstgids match because the room tilesets come first.)
        text = open(out).read()
        text = re.sub(r'(<layer [^>]*name="rooms"[^>]*>\s*<data encoding="csv">).*?(</data>)',
                      lambda m: m.group(1) + "\n" + data + "\n" + m.group(2), text, flags=re.S)
        open(out, "w").write(text)
        print(f"updated the rooms layer of {out}; now run autotile.py")
        return
    tmx = f"""<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" tiledversion="1.11.0" orientation="orthogonal" renderorder="right-down" width="{W}" height="{H}" tilewidth="{T}" tileheight="{T}" infinite="0" nextlayerid="2" nextobjectid="1">
{chr(10).join(tsx_refs)}
 <layer id="1" name="rooms" width="{W}" height="{H}">
  <data encoding="csv">
{data}
  </data>
 </layer>
</map>
"""
    open(out, "w").write(tmx)
    print(f"wrote {out}")


if __name__ == "__main__":
    main()
