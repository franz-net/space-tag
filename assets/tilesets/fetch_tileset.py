#!/usr/bin/env python3
"""Download a PixelLab top-down tileset into _originals/.

usage: python3 fetch_tileset.py <name> <pixellab-tileset-id>

Writes _originals/<name>.png (the sheet), <name>.json (per-tile corners,
pattern_4x4 and bounding boxes) and <name>.tsx (a Tiled tileset; 16-tile
sets get a corner terrain set so Tiled's Terrain brush works). Run
retile.py afterwards to build the room tilesets the game uses.

Name tall-wall (25-tile) sets <room>-tall, e.g. cafeteria-tall.
"""
import json
import os
import sys
import urllib.request

API = "https://api.pixellab.ai/mcp/tilesets"
HERE = os.path.dirname(os.path.abspath(__file__))


def fetch(url):
    with urllib.request.urlopen(url) as r:
        return r.read()


def tsx_for(name, tiles, size, cols, rows):
    """Tiled tileset. Only 16-tile (two-terrain) sets get a terrain set:
    tall-wall sets pick tiles by their neighbours too, which Tiled's corner
    sets can't express (autotile.py handles those)."""
    color = {"lower": 1, "upper": 2}
    wangtiles, floor_tile, wall_tile = [], 0, 0
    two_terrain = len(tiles) == 16
    for t in tiles if two_terrain else []:
        bb = t["bounding_box"]
        tile_id = (bb["y"] // size) * cols + bb["x"] // size
        c = t["corners"]
        wangtiles.append((tile_id, f'   <wangtile tileid="{tile_id}" wangid="0,{color[c["NE"]]},0,{color[c["SE"]]},0,{color[c["SW"]]},0,{color[c["NW"]]}"/>'))
        if set(c.values()) == {"lower"}:
            floor_tile = tile_id
        if set(c.values()) == {"upper"}:
            wall_tile = tile_id
    wangsets = ""
    if two_terrain:
        wangsets = f"""
 <wangsets>
  <wangset name="{name.capitalize()} floor and wall" type="corner" tile="-1">
   <wangcolor name="Floor (walkable)" color="#d8dde6" tile="{floor_tile}" probability="1"/>
   <wangcolor name="Wall" color="#2b3340" tile="{wall_tile}" probability="1"/>
{chr(10).join(line for _, line in sorted(wangtiles))}
  </wangset>
 </wangsets>"""
    return f"""<?xml version="1.0" encoding="UTF-8"?>
<tileset version="1.10" tiledversion="1.11.0" name="{name}" tilewidth="{size}" tileheight="{size}" tilecount="{cols * rows}" columns="{cols}">
 <image source="{name}.png" width="{cols * size}" height="{rows * size}"/>{wangsets}
</tileset>
"""


def main(name, tileset_id):
    out_dir = os.path.join(HERE, "_originals")
    os.makedirs(out_dir, exist_ok=True)

    meta = json.loads(fetch(f"{API}/{tileset_id}/metadata"))
    png = fetch(f"{API}/{tileset_id}/image?inline=true")
    tiles = meta["tileset_data"]["tiles"]
    size = tiles[0]["bounding_box"]["width"]
    dims = meta["tileset_image"]["dimensions"]
    cols, rows = dims["width"] // size, dims["height"] // size

    with open(os.path.join(out_dir, f"{name}.png"), "wb") as f:
        f.write(png)
    with open(os.path.join(out_dir, f"{name}.json"), "w") as f:
        json.dump({
            "pixellab_id": tileset_id,
            "tile_size": size,
            "tiles": [{"x": t["bounding_box"]["x"], "y": t["bounding_box"]["y"],
                       "corners": t["corners"], "pattern_4x4": t["pattern_4x4"]} for t in tiles],
        }, f, indent=1)
    with open(os.path.join(out_dir, f"{name}.tsx"), "w") as f:
        f.write(tsx_for(name, tiles, size, cols, rows))
    print(f"{name}: {len(tiles)} tiles, {size}px -> {out_dir}")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    main(sys.argv[1], sys.argv[2])
