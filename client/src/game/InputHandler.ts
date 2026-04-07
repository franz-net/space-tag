export class InputHandler {
  private keys: Set<string> = new Set();
  private handleKeyDown: (e: KeyboardEvent) => void;
  private handleKeyUp: (e: KeyboardEvent) => void;
  private joystickDir: { x: number; y: number } = { x: 0, y: 0 };

  setJoystickDirection(x: number, y: number) {
    this.joystickDir.x = x;
    this.joystickDir.y = y;
  }

  constructor() {
    this.handleKeyDown = (e: KeyboardEvent) => {
      this.keys.add(e.key.toLowerCase());
    };

    this.handleKeyUp = (e: KeyboardEvent) => {
      this.keys.delete(e.key.toLowerCase());
    };

    window.addEventListener("keydown", this.handleKeyDown);
    window.addEventListener("keyup", this.handleKeyUp);
  }

  getDirection(): { x: number; y: number } {
    let dx = 0;
    let dy = 0;

    if (this.keys.has("w") || this.keys.has("arrowup")) dy -= 1;
    if (this.keys.has("s") || this.keys.has("arrowdown")) dy += 1;
    if (this.keys.has("a") || this.keys.has("arrowleft")) dx -= 1;
    if (this.keys.has("d") || this.keys.has("arrowright")) dx += 1;

    // Normalize diagonal movement
    if (dx !== 0 && dy !== 0) {
      const mag = Math.sqrt(dx * dx + dy * dy);
      dx /= mag;
      dy /= mag;
    }

    // Joystick takes precedence if active
    if (this.joystickDir.x !== 0 || this.joystickDir.y !== 0) {
      return { x: this.joystickDir.x, y: this.joystickDir.y };
    }

    return { x: dx, y: dy };
  }

  destroy() {
    window.removeEventListener("keydown", this.handleKeyDown);
    window.removeEventListener("keyup", this.handleKeyUp);
  }
}
