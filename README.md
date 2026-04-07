# SpaceTag

> A kid-friendly multiplayer social deduction game inspired by Among Us.

## Why this exists

My son wanted to play **Among Us**. It's a fantastic game — but as a parent, I had concerns.

The chat is unmoderated. Strangers say things to kids that I don't want my kid hearing. The "voting someone out" can get mean. Even the language ("kill", "dead body") felt off for a 6-year-old. And like a lot of free games, there are ads, microtransactions, and pressure to buy cosmetics.

I looked for a kid-friendly alternative and couldn't find one I was happy with. So I decided to build it.

**SpaceTag** is what I came up with: the same core social deduction loop that makes Among Us fun, but rebuilt from the ground up to be safe for young children:

- **No free-text chat.** Communication during meetings is limited to a fixed grid of icon messages ("I saw red", "trust me", "I was doing tasks"). No way for anyone to type anything inappropriate, ever.
- **No violence.** The impostor doesn't kill — they're the **Tagger**, and they **freeze** crewmates with ice. Frozen players become **ghosts** who can still wander the ship and (if they were crewmates) keep helping their team finish tasks.
- **AI bots.** When friends aren't around, the room auto-fills with friendly AI players so kids can always play.
- **No ads. No purchases. No accounts.** Just enter your name, share a 4-letter room code with a friend, and play.
- **Open source.** MIT licensed. Anyone can read the code, fork it, customize it for their own family, or contribute improvements.

It's a small project built for one specific kid, but if it's useful to other parents or to anyone learning game programming, that's a bonus.

---

## How the game works

Players spawn in a 2D top-down spaceship with 6 rooms (Cafeteria, Medbay, Navigation, Engine, Storage, Reactor). One player is secretly the **Tagger**; the rest are **Crewmates**.

- **Crewmates** wander the ship completing simple tap-and-drag mini-tasks (matching colors, connecting wires, tap targets, repeat-the-pattern). When all tasks are done, the crew wins.
- **The Tagger** pretends to do tasks while secretly freezing crewmates one at a time (with a cooldown so they can't spam).
- When someone finds a frozen friend, they hit **REPORT** and everyone gathers in the cafeteria for a **meeting**.
- During meetings, players use **icon messages** to share suspicions and then **vote** on who to send home. Tied or no votes means no one is ejected.
- Crew wins by **completing all tasks** OR **voting out the tagger**. Tagger wins if they freeze enough crewmates.

Frozen players become **ghosts**: they can move freely (no walls, no fog), see everything, and continue completing tasks — so getting tagged isn't a "you lose, sit out" punishment. Kids stay engaged.

## How to play

### 1. Get into a room
- Type your name on the home screen
- Click **Create Room** to start a new game, or enter a 4-letter code from a friend and click **Join**
- You'll see a lobby with everyone in the room
- The host can add bot players if you don't have enough friends

### 2. Start the game
- The host clicks **Start Game** (need at least 2 players)
- You'll see your secret role: **Crewmate** 💙 or **Tagger** 🔴
- Don't tell anyone what you are!

### 3. Move around the ship
- **Computer**: Use **WASD** or **arrow keys**
- **Phone/tablet**: Drag the **joystick** in the bottom-left corner

### 4. If you're a Crewmate
- Look at your **task list** (top-right) — it shows what to do and where
- Walk to a **yellow glowing station** in the room shown
- When you're close, the **USE** button lights up — tap it
- Solve the simple puzzle to complete the task
- **Watch out!** If everything goes dark, the Tagger is near...
- **Win**: complete all your team's tasks before the Tagger freezes everyone

### 5. If you're the Tagger
- Pretend you're doing tasks so no one suspects you
- When you're alone with a Crewmate, press the **TAG** button to freeze them
- The TAG button has a **cooldown** so you can't spam it
- Don't get caught!
- **Win**: freeze enough Crewmates so you outnumber them

### 6. Find a frozen friend?
- Walk up to the icy body
- The **REPORT** button will pulse blue — tap it
- Everyone teleports to the Cafeteria for a **meeting**

### 7. Need to call a meeting?
- Walk into the **Cafeteria**
- Tap the orange **🚨 EMERGENCY** button
- (You can only use this once per game!)

### 8. During meetings
- Look at the **icon messages** at the bottom
- Tap one to share what you think (like 🔴 "Red is sus" or 🤝 "I was with Blue")
- When voting time starts, tap a player's face to vote them out
- Or tap **⏭️ Skip** if you're not sure
- The player with the most votes goes home

### 9. If you get frozen
- You become a **ghost** ❄️
- You can fly anywhere — through walls, no fog of war
- **Crewmate ghosts can still do tasks** to help your team win!
- Other living players can't see you
- Just have fun watching the rest of the game

## Tech stack

| Layer | Technology | Purpose |
|---|---|---|
| Frontend | Next.js (App Router) + React + PixiJS v8 | UI, menus, 2D game canvas |
| State | Zustand | Client game state |
| Backend | Go + gorilla/websocket | WebSocket server, room management, game loop, AI |
| Protocol | JSON over WebSockets | Real-time, single connection per player |
| Storage | In-memory (Go maps) | No database — rooms vanish when empty |

The whole game runs from a single Go binary + a static Next.js build. No accounts, no database, no analytics.

## Project structure

```
space-tag/
├── server/                 # Go backend
│   ├── main.go             # HTTP + WebSocket entry point
│   ├── hub.go              # Connection registry
│   ├── client.go           # Per-connection read/write pumps + handlers
│   ├── room.go             # Room lifecycle, player slots
│   ├── game.go             # Game state, 20Hz tick loop, ghost movement
│   ├── gamemap.go          # Map topology (rooms, hallways, walkability)
│   ├── collision.go        # Circle-vs-AABB collision + wall sliding
│   ├── tasks.go            # Task stations, assignment, progress
│   ├── voting.go           # Meeting state, votes, pre-defined messages
│   ├── messages.go         # Protocol message types
│   ├── player.go           # Player + role types
│   └── errors.go
└── client/                 # Next.js frontend
    ├── src/
    │   ├── app/page.tsx              # Single-page entry, routes by game state
    │   ├── components/
    │   │   ├── HomeScreen.tsx        # Name + create/join room
    │   │   ├── Lobby.tsx             # Player list, bot controls, start
    │   │   ├── GameScreen.tsx        # PixiJS canvas + HUD overlays
    │   │   ├── HUD.tsx               # Tasks, action buttons, role badge
    │   │   ├── TaskOverlay.tsx       # Mini-game overlay
    │   │   ├── MeetingScreen.tsx     # Voting + quick chat
    │   │   ├── GameOverScreen.tsx
    │   │   ├── Joystick.tsx          # Mobile virtual joystick
    │   │   └── tasks/                # The 4 mini-games
    │   ├── game/                     # PixiJS engine
    │   │   ├── Engine.ts
    │   │   ├── MapRenderer.ts        # Rooms, hallways, starfield
    │   │   ├── PlayerManager.ts      # Player sprites, ghost visibility
    │   │   ├── BodyRenderer.ts       # Frozen body sprites
    │   │   ├── TaskStations.ts
    │   │   ├── Camera.ts
    │   │   ├── FogOfWar.ts
    │   │   └── InputHandler.ts
    │   ├── hooks/useWebSocket.ts
    │   ├── lib/protocol.ts           # Message types (mirrors server)
    │   └── stores/gameStore.ts
    └── public/
```

## Design principles

These rules drive every decision:

1. **Icons over text.** A 5-year-old can't read paragraph instructions. Color, shape, and pictures first.
2. **No violence language.** "Tag" not "kill". "Freeze" not "die". "Sent home" not "ejected". Frozen players are ghosts, not corpses.
3. **No free-text input anywhere.** The only text users type is their own name. Communication during meetings is a fixed icon grid validated server-side.
4. **Frustration-free.** Frozen players become ghosts who can keep playing. Disconnected players don't break the game. Soft player collisions (push apart) instead of getting stuck on each other.
5. **Touch and keyboard equally first-class.** Virtual joystick for mobile, WASD/arrows for desktop, drag-and-drop for tasks.
6. **Server-authoritative.** All game logic runs on the server. The client only sends inputs and renders snapshots — no way for a tampered client to cheat.

## Implementation phases

### Phase 1: Project Setup + Networking ✅
- [x] Go server with `gorilla/websocket`
- [x] JSON envelope protocol (`{type, payload}`)
- [x] Room system with 4-letter join codes (max 6 players)
- [x] Hub managing connections, routing to rooms
- [x] Next.js client (App Router, Tailwind, Zustand)
- [x] Single-page client with one persistent WebSocket connection
- [x] Landing page → lobby → game flow
- [x] Auto-reconnect on disconnect
- [x] Host can start the game (2+ players required)
- [x] Random tagger assignment

### Phase 2: Game Map + Movement ✅
- [x] 6-room spaceship with hallways (overlapping rects for smooth transitions)
- [x] PixiJS v8 game engine with layered containers
- [x] Server-side movement at 20Hz, client interpolation at 60fps
- [x] Camera follows local player
- [x] Fog of war (vision circle) — masks ship interior, leaves space visible
- [x] Wall-sliding collision
- [x] Soft player-to-player separation (no overlap, no stuck)
- [x] WASD/arrow keys + virtual joystick on small screens

### Phase 3: Tasks System ✅
- [x] 10 task stations across all 6 rooms
- [x] Each crewmate gets 4 random tasks
- [x] Task list visible in HUD with room hints
- [x] **Tap Targets** mini-game (tap appearing stars)
- [x] **Connect Wires** mini-game (drag-and-drop matching colors)
- [x] **Match Colors** mini-game (memory pairs)
- [x] **Simon Says** mini-game (repeat color sequence)
- [x] Server-side task validation (proximity check + completion tracking)
- [x] Crew win condition: all crewmate tasks complete

### Phase 4: Tagger Mechanics + Voting ✅
- [x] TAG button with 25s cooldown, 70px range
- [x] Frozen visual (ice-blue body sprite stays at freeze point)
- [x] **Ghost mode** for frozen players (free movement, full vision, can still complete tasks)
- [x] REPORT button when near a body
- [x] Emergency meeting button (1 use per player, cafeteria only)
- [x] Meeting screen with icon-based quick chat (14 messages, server-validated)
- [x] 30s discussion + 20s voting phases
- [x] Vote tallying (most votes = sent home, ties = no ejection)
- [x] Win conditions: crew wins by tasks/eject, tagger wins by outnumbering
- [x] Game over screen with roles revealed
- [x] Leave Game button + clean disconnect handling

### Phase 5: AI Players ⬜
- [ ] Auto-fill empty slots with AI bots
- [ ] Kid-friendly AI names (Astro, Cosmo, Nova, Pixel...)
- [ ] BFS pathfinding on map waypoint graph
- [ ] AI crewmate behavior — navigate to tasks, complete them, report bodies
- [ ] AI tagger behavior — fake tasks, hunt isolated players, freeze when unwatched
- [ ] AI voting (follows accusations, sends quick messages)
- [ ] Easy/normal difficulty
- [ ] Human-like delays and movement variation

### Phase 6: Polish ⬜
- [ ] Sound effects (footsteps, chimes, freeze, meeting bell, win/lose)
- [ ] Animations (player bobbing, station glow, confetti, countdown)
- [ ] Tutorial (illustrated slides for first-time players)
- [ ] Accessibility (aria labels, color-blind shape badges, sound toggle)
- [ ] Settings menu
- [ ] Room ambient lighting per area

---

## Running locally

You'll need **Go 1.22+** and **Node.js 18+**.

```bash
# Terminal 1 — Go server (port 8080)
cd server
go run .

# Terminal 2 — Next.js dev server (port 3000)
cd client
npm install
npm run dev
```

Open http://localhost:3000 in two browser tabs (or your phone on the same network) to test multiplayer locally. Create a room in one tab, share the 4-letter code, join in the other.

## Contributing

This is a personal project but contributions are welcome. If you have ideas — new mini-games, sound effects, art, AI improvements, translations, accessibility fixes — open an issue or PR.

A few principles that guide what gets accepted:
- Anything added must be appropriate for a 5-year-old.
- No free-text input from users, ever.
- No analytics, ads, or third-party tracking.
- No accounts or persistent storage of player data.

## License

MIT — see [LICENSE](LICENSE). Use it, fork it, customize it for your own kids.
