"""Generate a map template PNG showing all room bounds, hallways, obstacles,
task stations, fix stations, spawn point, and waypoints — so an artist can
draw on top of it and know everything lines up with the game's collision."""

from PIL import Image, ImageDraw, ImageFont

W, H = 2200, 1300

img = Image.new("RGBA", (W, H), (10, 10, 26, 255))  # dark space background
draw = ImageDraw.Draw(img)

# Try to use a decent font, fall back to default
try:
    font = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 20)
    font_sm = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 14)
    font_xs = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 11)
except:
    font = ImageFont.load_default()
    font_sm = font
    font_xs = font

# --- Hallways (draw first, under rooms) ---
hallways = [
    (440, 200, 520, 100),   # medbay <-> cafeteria
    (1240, 200, 520, 100),  # cafeteria <-> navigation
    (440, 1000, 520, 100),  # engine <-> storage
    (1240, 1000, 520, 100), # storage <-> reactor
    (200, 340, 100, 620),   # medbay <-> engine
    (1050, 340, 100, 620),  # cafeteria <-> storage
    (1800, 340, 100, 620),  # navigation <-> reactor
]
for x, y, w, h in hallways:
    draw.rectangle([x, y, x+w, y+h], fill=(45, 55, 72, 200), outline=(100, 100, 120, 255), width=1)

# --- Rooms ---
rooms = [
    ("Medbay",     100,  100, 400, 300, "#2D6A4F"),
    ("Cafeteria",  900,  100, 400, 300, "#4A5568"),
    ("Navigation", 1700, 100, 400, 300, "#1E40AF"),
    ("Engine",     100,  900, 400, 300, "#92400E"),
    ("Storage",    900,  900, 400, 300, "#5B21B6"),
    ("Reactor",    1700, 900, 400, 300, "#991B1B"),
]

def hex_to_rgb(h):
    h = h.lstrip("#")
    return tuple(int(h[i:i+2], 16) for i in (0, 2, 4))

for name, x, y, w, h, color in rooms:
    r, g, b = hex_to_rgb(color)
    # Semi-transparent fill
    draw.rectangle([x, y, x+w, y+h], fill=(r, g, b, 120), outline=(180, 180, 200, 255), width=2)
    # Room label
    bbox = draw.textbbox((0, 0), name, font=font)
    tw = bbox[2] - bbox[0]
    draw.text((x + w//2 - tw//2, y + 12), name, fill="white", font=font)

# --- Obstacles (furniture collision boxes) — RED outlines ---
obstacles = [
    ("Bed",       150,  170,  60,  70),
    ("Bed",       390,  170,  60,  70),
    ("Table",     1040, 265, 120,  40),
    ("Desk",      1820, 160, 160,  25),
    ("Turbine",   170, 1040,  60,  60),
    ("Turbine",   330, 1040,  60,  60),
    ("Crate",     960,  980,  80,  70),
    ("Crate",    1120, 1080,  80,  70),
    ("Core",     1860, 1030,  80,  80),
]
for label, x, y, w, h in obstacles:
    draw.rectangle([x, y, x+w, y+h], fill=(255, 50, 50, 60), outline=(255, 80, 80, 255), width=2)
    bbox = draw.textbbox((0, 0), label, font=font_xs)
    tw = bbox[2] - bbox[0]
    draw.text((x + w//2 - tw//2, y + h//2 - 6), label, fill=(255, 150, 150), font=font_xs)

# --- Spawn point — GREEN circle ---
spawn_x, spawn_y = 1100, 180
draw.ellipse([spawn_x-50, spawn_y-50, spawn_x+50, spawn_y+50],
             outline=(80, 255, 80, 200), width=2)
draw.text((spawn_x - 25, spawn_y - 8), "SPAWN", fill=(80, 255, 80), font=font_sm)

# --- Task station positions — YELLOW dots ---
# From tasks.go: each room has stations at specific positions
task_stations = [
    # Medbay
    (200, 250), (400, 300),
    # Cafeteria
    (1000, 200), (1200, 350),
    # Navigation
    (1800, 300), (2000, 250),
    # Engine
    (200, 960), (400, 960),
    # Storage
    (1000, 1100), (1200, 1000),
    # Reactor
    (1800, 960), (2000, 1100),
]
for tx, ty in task_stations:
    draw.ellipse([tx-8, ty-8, tx+8, ty+8], fill=(255, 200, 50, 150), outline=(255, 200, 50, 255), width=1)

# --- Sabotage fix stations — ORANGE diamonds ---
fix_stations = [
    ("FIX: Lights", 1900, 960),   # reactor (lights_out fix)
    ("FIX: Comms",  1900, 250),   # navigation (comms_down fix)
    ("FIX: Melt1",  300, 960),    # engine (meltdown fix 1)
    ("FIX: Melt2",  1900, 960),   # reactor (meltdown fix 2)
]
for label, fx, fy in fix_stations:
    pts = [(fx, fy-10), (fx+10, fy), (fx, fy+10), (fx-10, fy)]
    draw.polygon(pts, fill=(255, 140, 0, 150), outline=(255, 140, 0, 255))
    draw.text((fx + 14, fy - 7), label, fill=(255, 180, 80), font=font_xs)

# --- Legend ---
legend_x, legend_y = 20, H - 160
draw.rectangle([legend_x, legend_y, legend_x + 280, legend_y + 150],
               fill=(0, 0, 0, 180), outline=(100, 100, 100))
draw.text((legend_x + 10, legend_y + 8), "LEGEND", fill="white", font=font_sm)

# Room fill
draw.rectangle([legend_x+10, legend_y+30, legend_x+30, legend_y+45],
               fill=(70, 80, 100, 120), outline=(180, 180, 200))
draw.text((legend_x+40, legend_y+30), "Room bounds (walkable)", fill=(200,200,200), font=font_xs)

# Hallway
draw.rectangle([legend_x+10, legend_y+50, legend_x+30, legend_y+65],
               fill=(45, 55, 72, 200), outline=(100, 100, 120))
draw.text((legend_x+40, legend_y+50), "Hallway (walkable)", fill=(200,200,200), font=font_xs)

# Obstacle
draw.rectangle([legend_x+10, legend_y+70, legend_x+30, legend_y+85],
               fill=(255, 50, 50, 60), outline=(255, 80, 80))
draw.text((legend_x+40, legend_y+70), "Obstacle (blocks movement)", fill=(200,200,200), font=font_xs)

# Task
draw.ellipse([legend_x+14, legend_y+92, legend_x+26, legend_y+104],
             fill=(255, 200, 50, 150), outline=(255, 200, 50))
draw.text((legend_x+40, legend_y+90), "Task station", fill=(200,200,200), font=font_xs)

# Spawn
draw.ellipse([legend_x+14, legend_y+112, legend_x+26, legend_y+124],
             outline=(80, 255, 80, 200), width=2)
draw.text((legend_x+40, legend_y+110), "Spawn area", fill=(200,200,200), font=font_xs)

# Fix station
draw.text((legend_x+16, legend_y+128), "<>", fill=(255, 140, 0), font=font_xs)
draw.text((legend_x+40, legend_y+130), "Sabotage fix station", fill=(200,200,200), font=font_xs)

# --- Dimensions annotation ---
# Top ruler
draw.line([(0, 8), (W, 8)], fill=(100, 100, 100), width=1)
draw.text((W//2 - 40, 12), f"{W}px", fill=(150, 150, 150), font=font_sm)
# Left ruler
draw.line([(8, 0), (8, H)], fill=(100, 100, 100), width=1)

# Player size reference
ref_x, ref_y = W - 120, H - 60
draw.ellipse([ref_x-16, ref_y-16, ref_x+16, ref_y+16],
             fill=(60, 130, 240, 200), outline=(255, 255, 255, 200), width=2)
draw.text((ref_x - 45, ref_y + 20), "Player r=16", fill=(150, 150, 150), font=font_xs)

# Grid lines every 100px (very subtle)
for gx in range(0, W, 100):
    draw.line([(gx, 0), (gx, H)], fill=(255, 255, 255, 15), width=1)
for gy in range(0, H, 100):
    draw.line([(0, gy), (W, gy)], fill=(255, 255, 255, 15), width=1)

# Save
out_path = "/home/franz/Dropbox/IT/gameX/map_template.png"
img.save(out_path)
print(f"Saved template to {out_path}")
print(f"Size: {W}x{H}px")
