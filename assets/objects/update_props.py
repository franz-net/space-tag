#!/usr/bin/env python3
"""Refresh props.tsx (the Tiled image collection) after adding or redrawing objects.

usage: python3 update_props.py

Existing objects keep their tile ids, so maps that already use them stay
correct; new PNGs are appended. Image sizes are refreshed in props.tsx and
on every placed object in ../maps/*.tmx. Then run ../maps/autotile.py to
re-snap them.
"""
import glob
import os
import re
from PIL import Image

HERE = os.path.dirname(os.path.abspath(__file__))
TSX = os.path.join(HERE, "props.tsx")


def main():
    old = {}
    if os.path.exists(TSX):
        old = {src: int(tid) for tid, src in re.findall(r'<tile id="(\d+)">\s*<image source="([^"]+)"', open(TSX).read())}
    files = sorted(f for f in os.listdir(HERE) if f.endswith(".png"))
    ids = {f: old[f] for f in files if f in old}
    next_id = max(list(old.values()) + [-1]) + 1
    for f in files:
        if f not in ids:
            ids[f] = next_id
            next_id += 1
    sizes = {f: Image.open(os.path.join(HERE, f)).size for f in files}

    tiles = "".join(f' <tile id="{ids[f]}">\n  <image source="{f}" width="{sizes[f][0]}" height="{sizes[f][1]}"/>\n </tile>\n'
                    for f in sorted(files, key=ids.get))
    mw = max(w for w, _ in sizes.values())
    mh = max(h for _, h in sizes.values())
    with open(TSX, "w") as fh:
        fh.write(f'''<?xml version="1.0" encoding="UTF-8"?>
<tileset version="1.10" tiledversion="1.11.0" name="props" tilewidth="{mw}" tileheight="{mh}" tilecount="{len(files)}" columns="0" nextid="{next_id}" objectalignment="bottom">
 <grid orientation="orthogonal" width="1" height="1"/>
{tiles}</tileset>
''')

    # Placed objects carry their own width/height: keep them in step with the PNGs
    by_id = {i: f for f, i in ids.items()}
    for tmx in glob.glob(os.path.join(HERE, "..", "maps", "*.tmx")):
        text = open(tmx).read()
        m = re.search(r'<tileset firstgid="(\d+)" source="[^"]*props\.tsx"', text)
        if not m:
            continue
        first = int(m.group(1))

        def fix(obj):
            gid = int(obj.group(2)) & 0x0FFFFFFF
            f = by_id.get(gid - first)
            if not f:
                return obj.group(0)
            w, h = sizes[f]
            return re.sub(r'width="[\d.]+" height="[\d.]+"', f'width="{w}" height="{h}"', obj.group(0))

        text = re.sub(r'(<object [^>]*gid="(\d+)"[^>]*/>)', fix, text)
        open(tmx, "w").write(text)
    print(f"props.tsx: {len(files)} objects ({len(files) - len(old)} new)")


if __name__ == "__main__":
    main()
