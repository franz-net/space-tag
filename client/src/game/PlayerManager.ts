import { Container, Graphics, Text } from "pixi.js";
import type { PlayerInfo } from "./Engine";
import { COLOR_HEX } from "@/lib/protocol";

const PLAYER_RADIUS = 16;
const LERP_SPEED = 0.2;
const VISION_RADIUS = 250;

interface PlayerSprite {
  container: Container;
  body: Graphics;
  nameLabel: Text;
  iceOverlay: Graphics;
  color: string;
  currentX: number;
  currentY: number;
  targetX: number;
  targetY: number;
  frozen: boolean;
}

export class PlayerManager {
  container: Container;
  private sprites: Map<string, PlayerSprite> = new Map();
  private localPlayerId: string = "";

  constructor(players: PlayerInfo[]) {
    this.container = new Container();

    for (const p of players) {
      this.createSprite(p);
    }
  }

  private createSprite(info: PlayerInfo) {
    const playerContainer = new Container();

    // Body circle
    const body = new Graphics();
    const hex = parseInt(COLOR_HEX[info.color].replace("#", ""), 16);
    body.circle(0, 0, PLAYER_RADIUS);
    body.fill(hex);
    body.circle(0, 0, PLAYER_RADIUS);
    body.stroke({ color: 0x000000, width: 2 });
    playerContainer.addChild(body);

    // Ice overlay (hidden until frozen)
    const iceOverlay = new Graphics();
    iceOverlay.circle(0, 0, PLAYER_RADIUS + 4);
    iceOverlay.fill({ color: 0x93c5fd, alpha: 0.6 });
    iceOverlay.visible = false;
    playerContainer.addChild(iceOverlay);

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
      body,
      nameLabel,
      iceOverlay,
      color: COLOR_HEX[info.color],
      currentX: 0,
      currentY: 0,
      targetX: 0,
      targetY: 0,
      frozen: false,
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
        // Ghost look: pale/translucent name label, hide ice overlay (body is rendered separately)
        sprite.iceOverlay.visible = false;
        sprite.nameLabel.alpha = isFrozen ? 0.5 : 1.0;
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

      if (id === this.localPlayerId) {
        sprite.container.visible = true;
        sprite.container.alpha = sprite.frozen ? 0.6 : 1.0;
        continue;
      }

      // Other players' visibility rules:
      // - Living players see only living players within vision radius
      // - Ghost players see all living players AND all other ghosts (no fog)
      // - Living players cannot see ghosts (only bodies)
      if (sprite.frozen) {
        // This is a ghost — only ghosts can see them
        sprite.container.visible = localIsGhost;
        sprite.container.alpha = 0.5;
      } else {
        // Living player
        if (localIsGhost) {
          // Ghosts see everyone, no fog
          sprite.container.visible = true;
          sprite.container.alpha = 1.0;
        } else {
          // Living: vision radius
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

  /** Returns the player ID of the closest non-self alive player within range */
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

  /** Returns the ID of a frozen body within range */
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
