import { Container, Graphics, Sprite, Text, Texture, Rectangle } from "pixi.js";
import type { PlayerInfo } from "./Engine";
import type { PlayerColor } from "@/lib/protocol";

const PLAYER_RADIUS = 16;
const LERP_SPEED = 0.2;
const VISION_RADIUS = 250;

// Spritesheet layout: 13 cols x 6 rows, 32x32 per frame
// Cols: Down(0-2), Up(3-5), Left(6-8), Right(9-11), Frozen(12)
// Rows: red, blue, green, yellow, purple, orange
const FRAME_SIZE = 32;
const COLOR_ROW: Record<PlayerColor, number> = {
  red: 0,
  blue: 1,
  green: 2,
  yellow: 3,
  purple: 4,
  orange: 5,
};

// Which columns have art: Down1=0, Up2=4, Up3=5
const COL_FRONT = 0; // front-facing (Down frame 1)
const COL_BACK = 4;  // back-facing (Up frame 2)

type Direction = "down" | "up" | "left" | "right";

interface PlayerSpriteData {
  container: Container;
  sprite: Sprite;
  shadow: Graphics;
  nameLabel: Text;
  color: PlayerColor;
  currentX: number;
  currentY: number;
  targetX: number;
  targetY: number;
  frozen: boolean;
  bobPhase: number;
  prevX: number;
  prevY: number;
  direction: Direction;
  frontTexture: Texture;
  backTexture: Texture;
}

// Cached textures per color, created once from the spritesheet
let textureCache: Map<PlayerColor, { front: Texture; back: Texture }> | null = null;

function getTextures(baseTexture: Texture): Map<PlayerColor, { front: Texture; back: Texture }> {
  if (textureCache) return textureCache;
  textureCache = new Map();
  for (const [color, row] of Object.entries(COLOR_ROW) as [PlayerColor, number][]) {
    const front = new Texture({
      source: baseTexture.source,
      frame: new Rectangle(COL_FRONT * FRAME_SIZE, row * FRAME_SIZE, FRAME_SIZE, FRAME_SIZE),
    });
    const back = new Texture({
      source: baseTexture.source,
      frame: new Rectangle(COL_BACK * FRAME_SIZE, row * FRAME_SIZE, FRAME_SIZE, FRAME_SIZE),
    });
    textureCache.set(color, { front, back });
  }
  return textureCache;
}

export class PlayerManager {
  container: Container;
  private sprites: Map<string, PlayerSpriteData> = new Map();
  private localPlayerId: string = "";
  private sheetTexture: Texture | null = null;

  constructor(players: PlayerInfo[]) {
    this.container = new Container();

    // Load spritesheet
    const tex = Texture.from("/sprites.png");
    this.sheetTexture = tex;

    for (const p of players) {
      this.createSprite(p, tex);
    }
  }

  private createSprite(info: PlayerInfo, sheetTex: Texture) {
    const playerContainer = new Container();
    const textures = getTextures(sheetTex);
    const colorTextures = textures.get(info.color)!;

    // Shadow ellipse under the character
    const shadow = new Graphics();
    shadow.ellipse(0, 10, 14, 6).fill({ color: 0x000000, alpha: 0.35 });
    playerContainer.addChild(shadow);

    // Character sprite
    const sprite = new Sprite(colorTextures.front);
    sprite.anchor.set(0.5);
    // Scale: the sprite is 32x32 pixels, player radius is 16 game units
    // so we want the sprite to appear ~32x32 in game space (no extra scaling)
    playerContainer.addChild(sprite);

    // Name label
    const nameLabel = new Text({
      text: info.name,
      style: {
        fontFamily: "Arial",
        fontSize: 12,
        fontWeight: "bold",
        fill: 0xffffff,
        align: "center",
        dropShadow: {
          color: 0x000000,
          blur: 3,
          distance: 0,
          alpha: 0.8,
        },
      },
    });
    nameLabel.anchor.set(0.5);
    nameLabel.y = -PLAYER_RADIUS - 10;
    playerContainer.addChild(nameLabel);

    this.container.addChild(playerContainer);

    this.sprites.set(info.id, {
      container: playerContainer,
      sprite,
      shadow,
      nameLabel,
      color: info.color,
      currentX: 0,
      currentY: 0,
      targetX: 0,
      targetY: 0,
      frozen: false,
      bobPhase: 0,
      prevX: 0,
      prevY: 0,
      direction: "down",
      frontTexture: colorTextures.front,
      backTexture: colorTextures.back,
    });
  }

  setTargetPositions(positions: Record<string, { x: number; y: number }>) {
    for (const [id, pos] of Object.entries(positions)) {
      const sprite = this.sprites.get(id);
      if (sprite) {
        sprite.targetX = pos.x;
        sprite.targetY = pos.y;
      }
    }
  }

  setFrozen(frozenIds: Set<string>) {
    for (const [id, sprite] of this.sprites) {
      const isFrozen = frozenIds.has(id);
      if (isFrozen !== sprite.frozen) {
        sprite.frozen = isFrozen;
        sprite.nameLabel.alpha = isFrozen ? 0.4 : 1.0;
        sprite.container.scale.set(isFrozen ? 0.7 : 1.0);
      }
    }
  }

  setLocalPlayer(id: string) {
    this.localPlayerId = id;
  }

  update() {
    const local = this.sprites.get(this.localPlayerId);
    const localX = local?.currentX ?? 0;
    const localY = local?.currentY ?? 0;
    const localIsGhost = local?.frozen ?? false;

    for (const [id, sprite] of this.sprites) {
      // Lerp toward target
      sprite.currentX += (sprite.targetX - sprite.currentX) * LERP_SPEED;
      sprite.currentY += (sprite.targetY - sprite.currentY) * LERP_SPEED;

      sprite.container.x = sprite.currentX;
      sprite.container.y = sprite.currentY;

      // Movement direction — pick sprite based on which way we're moving
      const movedX = sprite.currentX - sprite.prevX;
      const movedY = sprite.currentY - sprite.prevY;
      const speed = Math.sqrt(movedX * movedX + movedY * movedY);

      if (speed > 0.3) {
        // Determine dominant direction
        if (Math.abs(movedY) > Math.abs(movedX)) {
          sprite.direction = movedY < 0 ? "up" : "down";
        } else {
          sprite.direction = movedX < 0 ? "left" : "right";
        }
      }

      // Set texture and flip based on direction
      switch (sprite.direction) {
        case "down":
          sprite.sprite.texture = sprite.frontTexture;
          sprite.sprite.scale.x = 1;
          break;
        case "up":
          sprite.sprite.texture = sprite.backTexture;
          sprite.sprite.scale.x = 1;
          break;
        case "left":
          // Use front texture, flipped horizontally
          sprite.sprite.texture = sprite.frontTexture;
          sprite.sprite.scale.x = -1;
          break;
        case "right":
          // Use front texture, normal
          sprite.sprite.texture = sprite.frontTexture;
          sprite.sprite.scale.x = 1;
          break;
      }

      // Walking bob — only when moving and not frozen
      sprite.prevX = sprite.currentX;
      sprite.prevY = sprite.currentY;
      if (!sprite.frozen && speed > 0.3) {
        sprite.bobPhase += 0.35;
        const bob = Math.sin(sprite.bobPhase) * 2;
        sprite.sprite.y = bob;
      } else {
        sprite.bobPhase = 0;
        sprite.sprite.y = 0;
      }

      if (id === this.localPlayerId) {
        sprite.container.visible = true;
        sprite.container.alpha = sprite.frozen ? 0.5 : 1.0;
        continue;
      }

      // Visibility rules (unchanged)
      if (sprite.frozen) {
        sprite.container.visible = localIsGhost;
        sprite.container.alpha = 0.4;
      } else {
        if (localIsGhost) {
          sprite.container.visible = true;
          sprite.container.alpha = 1.0;
        } else {
          const dx = sprite.currentX - localX;
          const dy = sprite.currentY - localY;
          const dist = Math.sqrt(dx * dx + dy * dy);
          sprite.container.visible = dist <= VISION_RADIUS;
          sprite.container.alpha = 1.0;
        }
      }
    }
  }

  getPosition(id: string): { x: number; y: number } | null {
    const sprite = this.sprites.get(id);
    if (!sprite) return null;
    return { x: sprite.currentX, y: sprite.currentY };
  }

  getNearestAlivePlayer(
    x: number,
    y: number,
    range: number,
    excludeId: string,
    frozenIds: Set<string>
  ): string | null {
    let nearestId: string | null = null;
    let nearestDistSq = range * range;
    for (const [id, sprite] of this.sprites) {
      if (id === excludeId) continue;
      if (frozenIds.has(id)) continue;
      const dx = sprite.currentX - x;
      const dy = sprite.currentY - y;
      const distSq = dx * dx + dy * dy;
      if (distSq <= nearestDistSq) {
        nearestDistSq = distSq;
        nearestId = id;
      }
    }
    return nearestId;
  }

  getNearestFrozen(
    x: number,
    y: number,
    range: number,
    frozenIds: Set<string>
  ): string | null {
    let nearestId: string | null = null;
    let nearestDistSq = range * range;
    for (const [id, sprite] of this.sprites) {
      if (!frozenIds.has(id)) continue;
      const dx = sprite.currentX - x;
      const dy = sprite.currentY - y;
      const distSq = dx * dx + dy * dy;
      if (distSq <= nearestDistSq) {
        nearestDistSq = distSq;
        nearestId = id;
      }
    }
    return nearestId;
  }
}
