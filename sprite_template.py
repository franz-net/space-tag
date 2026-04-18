"""Generate a character spritesheet template at 32x32 per frame.

Layout:
  Columns: 3 walk frames x 4 directions = 12 columns
           + 1 frozen frame = 13 columns total
  Rows: 6 colors (red, blue, green, yellow, purple, orange)

Each cell is 32x32. Total sheet: 416 x 192 pixels.

Also generates a 3x zoomed reference guide with labels.
"""

from PIL import Image, ImageDraw, ImageFont
import os

FRAME = 32
DIRS = ["Down", "Up", "Left", "Right"]
FRAMES_PER_DIR = 3
EXTRA_COLS = 1  # frozen
COLS = len(DIRS) * FRAMES_PER_DIR + EXTRA_COLS  # 13
COLORS = [
    ("Red",    (239, 68, 68)),
    ("Blue",   (59, 130, 246)),
    ("Green",  (34, 197, 94)),
    ("Yellow", (234, 179, 8)),
    ("Purple", (168, 85, 247)),
    ("Orange", (249, 115, 22)),
]
ROWS = len(COLORS)

W = COLS * FRAME   # 416
H = ROWS * FRAME   # 192

BASE = "/home/franz/Dropbox/IT/gameX"

try:
    font = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 8)
except:
    font = ImageFont.load_default()

# ============================================================
# 1. Actual spritesheet template (draw on this one)
# ============================================================
sheet = Image.new("RGBA", (W, H), (0, 0, 0, 0))
sheet_draw = ImageDraw.Draw(sheet)

for row, (color_name, (r, g, b)) in enumerate(COLORS):
    for col in range(COLS):
        x = col * FRAME
        y = row * FRAME
        # Cell background
        sheet_draw.rectangle([x, y, x+FRAME-1, y+FRAME-1],
                             fill=(30, 30, 40, 100),
                             outline=(80, 80, 80, 150), width=1)
        cx, cy = x + FRAME // 2, y + FRAME // 2

        # Placeholder astronaut silhouette
        is_frozen = col >= COLS - 1

        # Helmet (round top)
        sheet_draw.ellipse([cx-8, cy-13, cx+8, cy-1],
                           fill=(200, 200, 210, 50), outline=(180, 180, 190, 100), width=1)
        # Visor
        if not is_frozen:
            sheet_draw.rectangle([cx-5, cy-8, cx+5, cy-4],
                                 fill=(r, g, b, 100), outline=(r, g, b, 150), width=1)
        else:
            sheet_draw.rectangle([cx-5, cy-8, cx+5, cy-4],
                                 fill=(100, 150, 200, 60), outline=(150, 200, 255, 120), width=1)
        # Body
        sheet_draw.rounded_rectangle([cx-9, cy-2, cx+9, cy+10],
                                      radius=3,
                                      fill=(200, 200, 210, 40),
                                      outline=(180, 180, 190, 80), width=1)
        # Backpack bump (right side for Down/Left, left for Right)
        sheet_draw.rectangle([cx+8, cy-1, cx+11, cy+6],
                             fill=(180, 180, 190, 40), outline=(160, 160, 170, 60))
        # Legs
        sheet_draw.rectangle([cx-5, cy+10, cx-2, cy+14],
                             fill=(200, 200, 210, 40), outline=(180, 180, 190, 60))
        sheet_draw.rectangle([cx+2, cy+10, cx+5, cy+14],
                             fill=(200, 200, 210, 40), outline=(180, 180, 190, 60))

        if is_frozen:
            # Ice crystal hints
            sheet_draw.text((cx-3, cy+3), "ice", fill=(150, 200, 255, 80), font=font)

sheet.save(os.path.join(BASE, "sprite_template.png"))
print(f"Spritesheet template: {W}x{H}px ({COLS} cols x {ROWS} rows, {FRAME}x{FRAME} per frame)")

# ============================================================
# 2. Zoomed reference guide (3x scale, with labels)
# ============================================================
ZOOM = 3
ZW = W * ZOOM + 180
ZH = H * ZOOM + 80

guide = Image.new("RGBA", (ZW, ZH), (20, 20, 30, 255))
guide_draw = ImageDraw.Draw(guide)

try:
    gfont = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 14)
    gfont_sm = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 11)
    gfont_xs = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 9)
except:
    gfont = gfont_sm = gfont_xs = ImageFont.load_default()

OFFSET_X = 60
OFFSET_Y = 55

# Direction group headers
for i, d in enumerate(DIRS):
    gx = OFFSET_X + i * FRAMES_PER_DIR * FRAME * ZOOM
    gw = FRAMES_PER_DIR * FRAME * ZOOM
    bbox = guide_draw.textbbox((0, 0), d, font=gfont_sm)
    tw = bbox[2] - bbox[0]
    guide_draw.text((gx + gw // 2 - tw // 2, OFFSET_Y - 32), d, fill="white", font=gfont_sm)
    guide_draw.line([(gx + 4, OFFSET_Y - 18), (gx + gw - 4, OFFSET_Y - 18)],
                    fill=(100, 100, 100), width=1)

# Frame numbers
for col in range(COLS):
    cx = OFFSET_X + col * FRAME * ZOOM + FRAME * ZOOM // 2
    if col < 12:
        label = str((col % 3) + 1)
    else:
        label = "F"
    bbox = guide_draw.textbbox((0, 0), label, font=gfont_xs)
    tw = bbox[2] - bbox[0]
    guide_draw.text((cx - tw // 2, OFFSET_Y - 14), label,
                    fill=(150, 150, 150) if col < 12 else (150, 200, 255),
                    font=gfont_xs)

# Frozen header
fx = OFFSET_X + 12 * FRAME * ZOOM
guide_draw.text((fx + 10, OFFSET_Y - 32), "Frozen", fill=(150, 200, 255), font=gfont_sm)

# Row labels
for row, (color_name, (r, g, b)) in enumerate(COLORS):
    ry = OFFSET_Y + row * FRAME * ZOOM + FRAME * ZOOM // 2 - 6
    guide_draw.text((6, ry), color_name, fill=(r, g, b), font=gfont_sm)

# Draw zoomed cells with placeholders
zoomed_sheet = sheet.resize((W * ZOOM, H * ZOOM), Image.NEAREST)
guide.paste(zoomed_sheet, (OFFSET_X, OFFSET_Y), zoomed_sheet)

# Grid lines on top
for col in range(COLS + 1):
    x = OFFSET_X + col * FRAME * ZOOM
    guide_draw.line([(x, OFFSET_Y), (x, OFFSET_Y + H * ZOOM)], fill=(80, 80, 80), width=1)
for row in range(ROWS + 1):
    y = OFFSET_Y + row * FRAME * ZOOM
    guide_draw.line([(OFFSET_X, y), (OFFSET_X + W * ZOOM, y)], fill=(80, 80, 80), width=1)

# Info
info_y = OFFSET_Y + ROWS * ZOOM * FRAME + 10
guide_draw.text((10, info_y),
    f"Each cell: {FRAME}x{FRAME}px  |  Sheet: {W}x{H}px  |  {ZOOM}x zoom shown above  |  "
    f"In-game: {FRAME*4}x{FRAME*4}px (4x upscale)",
    fill=(120, 120, 120), font=gfont_xs)

guide.save(os.path.join(BASE, "sprite_template_guide.png"))
print(f"Reference guide: {ZW}x{ZH}px ({ZOOM}x zoom with labels)")

# ============================================================
# 3. Minimal: 1 frame per color
# ============================================================
mini = Image.new("RGBA", (FRAME, FRAME * ROWS), (0, 0, 0, 0))
mini_draw = ImageDraw.Draw(mini)

for row, (color_name, (r, g, b)) in enumerate(COLORS):
    x, y = 0, row * FRAME
    mini_draw.rectangle([x, y, x+FRAME-1, y+FRAME-1],
                        fill=(30, 30, 40, 100), outline=(80, 80, 80, 150))
    cx, cy = FRAME // 2, y + FRAME // 2
    # Helmet
    mini_draw.ellipse([cx-8, cy-13, cx+8, cy-1],
                      fill=(200, 200, 210, 50), outline=(180, 180, 190, 100), width=1)
    # Visor
    mini_draw.rectangle([cx-5, cy-8, cx+5, cy-4],
                        fill=(r, g, b, 100), outline=(r, g, b, 150), width=1)
    # Body
    mini_draw.rounded_rectangle([cx-9, cy-2, cx+9, cy+10],
                                 radius=3,
                                 fill=(200, 200, 210, 40),
                                 outline=(180, 180, 190, 80), width=1)
    # Legs
    mini_draw.rectangle([cx-5, cy+10, cx-2, cy+14],
                        fill=(200, 200, 210, 40), outline=(180, 180, 190, 60))
    mini_draw.rectangle([cx+2, cy+10, cx+5, cy+14],
                        fill=(200, 200, 210, 40), outline=(180, 180, 190, 60))

mini.save(os.path.join(BASE, "sprite_template_minimal.png"))
print(f"Minimal template (1 frame per color): {FRAME}x{FRAME * ROWS}px")

# ============================================================
# Cleanup old 16x16 files
# ============================================================
for old_file in ["sprite_template_pixelart.py"]:
    path = os.path.join(BASE, old_file)
    if os.path.exists(path):
        os.remove(path)
        print(f"Removed old file: {old_file}")
