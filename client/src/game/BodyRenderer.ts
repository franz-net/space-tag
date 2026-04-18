import { Container, Sprite, Texture, Rectangle } from "pixi.js";
import type { PlayerColor } from "@/lib/protocol";

const FRAME_SIZE = 32;
const COLOR_ROW: Record<PlayerColor, number> = {
  red: 0,
  blue: 1,
  green: 2,
  yellow: 3,
  purple: 4,
  orange: 5,
};
const FROZEN_COL = 12;

interface BodySprite {
  container: Container;
  bodyId: string;
}

interface PlayerColors {
  [id: string]: PlayerColor;
}

export class BodyRenderer {
  container: Container;
  private sprites: Map<string, BodySprite> = new Map();
  private colors: PlayerColors;

  constructor(playerColors: PlayerColors) {
    this.container = new Container();
    this.colors = playerColors;
  }

  getBodyIds(): string[] {
    // Returns IDs in same order as container children
    return Array.from(this.sprites.keys());
  }

  /** Set the bodies map. Adds new ones, removes old ones. */
  setBodies(bodies: Record<string, { x: number; y: number }>) {
    // Add or update
    for (const [bodyId, pos] of Object.entries(bodies)) {
      let sprite = this.sprites.get(bodyId);
      if (!sprite) {
        sprite = this.createBody(bodyId);
        this.sprites.set(bodyId, sprite);
      }
      sprite.container.x = pos.x;
      sprite.container.y = pos.y;
    }

    // Remove bodies that no longer exist
    for (const id of this.sprites.keys()) {
      if (!(id in bodies)) {
        const s = this.sprites.get(id);
        if (s) {
          this.container.removeChild(s.container);
          s.container.destroy({ children: true });
        }
        this.sprites.delete(id);
      }
    }
  }

  private createBody(bodyId: string): BodySprite {
    const c = new Container();

    const colorKey = this.colors[bodyId] ?? "red";
    const row = COLOR_ROW[colorKey];

    const sheetTex = Texture.from("/sprites.png");
    const frozenTex = new Texture({
      source: sheetTex.source,
      frame: new Rectangle(FROZEN_COL * FRAME_SIZE, row * FRAME_SIZE, FRAME_SIZE, FRAME_SIZE),
    });

    const sprite = new Sprite(frozenTex);
    sprite.anchor.set(0.5);
    c.addChild(sprite);

    this.container.addChild(c);

    return { container: c, bodyId };
  }
}
