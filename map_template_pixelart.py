"""Generate a quarter-size (550x325) pixel art template.
Each pixel here = 4x4 pixels in game. Draw at this size, the game
upscales with nearest-neighbor (crisp pixels, no blur)."""

from PIL import Image, ImageDraw, ImageFont

SCALE = 4
W, H = 2200 // SCALE, 1300 // SCALE  # 550 x 325

img = Image.new("RGBA", (W, H), (10, 10, 26, 255))
draw = ImageDraw.Draw(img)

try:
    font = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 6)
    font_sm = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 5)
except:
    font = ImageFont.load_default()
    font_sm = font

def s(v):
    """Scale a coordinate from game space to template space."""
    return v // SCALE

def sr(x, y, w, h):
    """Scale a rect."""
    return [s(x), s(y), s(x+w), s(y+h)]

# --- Hallways ---
hallways = [
    (440, 200, 520, 100),
    (1240, 200, 520, 100),
    (440, 1000, 520, 100),
    (1240, 1000, 520, 100),
    (200, 340, 100, 620),
    (1050, 340, 100, 620),
    (1800, 340, 100, 620),
]
for x, y, w, h in hallways:
    draw.rectangle(sr(x, y, w, h), fill=(45, 55, 72, 200), outline=(80, 80, 100, 255), width=1)

# --- Rooms ---
rooms = [
    ("MedBay",     100,  100, 400, 300, "#2D6A4F"),
    ("Cafe",       900,  100, 400, 300, "#4A5568"),
    ("Nav",       1700,  100, 400, 300, "#1E40AF"),
    ("Engine",     100,  900, 400, 300, "#92400E"),
    ("Storage",    900,  900, 400, 300, "#5B21B6"),
    ("Reactor",   1700,  900, 400, 300, "#991B1B"),
]

def hex_to_rgb(h):
    h = h.lstrip("#")
    return tuple(int(h[i:i+2], 16) for i in (0, 2, 4))

for name, x, y, w, h, color in rooms:
    r, g, b = hex_to_rgb(color)
    draw.rectangle(sr(x, y, w, h), fill=(r, g, b, 120), outline=(180, 180, 200, 255), width=1)
    draw.text((s(x) + 3, s(y) + 2), name, fill="white", font=font)

# --- Obstacles (RED) ---
obstacles = [
    ("Bed",     150,  170,  60,  70),
    ("Bed",     390,  170,  60,  70),
    ("Tbl",    1040,  265, 120,  40),
    ("Dsk",    1820,  160, 160,  25),
    ("Trb",     170, 1040,  60,  60),
    ("Trb",     330, 1040,  60,  60),
    ("Crt",     960,  980,  80,  70),
    ("Crt",    1120, 1080,  80,  70),
    ("Cor",    1860, 1030,  80,  80),
]
for label, x, y, w, h in obstacles:
    draw.rectangle(sr(x, y, w, h), fill=(255, 50, 50, 80), outline=(255, 80, 80, 255), width=1)

# --- Spawn ---
sx, sy = s(1100), s(180)
draw.ellipse([sx-12, sy-12, sx+12, sy+12], outline=(80, 255, 80, 200), width=1)

# --- Task stations (YELLOW) ---
task_stations = [
    (200, 250), (400, 300),
    (1000, 200), (1200, 350),
    (1800, 300), (2000, 250),
    (200, 960), (400, 960),
    (1000, 1100), (1200, 1000),
    (1800, 960), (2000, 1100),
]
for tx, ty in task_stations:
    px, py = s(tx), s(ty)
    draw.rectangle([px-1, py-1, px+1, py+1], fill=(255, 200, 50, 200))

# --- Player size reference ---
pr = s(16)  # player radius = 4px at this scale
ref_x, ref_y = W - 20, H - 15
draw.ellipse([ref_x-pr, ref_y-pr, ref_x+pr, ref_y+pr],
             fill=(60, 130, 240, 200), outline=(255, 255, 255, 200), width=1)
draw.text((ref_x - 14, ref_y + pr + 1), "player", fill=(150, 150, 150), font=font_sm)

# --- Grid every 25px (= 100 game pixels) ---
for gx in range(0, W, 25):
    draw.line([(gx, 0), (gx, H)], fill=(255, 255, 255, 20), width=1)
for gy in range(0, H, 25):
    draw.line([(0, gy), (W, gy)], fill=(255, 255, 255, 20), width=1)

# Save
out_path = "/home/franz/Dropbox/IT/gameX/map_template_pixel.png"
img.save(out_path)
print(f"Saved pixel art template to {out_path}")
print(f"Size: {W}x{H}px (each pixel = {SCALE}x{SCALE} in game)")
print(f"Player radius at this scale: {pr}px")
