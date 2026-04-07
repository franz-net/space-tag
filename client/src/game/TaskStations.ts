import { Container, Graphics, Text } from "pixi.js";
import type { PlayerTaskInfo } from "@/lib/protocol";

const STATION_RADIUS = 14;
const INTERACTION_RANGE = 60;

const TASK_ICONS: Record<string, string> = {
  tap_targets: "⭐",
  connect_wires: "🔌",
  match_colors: "🎴",
  simon_says: "🎵",
};

interface StationSprite {
  container: Container;
  glow: Graphics;
  stationId: string;
  x: number;
  y: number;
  isMyTask: boolean;
  completed: boolean;
}

export class TaskStationsRenderer {
  container: Container;
  private stations: StationSprite[] = [];
  private pulseTime = 0;

  constructor() {
    this.container = new Container();
  }

  setup(tasks: PlayerTaskInfo[]) {
    // Clear existing
    this.container.removeChildren();
    this.stations = [];

    for (const task of tasks) {
      const stationContainer = new Container();
      stationContainer.x = task.position.x;
      stationContainer.y = task.position.y;

      // Glow ring (pulsing for active tasks)
      const glow = new Graphics();
      glow.circle(0, 0, STATION_RADIUS + 6);
      glow.fill({ color: 0xfbbf24, alpha: 0.3 });
      stationContainer.addChild(glow);

      // Station body
      const body = new Graphics();
      body.circle(0, 0, STATION_RADIUS);
      body.fill(task.completed ? 0x4b5563 : 0xfbbf24);
      body.circle(0, 0, STATION_RADIUS);
      body.stroke({ color: 0x000000, width: 2 });
      stationContainer.addChild(body);

      // Icon
      const icon = new Text({
        text: task.completed ? "✅" : (TASK_ICONS[task.type] || "❓"),
        style: { fontSize: 14 },
      });
      icon.anchor.set(0.5);
      stationContainer.addChild(icon);

      this.container.addChild(stationContainer);

      this.stations.push({
        container: stationContainer,
        glow,
        stationId: task.stationId,
        x: task.position.x,
        y: task.position.y,
        isMyTask: true,
        completed: task.completed,
      });
    }
  }

  updateTasks(tasks: PlayerTaskInfo[]) {
    for (const task of tasks) {
      const station = this.stations.find((s) => s.stationId === task.stationId);
      if (station && task.completed && !station.completed) {
        station.completed = true;
        // Update visuals
        station.container.removeChildren();

        const body = new Graphics();
        body.circle(0, 0, STATION_RADIUS);
        body.fill(0x4b5563);
        body.circle(0, 0, STATION_RADIUS);
        body.stroke({ color: 0x000000, width: 2 });
        station.container.addChild(body);

        const icon = new Text({
          text: "✅",
          style: { fontSize: 14 },
        });
        icon.anchor.set(0.5);
        station.container.addChild(icon);

        station.glow = new Graphics(); // no glow
      }
    }
  }

  update() {
    // Pulse glow on incomplete stations
    this.pulseTime += 0.05;
    const alpha = 0.2 + Math.sin(this.pulseTime) * 0.15;

    for (const station of this.stations) {
      if (!station.completed && station.glow.parent) {
        station.glow.clear();
        station.glow.circle(0, 0, STATION_RADIUS + 6);
        station.glow.fill({ color: 0xfbbf24, alpha });
      }
    }
  }

  /** Returns the station ID if the player is near an incomplete task, else null */
  getNearbyTask(
    playerX: number,
    playerY: number
  ): string | null {
    for (const station of this.stations) {
      if (station.completed) continue;
      const dx = playerX - station.x;
      const dy = playerY - station.y;
      if (dx * dx + dy * dy <= INTERACTION_RANGE * INTERACTION_RANGE) {
        return station.stationId;
      }
    }
    return null;
  }
}
