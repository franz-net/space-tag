#!/usr/bin/env python3
"""Give every room the same (clean) wall by recomposing its tileset from a template.

usage: python3 retile.py [template]   (default template: cafeteria)

PixelLab draws the shared hull wall slightly differently in each tileset,
with its own small glitches. This keeps the template's walls, trims and
stripes, and swaps in each room's own floor (from its plain floor tile,
read from _originals/<room>.png + .tsx, the untouched PixelLab downloads), darkening it where the template had a shadow.
Results land in <room>/<room>.png + .tsx (same tile layout as the template).
"""
import os
import re
import sys
from PIL import Image

ROOMS = ["cafeteria", "medbay", "navigation", "engine", "storage", "reactor", "hallway"]
HERE = os.path.dirname(os.path.abspath(__file__))
T = 64

# AAP-64, the palette every tileset was generated with
PALETTE = [tuple(int(h[i:i + 2], 16) for i in (1, 3, 5)) for h in (
    "#060608 #141013 #3b1725 #73172d #b4202a #df3e23 #fa6a0a #f9a31b #ffd541 #fffc40 #d6f264 "
    "#9cdb43 #59c135 #14a02e #1a7a3e #24523b #122020 #143464 #285cc4 #249fde #20d6c7 #a6fcdb "
    "#ffffff #fef3c0 #fad6b8 #f5a097 #e86a73 #bc4a9b #793a80 #403353 #242234 #221c1a #322b28 "
    "#71413b #bb7547 #dba463 #f4d29c #dae0ea #b3b9d1 #8b93af #6d758d #4a5462 #333941 #422433 "
    "#5b3138 #8e5252 #ba756a #e9b5a3 #e3e6ff #b9bffb #849be4 #588dbe #477d85 #23674e #328464 "
    "#5daf8d #92dcba #cdf7e2 #e4d2aa #c7b08b #a08662 #796755 #5a4e44 #423934").split()]


def nearest(c):
    return min(PALETTE, key=lambda p: sum((a - b) ** 2 for a, b in zip(p, c)))


def lum(c):
    return 0.299 * c[0] + 0.587 * c[1] + 0.114 * c[2]


def floor_tile_id(tsx_path):
    return int(re.search(r'<wangcolor name="Floor[^"]*" color="[^"]*" tile="(\d+)"', open(tsx_path).read()).group(1))


def crop(im, tid):
    cols = im.width // T
    return im.crop(((tid % cols) * T, (tid // cols) * T, (tid % cols + 1) * T, (tid // cols + 1) * T))


def main(template):
    src = Image.open(os.path.join(HERE, "_originals", f"{template}.png")).convert("RGB")
    tsx = open(os.path.join(HERE, "_originals", f"{template}.tsx")).read()
    tfloor = crop(src, floor_tile_id(os.path.join(HERE, "_originals", f"{template}.tsx")))
    floor_colors = set(tfloor.getdata())
    base = max(floor_colors, key=lambda c: list(tfloor.getdata()).count(c))  # main floor color
    bright = {c for c in floor_colors if lum(c) >= lum(base)}  # floor, never shadow

    # Per template pixel: None = wall (keep), else a brightness factor for the room floor
    plan = []
    for y in range(src.height):
        row = []
        for x in range(src.width):
            p = src.getpixel((x, y))
            # Light, near-gray pixels are floor highlights (the yellow stripe is bright but saturated)
            highlight = lum(p) >= lum(base) and max(p) - min(p) < 32
            if p == tfloor.getpixel((x % T, y % T)) or p in bright or highlight:
                row.append(1.0)
            elif p in floor_colors:
                row.append(lum(p) / lum(base))  # shadow cast on the floor
            else:
                row.append(None)
        plan.append(row)

    for room in ROOMS:
        orig = Image.open(os.path.join(HERE, "_originals", f"{room}.png")).convert("RGB")
        rfloor = crop(orig, floor_tile_id(os.path.join(HERE, "_originals", f"{room}.tsx")))
        out = src.copy()
        cache = {}
        for y in range(src.height):
            for x in range(src.width):
                k = plan[y][x]
                if k is None:
                    continue
                f = rfloor.getpixel((x % T, y % T))
                if k != 1.0:
                    key = (f, round(k, 2))
                    if key not in cache:
                        cache[key] = nearest(tuple(min(255, int(v * k)) for v in f))
                    f = cache[key]
                out.putpixel((x, y), f)
        out.save(os.path.join(HERE, room, f"{room}.png"))
        with open(os.path.join(HERE, room, f"{room}.tsx"), "w") as fh:
            fh.write(tsx.replace(f'name="{template}"', f'name="{room}"')
                        .replace(f"{template}.png", f"{room}.png")
                        .replace(f'name="{template.capitalize()} floor', f'name="{room.capitalize()} floor'))
        print(f"{room}: recomposed from {template} walls")


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else "cafeteria")
