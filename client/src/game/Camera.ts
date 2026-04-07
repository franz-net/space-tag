import type { Container } from "pixi.js";

export class Camera {
  private container: Container;
  private screenW: number;
  private screenH: number;
  private mapW: number;
  private mapH: number;

  // Current camera offset (smoothed)
  x = 0;
  y = 0;

  constructor(
    container: Container,
    screenW: number,
    screenH: number,
    mapW: number,
    mapH: number
  ) {
    this.container = container;
    this.screenW = screenW;
    this.screenH = screenH;
    this.mapW = mapW;
    this.mapH = mapH;
  }

  follow(targetX: number, targetY: number) {
    // Center the target on screen
    let camX = this.screenW / 2 - targetX;
    let camY = this.screenH / 2 - targetY;

    // Clamp to map bounds
    camX = Math.min(0, Math.max(this.screenW - this.mapW, camX));
    camY = Math.min(0, Math.max(this.screenH - this.mapH, camY));

    // Smooth follow
    this.x += (camX - this.x) * 0.15;
    this.y += (camY - this.y) * 0.15;

    this.container.x = this.x;
    this.container.y = this.y;
  }

  resize(width: number, height: number) {
    this.screenW = width;
    this.screenH = height;
  }
}
