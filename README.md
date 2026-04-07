# SpaceTag - A Kid-Friendly Social Deduction Game

A multiplayer social deduction game inspired by Among Us, designed for kids ages 5-8. No free-text chat — only pre-defined icon messages. AI players fill in when friends aren't available.

## Concept

Players explore a spaceship, complete tasks, and try to figure out who the secret "tagger" is — all with bright colors, friendly animations, and zero toxicity.

- **Crewmates** complete simple mini-tasks around the ship
- **The Tagger** (impostor) secretly "freezes" crewmates
- **Everyone** votes using icon-based quick chat to find the tagger
- **AI bots** fill empty player slots so kids can always play

## Tech Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| Frontend | Next.js + React + PixiJS | UI, menus, 2D game rendering |
| Backend | Go | WebSocket server, rooms, game logic, AI |
| Protocol | WebSockets (JSON) | Real-time communication |
| State | In-memory (Go maps) | No database needed |

## Project Structure

```
gameX/
├── server/           # Go backend
│   ├── main.go       # HTTP + WebSocket server
│   ├── hub.go        # Connection registry
│   ├── client.go     # Per-connection read/write pumps
│   ├── room.go       # Room lifecycle
│   ├── game.go       # Game state machine + loop
│   ├── player.go     # Player types + roles
│   ├── tasks.go      # Task definitions + progress
│   ├── voting.go     # Meeting + vote logic
│   ├── ai.go         # AI player behavior
│   ├── map.go        # Map topology
│   ├── messages.go   # Protocol message types
│   └── collision.go  # Collision detection
├── client/           # Next.js frontend
│   ├── src/
│   │   ├── app/          # Pages (home, lobby, game)
│   │   ├── components/   # React UI components
│   │   ├── game/         # PixiJS game engine
│   │   ├── hooks/        # WebSocket, input, state
│   │   ├── lib/          # Protocol types, constants
│   │   └── stores/       # Zustand state
│   └── public/assets/    # Sprites, icons, sounds
└── README.md
```

## Design Principles

- **Icons over text** — 5-year-olds can't read much, so use colors, shapes, and pictures
- **No violence** — the impostor "freezes" players (ice/snowflake theme), not kills
- **No free chat** — only pre-defined icon messages during voting
- **Simple controls** — WASD/arrows or virtual joystick for mobile
- **Bright & friendly** — pastel colors, cheerful sounds, encouraging feedback
- **Color-blind safe** — each player color has a unique shape badge

## Game Flow

1. **Lobby** — Create/join room with 4-letter code, wait for players (4-6)
2. **Roles** — 1 tagger, rest are crewmates (assigned secretly)
3. **Play** — Crewmates do tasks, tagger freezes players
4. **Report** — Find a frozen player or hit emergency button → meeting
5. **Vote** — Send icon messages, vote to eject someone
6. **Win/Lose** — Crewmates win by finishing tasks or ejecting tagger; tagger wins by freezing enough crewmates

---

## Implementation Phases

### Phase 1: Project Setup + Networking Foundation ⬜
> Goal: WebSocket connection, room system, lobby UI

- [ ] Go server with WebSocket support (gorilla/websocket)
- [ ] Message protocol (JSON envelopes with type + payload)
- [ ] Room system — create/join with 4-letter codes, max 6 players
- [ ] Hub for managing connections and routing messages to rooms
- [ ] Next.js client scaffolding (App Router, Tailwind, Zustand)
- [ ] Landing page — name input, create room, join with code
- [ ] Lobby page — player list with colored avatars, room code display
- [ ] WebSocket hook with auto-reconnect
- [ ] Host can start the game (2+ players required)
- [ ] Role assignment on game start (1 tagger, rest crewmates)

### Phase 2: Game Map + Movement ⬜
> Goal: 2D spaceship map, player movement, real-time sync

- [ ] Map topology — 6 rooms (Cafeteria, Engine, Navigation, Medbay, Reactor, Storage)
- [ ] PixiJS game engine setup with canvas mounting
- [ ] Map rendering — colored rooms, hallways, room labels with icons
- [ ] Player sprites — colored circles with name labels
- [ ] Movement system — client sends intent, server validates + broadcasts positions
- [ ] Server game loop at 20Hz for position broadcasting
- [ ] Camera system — viewport follows local player
- [ ] Fog of war — circular vision around player
- [ ] Collision detection — circle vs AABB, wall sliding
- [ ] Input — keyboard (WASD/arrows) + virtual joystick for mobile

### Phase 3: Tasks System ⬜
> Goal: Mini-task games, progress tracking, crew win condition

- [ ] Task stations placed around the map (8-10 stations, 1-2 per room)
- [ ] Task assignment — each crewmate gets 3-4 random tasks
- [ ] HUD — task progress bar, USE button, role badge
- [ ] Task interaction — USE button activates when near a station
- [ ] Mini-game: Tap Targets — tap 5 appearing stars
- [ ] Mini-game: Connect Wires — drag wires to matching colors
- [ ] Mini-game: Match Colors — 2x3 memory card matching
- [ ] Mini-game: Simon Says — repeat a 3-color sequence
- [ ] Task completion validation (server-side)
- [ ] Win condition: all crew tasks completed → crewmates win

### Phase 4: Tagger Mechanics + Voting ⬜
> Goal: Freeze mechanic, meetings, icon-based voting, win/lose

- [ ] TAG button for tagger (active when near a player, 25s cooldown)
- [ ] Freeze animation — snowflake burst, ice-blue body stays on map
- [ ] Body discovery — REPORT button when near a frozen player
- [ ] Emergency meeting button in Cafeteria (1 use per player per game)
- [ ] Meeting screen — all players teleported to Cafeteria
- [ ] Pre-defined messages — icon grid (accusation, defense, evidence, reactions)
- [ ] Discussion phase (30s) + voting phase (15s)
- [ ] Vote tallying — most votes = ejected, ties = no ejection
- [ ] Ejection animation — player floats away
- [ ] Win conditions: tagger ejected → crew wins; living tagger >= living crew → tagger wins
- [ ] Game over screen — roles revealed, play again button

### Phase 5: AI Players ⬜
> Goal: AI bots fill empty slots, playable solo with bots

- [ ] Add AI button in lobby + auto-fill to minimum 4 players
- [ ] AI names from kid-friendly list (Astro, Cosmo, Nova, Pixel, etc.)
- [ ] Waypoint pathfinding on map graph (BFS, ~20 nodes)
- [ ] AI crewmate — navigate to tasks, "complete" them, report bodies
- [ ] AI tagger — fake tasks, hunt isolated players, freeze when unwatched
- [ ] AI voting — follow accusations, send 0-1 icon messages
- [ ] Difficulty settings (easy/normal)
- [ ] Human-like delays (0.5-1.5s reaction times, movement variation)

### Phase 6: Polish ⬜
> Goal: Sound, animation, tutorial, mobile, accessibility

- [ ] Sound effects — footsteps, chimes, freeze poof, meeting bell, win/lose
- [ ] Animations — player bobbing, task station glow, confetti, countdown
- [ ] Tutorial — 5 illustrated slides for first-time players
- [ ] Mobile responsive — landscape prompt, touch-friendly buttons (48px+)
- [ ] Accessibility — aria labels, color-blind shape badges, sound toggle
- [ ] Settings — sound, music, tutorial replay, name change
- [ ] Room lighting — different ambient colors per room
- [ ] Starfield background through ship windows

---

## Running Locally

```bash
# Terminal 1: Go server
cd server
go run .
# Serves on :8080

# Terminal 2: Next.js client
cd client
npm install
npm run dev
# Serves on :3000, proxies WebSocket to :8080
```

Open multiple browser tabs to test multiplayer locally.

## Requirements

- Go 1.22+
- Node.js 18+
- npm or yarn
