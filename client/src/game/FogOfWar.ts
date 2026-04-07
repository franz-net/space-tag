import { Graphics } from "pixi.js";

const VISION_RADIUS = 250;

export class FogOfWar {
  // A Graphics object used as a mask on the ship layer.
  // Only the area inside the vision circle is visible.
  mask: Graphics;

  constructor() {
    this.mask = new Graphics();
  }

  update(playerX: number, playerY: number) {
    this.mask.clear();
    this.mask.circle(playerX, playerY, VISION_RADIUS);
    this.mask.fill(0xffffff);
  }
}
