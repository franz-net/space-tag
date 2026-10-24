#!/usr/bin/env python3
"""Download a PixelLab top-down tileset and write a Tiled tileset (.tsx) for it.

usage: python3 fetch_tileset.py <name> <pixellab-tileset-id>

Creates assets/tilesets/<name>/<name>.png and <name>.tsx. The .tsx has a
corner-type terrain set built from PixelLab's per-tile corner metadata, so
Tiled's Terrain brush picks the right floor/wall edge pieces automatically.
"""
import json
import os
import sys
import urllib.request

API = "https://api.pixellab.ai/mcp/tilesets"


def fetch(url):
    with urllib.request.urlopen(url) as r:
        return r.read()


def main(name, tileset_id):
    out_dir = os.path.join(os.path.dirname(os.path.abspath(__file__)), name)
    os.makedirs(out_dir, exist_ok=True)

    meta = json.loads(fetch(f"{API}/{tileset_id}/metadata"))
    png = fetch(f"{API}/{tileset_id}/image?inline=true")
    with open(os.path.join(out_dir, f"{name}.png"), "wb") as f:
        f.write(png)

    tiles = meta["tileset_data"]["tiles"]
    size = tiles[0]["bounding_box"]["width"]
    cols = max(t["bounding_box"]["x"] for t in tiles) // size + 1
    rows = max(t["bounding_box"]["y"] for t in tiles) // size + 1

    # Tiled wangid order: top, top-right, right, bottom-right, bottom,
    # bottom-left, left, top-left. Corner sets leave the edges at 0.
    color = {"lower": 1, "upper": 2}
    wangtiles, floor_tile, wall_tile = [], 0, 0
    for t in tiles:
        bb = t["bounding_box"]
        tile_id = (bb["y"] // size) * cols + bb["x"] // size
        c = t["corners"]
        if any(v not in color for v in c.values()):
            continue  # transition-only tiles in 25-tile sheets
        wangtiles.append(
            f'   <wangtile tileid="{tile_id}" wangid="0,{color[c["NE"]]},0,{color[c["SE"]]},0,{color[c["SW"]]},0,{color[c["NW"]]}"/>'
        )
        if set(c.values()) == {"lower"}:
            floor_tile = tile_id
        if set(c.values()) == {"upper"}:
            wall_tile = tile_id

    tsx = f"""<?xml version="1.0" encoding="UTF-8"?>
<tileset version="1.10" tiledversion="1.11.0" name="{name}" tilewidth="{size}" tileheight="{size}" tilecount="{cols * rows}" columns="{cols}">
 <image source="{name}.png" width="{cols * size}" height="{rows * size}"/>
 <wangsets>
  <wangset name="{name.capitalize()} floor and wall" type="corner" tile="-1">
   <wangcolor name="Floor (walkable)" color="#d8dde6" tile="{floor_tile}" probability="1"/>
   <wangcolor name="Wall" color="#2b3340" tile="{wall_tile}" probability="1"/>
{chr(10).join(sorted(wangtiles, key=lambda s: int(s.split('"')[1])))}
  </wangset>
 </wangsets>
</tileset>
"""
    with open(os.path.join(out_dir, f"{name}.tsx"), "w") as f:
        f.write(tsx)
    # Untouched copy: retile.py rebuilds <name>/ from these
    orig_dir = os.path.join(os.path.dirname(out_dir), "_originals")
    os.makedirs(orig_dir, exist_ok=True)
    with open(os.path.join(orig_dir, f"{name}.png"), "wb") as f:
        f.write(png)
    with open(os.path.join(orig_dir, f"{name}.tsx"), "w") as f:
        f.write(tsx)
    print(f"{name}: {len(tiles)} tiles, {size}px -> {out_dir}")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    main(sys.argv[1], sys.argv[2])
