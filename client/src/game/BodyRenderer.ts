import { Container, Graphics } from "pixi.js";
import { COLOR_HEX, type PlayerColor } from "@/lib/protocol";

const BODY_RADIUS = 16;

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
          s.container.destroy();
        }
        this.sprites.delete(id);
      }
    }
  }

  private createBody(bodyId: string): BodySprite {
    const c = new Container();

    const colorKey = this.colors[bodyId];
    const hex = colorKey
      ? parseInt(COLOR_HEX[colorKey].replace("#", ""), 16)
      : 0x888888;

    // Body — slumped/horizontal look (oval) with ice tint
    const body = new Graphics();
    body.ellipse(0, 0, BODY_RADIUS + 4, BODY_RADIUS - 2);
    body.fill(hex);
    body.ellipse(0, 0, BODY_RADIUS + 4, BODY_RADIUS - 2);
    body.stroke({ color: 0x000000, width: 2 });
    c.addChild(body);

    // Ice overlay
    const ice = new Graphics();
    ice.ellipse(0, 0, BODY_RADIUS + 6, BODY_RADIUS);
    ice.fill({ color: 0x93c5fd, alpha: 0.55 });
    c.addChild(ice);

    // Snowflake icon on top
    const snowflake = new Graphics();
    snowflake.circle(0, -2, 3);
    snowflake.fill(0xffffff);
    c.addChild(snowflake);

    this.container.addChild(c);

    return { container: c, bodyId };
  }
}
