#!/usr/bin/env python3
"""Give every room the same (clean) walls by recomposing its tilesets from
the cafeteria's.

usage: python3 retile.py

PixelLab draws the shared hull wall slightly differently in each tileset,
with its own small glitches. This keeps the cafeteria's walls, trims and
stripes and swaps in each room's own floor (from its plain floor tile in
_originals/), darkening it where the template had a shadow. For each room
it writes, into <room>/:

  <room>.png / .tsx            16-tile set: paint rooms with it in Tiled
  <room>-tall.png / .tsx / .json  tall-wall set the game draws (autotile.py)
"""
import json
import os
import re
import shutil
from PIL import Image

ROOMS = ["cafeteria", "medbay", "navigation", "engine", "storage", "reactor", "hallway"]
TEMPLATE = "cafeteria"
HERE = os.path.dirname(os.path.abspath(__file__))
ORIG = os.path.join(HERE, "_originals")

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


def floor_tile(name):
    """The all-floor tile of an _originals/ sheet, from its metadata."""
    meta = json.load(open(os.path.join(ORIG, f"{name}.json")))
    size = meta["tile_size"]
    for t in meta["tiles"]:
        if set(t["corners"].values()) == {"lower"}:
            sheet = Image.open(os.path.join(ORIG, f"{name}.png")).convert("RGB")
            return sheet.crop((t["x"], t["y"], t["x"] + size, t["y"] + size)), size
    raise ValueError(f"{name}: no all-floor tile")


def recompose(template, rooms_floor):
    """Yield (room, image): the template sheet with each room's floor."""
    src = Image.open(os.path.join(ORIG, f"{template}.png")).convert("RGB")
    tfloor, T = floor_tile(template)
    floor_colors = set(tfloor.getdata())
    pixels = list(tfloor.getdata())
    base = max(floor_colors, key=pixels.count)  # main floor colour
    bright = {c for c in floor_colors if lum(c) >= lum(base)}

    # Per template pixel: None = wall (keep), else a brightness factor for the room floor
    plan = []
    for y in range(src.height):
        row = []
        for x in range(src.width):
            p = src.getpixel((x, y))
            # Light, near-grey pixels are floor highlights (the yellow stripe is bright but saturated)
            highlight = lum(p) >= lum(base) and max(p) - min(p) < 32
            if p == tfloor.getpixel((x % T, y % T)) or p in bright or highlight:
                row.append(1.0)
            elif p in floor_colors:
                row.append(lum(p) / lum(base))  # shadow cast on the floor
            else:
                row.append(None)
        plan.append(row)

    for room, rfloor in rooms_floor.items():
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
        yield room, out


def rename_tsx(tsx, old, new):
    return (tsx.replace(f'name="{old}', f'name="{new}')
               .replace(f'source="{old}', f'source="{new}')
               .replace(f'name="{old.capitalize()} floor', f'name="{new.split("-")[0].capitalize()} floor'))


def main():
    floors = {room: floor_tile(room)[0] for room in ROOMS}
    for suffix in ("", "-tall"):
        template = TEMPLATE + suffix
        if not os.path.exists(os.path.join(ORIG, f"{template}.png")):
            print(f"skipping {template} (not in _originals/)")
            continue
        tsx = open(os.path.join(ORIG, f"{template}.tsx")).read()
        for room, img in recompose(template, floors):
            name = room + suffix
            os.makedirs(os.path.join(HERE, room), exist_ok=True)
            img.save(os.path.join(HERE, room, f"{name}.png"))
            with open(os.path.join(HERE, room, f"{name}.tsx"), "w") as fh:
                fh.write(rename_tsx(tsx, template, name))
            if suffix:
                shutil.copy(os.path.join(ORIG, f"{template}.json"), os.path.join(HERE, room, f"{name}.json"))
            print(f"{name}: recomposed from {template}")


if __name__ == "__main__":
    main()
