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
}
