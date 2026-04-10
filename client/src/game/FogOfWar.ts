import { Graphics } from "pixi.js";

const VISION_RADIUS = 250;
const REDUCED_VISION_RADIUS = 100;

export class FogOfWar {
  // A Graphics object used as a mask on the ship layer.
  // Only the area inside the vision circle is visible.
  mask: Graphics;
  reducedVision = false;

  constructor() {
    this.mask = new Graphics();
  }

  update(playerX: number, playerY: number) {
    const radius = this.reducedVision ? REDUCED_VISION_RADIUS : VISION_RADIUS;
    this.mask.clear();
    this.mask.circle(playerX, playerY, radius);
    this.mask.fill(0xffffff);
  }
}
