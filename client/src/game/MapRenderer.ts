import { Container, Graphics, Text } from "pixi.js";

export interface MapRect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface MapRoomData {
  id: string;
  label: string;
  bounds: MapRect;
  color: string;
}

export interface HallwayData {
  bounds: MapRect;
}

export interface GameMapData {
  rooms: MapRoomData[];
  hallways: HallwayData[];
  walls: MapRect[] | null;
  spawnPos: { x: number; y: number };
  width: number;
  height: number;
}

export class MapRenderer {
  /** Space + stars — always visible */
  backgroundContainer: Container;
  /** Rooms + hallways — masked by fog of war */
  shipContainer: Container;

  constructor(mapData: GameMapData) {
    this.backgroundContainer = new Container();
    this.shipContainer = new Container();

    // Background: dark space + stars
    this.drawBackground(mapData.width, mapData.height);

    // Ship interior
    for (const hallway of mapData.hallways) {
      this.drawHallway(hallway);
    }
    for (const room of mapData.rooms) {
      this.drawRoom(room);
      this.drawRoomProps(room);
    }
  }

  private drawBackground(width: number, height: number) {
    const bg = new Graphics();
    bg.rect(0, 0, width, height);
    bg.fill(0x0a0a1a);
    this.backgroundContainer.addChild(bg);

    // Sprinkle some stars
    const stars = new Graphics();
    for (let i = 0; i < 200; i++) {
      const x = Math.random() * width;
      const y = Math.random() * height;
      const size = Math.random() * 2 + 0.5;
      const alpha = Math.random() * 0.5 + 0.2;
      stars.circle(x, y, size);
      stars.fill({ color: 0xffffff, alpha });
    }
    this.backgroundContainer.addChild(stars);
  }

  private drawHallway(hallway: HallwayData) {
    const { x, y, w, h } = hallway.bounds;
    const g = new Graphics();

    // Floor
    g.roundRect(x, y, w, h, 4);
    g.fill(0x2d3748);

    // Subtle border
    g.roundRect(x, y, w, h, 4);
    g.stroke({ color: 0x4a5568, width: 1 });

    this.shipContainer.addChild(g);
  }

  private drawRoom(room: MapRoomData) {
    const { x, y, w, h } = room.bounds;
    const g = new Graphics();

    // Room floor
    const color = parseInt(room.color.replace("#", ""), 16);
    g.roundRect(x, y, w, h, 12);
    g.fill(color);

    // Room border
    g.roundRect(x, y, w, h, 12);
    g.stroke({ color: 0x718096, width: 2 });

    this.shipContainer.addChild(g);

    // Room label
    const label = new Text({
      text: room.label,
      style: {
        fontFamily: "Arial",
        fontSize: 20,
        fontWeight: "bold",
        fill: 0xffffff,
        align: "center",
        dropShadow: {
          color: 0x000000,
          blur: 4,
          distance: 0,
          alpha: 0.5,
        },
      },
    });
    label.anchor.set(0.5);
    label.x = x + w / 2;
    label.y = y + 24;
    this.shipContainer.addChild(label);
  }

  /** Draw decorative props inside each room so they look distinct. */
  private drawRoomProps(room: MapRoomData) {
    const { x, y, w, h } = room.bounds;
    const cx = x + w / 2;
    const cy = y + h / 2;
    const g = new Graphics();

    switch (room.id) {
      case "medbay":
        this.drawMedbay(g, x, y, w, h);
        break;
      case "cafeteria":
        this.drawCafeteria(g, cx, cy);
        break;
      case "navigation":
        this.drawNavigation(g, x, y, w, h);
        break;
      case "engine":
        this.drawEngine(g, x, y, w, h);
        break;
      case "storage":
        this.drawStorage(g, x, y, w, h);
        break;
      case "reactor":
        this.drawReactor(g, cx, cy);
        break;
    }

    this.shipContainer.addChild(g);
  }

  // ─── Per-room decorations ──────────────────────────────────────────

  private drawMedbay(g: Graphics, x: number, y: number, w: number, _h: number) {
    // Two hospital beds in upper portion of room (clear of hallway exits)
    for (const bx of [x + 50, x + w - 110]) {
      g.roundRect(bx, y + 70, 60, 70, 6).fill(0xc8dce8);
      g.roundRect(bx, y + 70, 60, 70, 6).stroke({ color: 0x8aabb8, width: 2 });
      // Pillow
      g.roundRect(bx + 10, y + 78, 40, 16, 4).fill(0xe8f0f4);
    }
    // Medical cross on the wall
    const crossX = x + w / 2;
    const crossY = y + 70;
    g.rect(crossX - 3, crossY - 12, 6, 24).fill(0xff4444);
    g.rect(crossX - 12, crossY - 3, 24, 6).fill(0xff4444);
  }

  private drawCafeteria(g: Graphics, cx: number, cy: number) {
    // Oval table — centered in room, narrow to leave side passages
    const tableY = cy - 10;
    g.ellipse(cx, tableY, 60, 25).fill(0x6b5335);
    g.ellipse(cx, tableY, 60, 25).stroke({ color: 0x8b7355, width: 2 });
    // Stools around the table
    const stools = [
      [-45, -18],
      [45, -18],
      [-45, 18],
      [45, 18],
      [0, -30],
      [0, 30],
    ];
    for (const [sx, sy] of stools) {
      g.circle(cx + sx, tableY + sy, 8).fill(0x555555);
      g.circle(cx + sx, tableY + sy, 8).stroke({ color: 0x777777, width: 1 });
    }
    // Emergency button (decorative red circle on table)
    g.circle(cx, tableY, 8).fill(0xff3333);
    g.circle(cx, tableY, 8).stroke({ color: 0xff0000, width: 1.5 });
  }

  private drawNavigation(
    g: Graphics,
    x: number,
    y: number,
    w: number,
    h: number
  ) {
    const cx = x + w / 2;
    // Curved console desk at the top
    g.roundRect(cx - 80, y + 55, 160, 30, 8).fill(0x1a1a3a);
    g.roundRect(cx - 80, y + 55, 160, 30, 8).stroke({ color: 0x4488ff, width: 2 });
    // Screen on the console
    g.roundRect(cx - 50, y + 60, 100, 18, 4).fill(0x001133);
    // Blips on the screen (tiny dots = "radar")
    for (const [dx, dy] of [[-20, 5], [15, 8], [5, -2], [-30, 2], [35, 6]]) {
      g.circle(cx + dx, y + 69 + dy, 2).fill(0x44ff88);
    }
    // Pilot chair
    g.roundRect(cx - 15, y + h - 100, 30, 40, 6).fill(0x333355);
    g.roundRect(cx - 15, y + h - 100, 30, 40, 6).stroke({ color: 0x4a4a6a, width: 1 });
    // Star chart on the wall (bottom-right corner)
    const chartX = x + w - 100;
    const chartY = y + h - 90;
    g.roundRect(chartX, chartY, 60, 50, 4).fill(0x0a0a2a);
    g.roundRect(chartX, chartY, 60, 50, 4).stroke({ color: 0x4488ff, width: 1 });
    for (const [dx, dy] of [[10, 10], [35, 15], [20, 35], [45, 30], [15, 25]]) {
      g.circle(chartX + dx, chartY + dy, 1.5).fill(0xffffff);
    }
  }

  private drawEngine(
    g: Graphics,
    x: number,
    y: number,
    w: number,
    _h: number
  ) {
    // Two turbine circles — centered, clear of right hallway exit
    const ty = y + 170;
    for (const tx of [x + 100, x + w - 170]) {
      // Outer ring
      g.circle(tx, ty, 30).fill(0x3a3a3a);
      g.circle(tx, ty, 30).stroke({ color: 0xff8800, width: 2 });
      // Inner ring
      g.circle(tx, ty, 14).fill(0x1a1a1a);
      g.circle(tx, ty, 14).stroke({ color: 0xff6600, width: 1 });
      // Center bolt
      g.circle(tx, ty, 4).fill(0xff8800);
    }
    // Connecting pipes between turbines
    const pipeX = x + 130;
    const pipeW = w - 300;
    g.rect(pipeX, ty - 5, pipeW, 10).fill(0x555555);
    g.rect(pipeX, ty - 5, pipeW, 10).stroke({ color: 0xff8800, width: 1 });
  }

  private drawStorage(
    g: Graphics,
    x: number,
    y: number,
    _w: number,
    _h: number
  ) {
    // Two crate stacks — positioned to match collision boxes and clear
    // of left (x≈960) and right (x≈1240) hallway entrances
    const stacks = [
      // Left stack (collision: x+60, y+80, 80x70)
      { cx: 60, cy: 80, cw: 80, ch: 70 },
      // Right stack (collision: x+220, y+180, 80x70)
      { cx: 220, cy: 180, cw: 80, ch: 70 },
    ];
    for (const s of stacks) {
      // Bottom crate
      g.roundRect(x + s.cx, y + s.cy, s.cw, s.ch / 2 + 5, 3).fill(0x8a6830);
      g.roundRect(x + s.cx, y + s.cy, s.cw, s.ch / 2 + 5, 3).stroke({ color: 0xbb9944, width: 1 });
      // Top crate (slightly offset)
      g.roundRect(x + s.cx + 5, y + s.cy + s.ch / 2 + 5, s.cw - 10, s.ch / 2, 3).fill(0x7a5c28);
      g.roundRect(x + s.cx + 5, y + s.cy + s.ch / 2 + 5, s.cw - 10, s.ch / 2, 3).stroke({ color: 0xbb9944, width: 1 });
      // Tape band
      g.rect(x + s.cx + 8, y + s.cy + s.ch / 2 - 2, s.cw - 16, 4).fill(0xddaa55);
    }
  }

  private drawReactor(g: Graphics, cx: number, cy: number) {
    cy += 30; // shift down from room label
    // Outer containment ring
    g.circle(cx, cy, 40).fill(0x2a0000);
    g.circle(cx, cy, 40).stroke({ color: 0xff2222, width: 2 });
    // Inner core ring
    g.circle(cx, cy, 22).fill(0x440000);
    g.circle(cx, cy, 22).stroke({ color: 0xff4444, width: 1.5 });
    // Glowing core
    g.circle(cx, cy, 10).fill(0xff3300);
    // Conduit pipes radiating outward
    for (const angle of [0, Math.PI / 2, Math.PI, (3 * Math.PI) / 2]) {
      const x1 = cx + Math.cos(angle) * 42;
      const y1 = cy + Math.sin(angle) * 42;
      const x2 = cx + Math.cos(angle) * 70;
      const y2 = cy + Math.sin(angle) * 70;
      g.moveTo(x1, y1).lineTo(x2, y2).stroke({ color: 0xff4444, width: 4 });
    }
  }
}
