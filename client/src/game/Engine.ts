import { Application, Container } from "pixi.js";
import { MapRenderer, type GameMapData } from "./MapRenderer";
import { PlayerManager } from "./PlayerManager";
import { Camera } from "./Camera";
import { FogOfWar } from "./FogOfWar";
import { InputHandler } from "./InputHandler";
import { TaskStationsRenderer } from "./TaskStations";
import { BodyRenderer } from "./BodyRenderer";
import type { PlayerColor, PlayerTaskInfo } from "@/lib/protocol";

export interface PlayerInfo {
  id: string;
  color: PlayerColor;
  name: string;
}

export class Engine {
  app: Application;
  mapRenderer!: MapRenderer;
  playerManager!: PlayerManager;
  camera!: Camera;
  fogOfWar!: FogOfWar;
  input!: InputHandler;
  taskStations!: TaskStationsRenderer;
  bodyRenderer!: BodyRenderer;

  private gameContainer!: Container;
  private bgContainer!: Container;
  private shipContainer!: Container;
  private localPlayerId: string = "";
  private onMove: (dx: number, dy: number) => void;
  private lastInputSent = 0;
  private initialized = false;

  // Exposed for HUD to read
  nearTaskId: string | null = null;
  nearTagTargetId: string | null = null;
  nearBodyId: string | null = null;
  inCafeteria = false;
  private frozenIds: Set<string> = new Set();
  private myRole: "crewmate" | "tagger" | null = null;

  constructor(onMove: (dx: number, dy: number) => void) {
    this.app = new Application();
    this.onMove = onMove;
  }

  async init(canvas: HTMLCanvasElement, width: number, height: number) {
    await this.app.init({
      canvas,
      width,
      height,
      backgroundColor: 0x0a0a1a,
      antialias: true,
      autoDensity: true,
      resolution: window.devicePixelRatio || 1,
    });

    this.gameContainer = new Container();
    this.app.stage.addChild(this.gameContainer);

    // Background layer: space + stars (always visible)
    this.bgContainer = new Container();
    this.gameContainer.addChild(this.bgContainer);

    // Ship layer: rooms, hallways, players (masked by vision circle)
    this.shipContainer = new Container();
    this.gameContainer.addChild(this.shipContainer);

    this.input = new InputHandler();
  }

  setupMap(mapData: GameMapData, players: PlayerInfo[], localPlayerId: string) {
    this.localPlayerId = localPlayerId;

    // Cache cafeteria bounds for emergency button check
    const caf = mapData.rooms.find((r) => r.id === "cafeteria");
    if (caf) {
      this.cafBounds = caf.bounds;
    }

    // Render map
    this.mapRenderer = new MapRenderer(mapData);
    this.bgContainer.addChild(this.mapRenderer.backgroundContainer);
    this.shipContainer.addChild(this.mapRenderer.shipContainer);

    // Task stations (rendered on ship layer, between map and players)
    this.taskStations = new TaskStationsRenderer();
    this.shipContainer.addChild(this.taskStations.container);

    // Bodies (frozen players) — visible to everyone
    const colors: Record<string, PlayerColor> = {};
    for (const p of players) {
      colors[p.id] = p.color;
    }
    this.bodyRenderer = new BodyRenderer(colors);
    this.shipContainer.addChild(this.bodyRenderer.container);

    // Create player sprites
    this.playerManager = new PlayerManager(players);
    this.playerManager.setLocalPlayer(localPlayerId);
    this.shipContainer.addChild(this.playerManager.container);

    // Camera follows local player
    this.camera = new Camera(
      this.gameContainer,
      this.app.screen.width,
      this.app.screen.height,
      mapData.width,
      mapData.height
    );

    // Fog of war
    this.fogOfWar = new FogOfWar();
    this.shipContainer.addChild(this.fogOfWar.mask);
    this.shipContainer.mask = this.fogOfWar.mask;

    // Start game loop
    this.app.ticker.add(() => this.update());
  }

  setupTasks(tasks: PlayerTaskInfo[]) {
    if (this.taskStations) {
      this.taskStations.setup(tasks);
    }
  }

  updateTasks(tasks: PlayerTaskInfo[]) {
    if (this.taskStations) {
      this.taskStations.updateTasks(tasks);
    }
  }

  private update() {
    if (!this.playerManager) return;

    const dir = this.input.getDirection();

    // Send input to server at ~15Hz
    const now = performance.now();
    if (now - this.lastInputSent > 66) {
      this.onMove(dir.x, dir.y);
      this.lastInputSent = now;
    }

    // Interpolate player positions
    this.playerManager.update();

    // Animate task station glow
    if (this.taskStations) {
      this.taskStations.update();
    }

    // Update camera to follow local player
    const localPos = this.playerManager.getPosition(this.localPlayerId);
    if (localPos) {
      this.camera.follow(localPos.x, localPos.y);
      this.fogOfWar.update(localPos.x, localPos.y);

      // Check proximity to task stations
      this.nearTaskId =
        this.taskStations?.getNearbyTask(localPos.x, localPos.y) ?? null;

      // Tagger: nearest alive player to tag
      if (this.myRole === "tagger") {
        this.nearTagTargetId = this.playerManager.getNearestAlivePlayer(
          localPos.x,
          localPos.y,
          70,
          this.localPlayerId,
          this.frozenIds
        );
      } else {
        this.nearTagTargetId = null;
      }

      // Anyone alive: nearest body (at body position, not ghost position)
      // Ghosts can't report bodies
      const amGhost = this.frozenIds.has(this.localPlayerId);
      if (amGhost) {
        this.nearBodyId = null;
      } else if (this.bodyRenderer) {
        this.nearBodyId = this.findNearbyBody(localPos.x, localPos.y, 90);
      }

      // In cafeteria? (for emergency button)
      this.inCafeteria = this.isInCafeteria(localPos.x, localPos.y);
    }
  }

  private findNearbyBody(x: number, y: number, range: number): string | null {
    if (!this.bodyRenderer) return null;
    let nearestId: string | null = null;
    let nearestDistSq = range * range;
    for (const sprite of this.bodyRenderer.container.children) {
      // Each child is a body container
      const dx = sprite.x - x;
      const dy = sprite.y - y;
      const distSq = dx * dx + dy * dy;
      if (distSq <= nearestDistSq) {
        nearestDistSq = distSq;
        // Find the body ID by index
        const idx = this.bodyRenderer.container.children.indexOf(sprite);
        const ids = this.bodyRenderer.getBodyIds();
        nearestId = ids[idx] ?? null;
      }
    }
    return nearestId;
  }

  private isInCafeteria(x: number, y: number): boolean {
    if (!this.mapRenderer) return false;
    // Find cafeteria room from map data
    const room = this.cafBounds;
    if (!room) return false;
    return (
      x >= room.x &&
      x <= room.x + room.w &&
      y >= room.y &&
      y <= room.y + room.h
    );
  }

  private cafBounds: { x: number; y: number; w: number; h: number } | null =
    null;

  updatePositions(positions: Record<string, { x: number; y: number }>) {
    if (this.playerManager) {
      this.playerManager.setTargetPositions(positions);
    }
  }

  setBodies(bodies: Record<string, { x: number; y: number }>) {
    if (this.bodyRenderer) {
      this.bodyRenderer.setBodies(bodies);
    }
  }

  setFrozen(frozen: Set<string>) {
    this.frozenIds = frozen;
    if (this.playerManager) {
      this.playerManager.setFrozen(frozen);
    }

    // If local player is now a ghost, remove fog of war (full vision).
    // The mask Graphics must also be hidden — when used as a mask it's
    // invisible, but as soon as we detach it via mask=null it would
    // render as a white circle on the ship layer.
    const amGhost = frozen.has(this.localPlayerId);
    if (this.shipContainer && this.fogOfWar) {
      this.shipContainer.mask = amGhost ? null : this.fogOfWar.mask;
      this.fogOfWar.mask.visible = !amGhost;
    }
  }

  setMyRole(role: "crewmate" | "tagger") {
    this.myRole = role;
  }

  resize(width: number, height: number) {
    this.app.renderer.resize(width, height);
    if (this.camera) {
      this.camera.resize(width, height);
    }
  }

  markReady() {
    this.initialized = true;
  }

  destroy() {
    this.input?.destroy();
    if (this.initialized) {
      this.app.destroy(true, { children: true });
    }
  }
}
