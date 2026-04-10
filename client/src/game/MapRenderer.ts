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

  private drawMedbay(g: Graphics, x: number, y: number, w: number, h: number) {
    // Two hospital beds side by side
    for (const bx of [x + 60, x + w - 130]) {
      // Bed frame
      g.roundRect(bx, y + h - 130, 70, 100, 6).fill({ color: 0xffffff, alpha: 0.15 });
      g.roundRect(bx, y + h - 130, 70, 100, 6).stroke({ color: 0xffffff, alpha: 0.3, width: 1 });
      // Pillow
      g.roundRect(bx + 15, y + h - 120, 40, 20, 4).fill({ color: 0xffffff, alpha: 0.25 });
    }
    // Medical cross
    const crossX = x + w / 2;
    const crossY = y + 70;
    g.rect(crossX - 3, crossY - 12, 6, 24).fill({ color: 0xff4444, alpha: 0.6 });
    g.rect(crossX - 12, crossY - 3, 24, 6).fill({ color: 0xff4444, alpha: 0.6 });
  }

  private drawCafeteria(g: Graphics, cx: number, cy: number) {
    // Long oval table — shifted to bottom half of room to avoid spawn point
    const tableY = cy + 90;
    g.ellipse(cx, tableY, 80, 35).fill({ color: 0x8b7355, alpha: 0.4 });
    g.ellipse(cx, tableY, 80, 35).stroke({ color: 0xffffff, alpha: 0.15, width: 1 });
    // Stools around the table
    const stools = [
      [-55, -15],
      [55, -15],
      [-55, 55],
      [55, 55],
      [0, -30],
      [0, 70],
    ];
    for (const [sx, sy] of stools) {
      g.circle(cx + sx, tableY + sy, 8).fill({ color: 0x666666, alpha: 0.35 });
    }
    // Emergency button (decorative red circle on table)
    g.circle(cx, tableY, 10).fill({ color: 0xff3333, alpha: 0.5 });
    g.circle(cx, tableY, 10).stroke({ color: 0xff0000, alpha: 0.4, width: 1.5 });
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
    g.roundRect(cx - 80, y + 55, 160, 30, 8).fill({ color: 0x1a1a3a, alpha: 0.6 });
    g.roundRect(cx - 80, y + 55, 160, 30, 8).stroke({ color: 0x4488ff, alpha: 0.4, width: 1 });
    // Screen on the console
    g.roundRect(cx - 50, y + 60, 100, 18, 4).fill({ color: 0x001133, alpha: 0.7 });
    // Blips on the screen (tiny dots = "radar")
    for (const [dx, dy] of [[-20, 5], [15, 8], [5, -2], [-30, 2], [35, 6]]) {
      g.circle(cx + dx, y + 69 + dy, 2).fill({ color: 0x44ff88, alpha: 0.7 });
    }
    // Pilot chair
    g.roundRect(cx - 15, y + h - 100, 30, 40, 6).fill({ color: 0x333355, alpha: 0.4 });
    // Star chart on the wall (bottom-right corner)
    const chartX = x + w - 100;
    const chartY = y + h - 90;
    g.roundRect(chartX, chartY, 60, 50, 4).fill({ color: 0x0a0a2a, alpha: 0.5 });
    g.roundRect(chartX, chartY, 60, 50, 4).stroke({ color: 0x4488ff, alpha: 0.3, width: 1 });
    for (const [dx, dy] of [[10, 10], [35, 15], [20, 35], [45, 30], [15, 25]]) {
      g.circle(chartX + dx, chartY + dy, 1.5).fill({ color: 0xffffff, alpha: 0.5 });
    }
  }

  private drawEngine(
    g: Graphics,
    x: number,
    y: number,
    w: number,
    h: number
  ) {
    // Two large turbine circles
    for (const [tx, ty] of [
      [x + 100, y + h / 2 + 20],
      [x + w - 100, y + h / 2 + 20],
    ]) {
      // Outer ring
      g.circle(tx, ty, 40).fill({ color: 0x333333, alpha: 0.4 });
      g.circle(tx, ty, 40).stroke({ color: 0xff8800, alpha: 0.35, width: 2 });
      // Inner ring
      g.circle(tx, ty, 18).fill({ color: 0x1a1a1a, alpha: 0.5 });
      g.circle(tx, ty, 18).stroke({ color: 0xff6600, alpha: 0.3, width: 1 });
      // Center bolt
      g.circle(tx, ty, 5).fill({ color: 0xff8800, alpha: 0.5 });
    }
    // Connecting pipes between turbines
    g.rect(x + 140, y + h / 2 + 14, w - 280, 12).fill({ color: 0x555555, alpha: 0.3 });
    g.rect(x + 140, y + h / 2 + 14, w - 280, 12).stroke({
      color: 0xff8800,
      alpha: 0.2,
      width: 1,
    });
  }

  private drawStorage(
    g: Graphics,
    x: number,
    y: number,
    w: number,
    h: number
  ) {
    // Stacked crates
    const crates = [
      { cx: 50, cy: 70, cw: 50, ch: 40 },
      { cx: 50, cy: 120, cw: 50, ch: 40 },
      { cx: 110, cy: 120, cw: 45, ch: 40 },
      { cx: w - 100, cy: h - 80, cw: 55, ch: 45 },
      { cx: w - 100, cy: h - 130, cw: 55, ch: 45 },
      { cx: w - 160, cy: h - 80, cw: 50, ch: 45 },
    ];
    for (const c of crates) {
      g.roundRect(x + c.cx, y + c.cy, c.cw, c.ch, 3).fill({
        color: 0xaa8844,
        alpha: 0.35,
      });
      g.roundRect(x + c.cx, y + c.cy, c.cw, c.ch, 3).stroke({
        color: 0xddaa55,
        alpha: 0.3,
        width: 1,
      });
      // Tape/band across crate
      g.rect(x + c.cx + 4, y + c.cy + c.ch / 2 - 2, c.cw - 8, 4).fill({
        color: 0xddaa55,
        alpha: 0.2,
      });
    }
  }

  private drawReactor(g: Graphics, cx: number, cy: number) {
    cy += 20; // shift down from room label
    // Outer containment ring
    g.circle(cx, cy, 50).fill({ color: 0x1a0000, alpha: 0.4 });
    g.circle(cx, cy, 50).stroke({ color: 0xff2222, alpha: 0.35, width: 2 });
    // Inner core ring
    g.circle(cx, cy, 28).fill({ color: 0x330000, alpha: 0.5 });
    g.circle(cx, cy, 28).stroke({ color: 0xff4444, alpha: 0.4, width: 1.5 });
    // Glowing core
    g.circle(cx, cy, 12).fill({ color: 0xff3300, alpha: 0.6 });
    // Conduit pipes radiating outward
    for (const angle of [0, Math.PI / 2, Math.PI, (3 * Math.PI) / 2]) {
      const x1 = cx + Math.cos(angle) * 52;
      const y1 = cy + Math.sin(angle) * 52;
      const x2 = cx + Math.cos(angle) * 85;
      const y2 = cy + Math.sin(angle) * 85;
      g.moveTo(x1, y1).lineTo(x2, y2).stroke({ color: 0xff4444, alpha: 0.25, width: 4 });
    }
  }
}
