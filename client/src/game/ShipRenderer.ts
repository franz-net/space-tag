import {
  Assets,
  Container,
  Graphics,
  Rectangle,
  Sprite,
  Texture,
  TilingSprite,
  type Renderer,
} from "pixi.js";

// The ship is drawn from files exported out of Tiled by
// assets/maps/export.py into public/map/:
//   ship.json     where the floor chunks and furniture go (game units)
//   chunk-*.png   floor + walls baked into big images; windows and the open
//                 space around the hull are see-through
//   props.png     every piece of free-standing furniture on one sheet
//   space/*.png   planets and debris for the backdrop

export interface MapRect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface MapRoomData {
  id: string;
  label: string;
  bounds: MapRect;
  color: string;
}

/** Gameplay map sent by the server. The art comes from public/map. */
export interface GameMapData {
  rooms: MapRoomData[];
  width: number;
  height: number;
}

interface ShipArt {
  width: number;
  height: number;
  artScale: number;
  chunks: { src: string; x: number; y: number; w: number; h: number }[];
  atlas: string;
  frames: Record<string, [number, number, number, number]>;
  props: { name: string; x: number; y: number; flip: boolean }[];
}

const MAP_DIR = "/map/";
const SPACE_ART = ["planet-ringed", "moon", "asteroid", "satellite"];

let artPromise: Promise<ShipArt> | null = null;

/** Download the ship art once (kept between games). */
export function loadShipArt(): Promise<ShipArt> {
  if (!artPromise) {
    artPromise = (async () => {
      const art: ShipArt = await (await fetch(MAP_DIR + "ship.json")).json();
      const files = [
        ...art.chunks.map((c) => MAP_DIR + c.src),
        MAP_DIR + art.atlas,
        ...SPACE_ART.map((n) => `${MAP_DIR}space/${n}.png`),
      ];
      await Assets.load(files);
      for (const f of files) {
        // Crisp pixel art when scaled
        Assets.get<Texture>(f).source.scaleMode = "nearest";
      }
      return art;
    })().catch((err) => {
      artPromise = null; // let the next game retry
      throw err;
    });
  }
  return artPromise;
}

export class ShipRenderer {
  /** Space backdrop: lives outside the camera, scrolled with parallax */
  backgroundContainer: Container;
  /** Floor and walls: masked by fog of war */
  shipContainer: Container;
  /** Furniture, sorted by depth together with whatever else is added (players) */
  entities: Container;

  private farStars: TilingSprite;
  private nearStars: TilingSprite;
  private drifters: { sprite: Sprite; x: number; y: number; vx: number; vy: number; spin: number; depth: number }[] = [];
  private time = 0;
  private viewW: number;
  private viewH: number;

  constructor(art: ShipArt, renderer: Renderer, viewW: number, viewH: number) {
    this.viewW = viewW;
    this.viewH = viewH;

    this.backgroundContainer = new Container();
    this.farStars = new TilingSprite({ texture: starTexture(renderer, 512, 140, 0.6), width: viewW, height: viewH });
    this.nearStars = new TilingSprite({ texture: starTexture(renderer, 384, 45, 1.3), width: viewW, height: viewH });
    this.backgroundContainer.addChild(this.farStars);
    this.addDrifters();
    this.backgroundContainer.addChild(this.nearStars);

    this.shipContainer = new Container();
    const scale = 1 / art.artScale;
    for (const c of art.chunks) {
      const s = new Sprite(Assets.get<Texture>(MAP_DIR + c.src));
      s.position.set(c.x, c.y);
      s.scale.set(scale);
      this.shipContainer.addChild(s);
    }

    this.entities = new Container();
    this.entities.sortableChildren = true;
    const atlas = Assets.get<Texture>(MAP_DIR + art.atlas);
    const frames = new Map<string, Texture>();
    for (const p of art.props) {
      let tex = frames.get(p.name);
      if (!tex) {
        const [fx, fy, fw, fh] = art.frames[p.name];
        tex = new Texture({ source: atlas.source, frame: new Rectangle(fx, fy, fw, fh) });
        frames.set(p.name, tex);
      }
      const s = new Sprite(tex);
      s.anchor.set(0.5, 1); // bottom-centre = the front of the furniture
      s.position.set(p.x, p.y);
      s.scale.set(p.flip ? -scale : scale, scale);
      s.zIndex = p.y;
      this.entities.addChild(s);
    }
  }

  /** A few planets and bits of debris drifting slowly past the windows. */
  private addDrifters() {
    const spots = [
      { name: "planet-ringed", x: 0.18, y: 0.3, depth: 0.12, size: 2.2 },
      { name: "moon", x: 0.72, y: 0.65, depth: 0.18, size: 1.4 },
      { name: "asteroid", x: 0.45, y: 0.15, depth: 0.35, size: 1 },
      { name: "satellite", x: 0.9, y: 0.25, depth: 0.3, size: 1 },
      { name: "asteroid", x: 0.3, y: 0.85, depth: 0.4, size: 0.7 },
    ];
    for (const d of spots) {
      const sprite = new Sprite(Assets.get<Texture>(`${MAP_DIR}space/${d.name}.png`));
      sprite.anchor.set(0.5);
      sprite.scale.set(0.5 * d.size);
      this.backgroundContainer.addChild(sprite);
      this.drifters.push({
        sprite,
        x: d.x * this.viewW,
        y: d.y * this.viewH,
        vx: (Math.random() - 0.5) * 6,
        vy: (Math.random() - 0.5) * 3,
        spin: d.name === "planet-ringed" ? 0 : (Math.random() - 0.5) * 0.004,
        depth: d.depth,
      });
    }
  }

  /** Scroll the backdrop for the camera's position (camX/Y are the world offset). */
  update(camX: number, camY: number, dtSec: number) {
    this.time += dtSec;
    const drift = this.time * 4;
    this.farStars.tilePosition.set(camX * 0.05 - drift, camY * 0.05);
    this.nearStars.tilePosition.set(camX * 0.15 - drift * 2.5, camY * 0.15);

    // Drifters wrap around a band a bit larger than the screen
    const wrapW = this.viewW + 400;
    const wrapH = this.viewH + 400;
    for (const d of this.drifters) {
      d.x += d.vx * dtSec;
      d.y += d.vy * dtSec;
      d.sprite.rotation += d.spin;
      const px = (((d.x + camX * d.depth) % wrapW) + wrapW) % wrapW - 200;
      const py = (((d.y + camY * d.depth) % wrapH) + wrapH) % wrapH - 200;
      d.sprite.position.set(px, py);
    }
  }

  resize(viewW: number, viewH: number) {
    this.viewW = viewW;
    this.viewH = viewH;
    this.farStars.width = this.nearStars.width = viewW;
    this.farStars.height = this.nearStars.height = viewH;
  }
}

/** A tile of random stars (generated, so it costs no download). */
function starTexture(renderer: Renderer, size: number, count: number, maxSize: number): Texture {
  const g = new Graphics();
  g.rect(0, 0, size, size).fill({ color: 0x000000, alpha: 0 });
  for (let i = 0; i < count; i++) {
    const r = Math.random() * maxSize + 0.4;
    const tint = [0xffffff, 0xcfe8ff, 0xfff1c9][i % 3];
    g.circle(Math.random() * size, Math.random() * size, r).fill({ color: tint, alpha: Math.random() * 0.6 + 0.3 });
  }
  const tex = renderer.generateTexture({ target: g, frame: new Rectangle(0, 0, size, size) });
  g.destroy();
  return tex;
}
