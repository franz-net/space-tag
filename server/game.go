package main

import (
	"math"
	"sync"
	"time"
)

const TickRate = 20 // Hz
const TickDuration = time.Second / TickRate

type GameState struct {
	Map         *GameMap
	Positions   map[string]Vec2    // playerID -> position
	MoveInputs  map[string]Vec2    // playerID -> direction vector (from client)
	Tasks       *TasksState
	Roles       map[string]Role    // playerID -> role
	Frozen      map[string]bool    // playerID -> is frozen (true=dead/body)
	BodyPos     map[string]Vec2    // bodyID -> position where body lies
	BodyReported map[string]bool   // bodyID -> has been reported
	TagCooldown time.Time          // earliest time tagger can tag again
	UsedEmergency map[string]bool  // playerID -> already used emergency
	Meeting     *Meeting           // current meeting, nil if none
	AIBrains    map[string]*AIBrain // playerID -> AI brain (only for AI players)
	Room        *Room              // back-pointer for AI tick callbacks
	mu          sync.RWMutex
	stopCh      chan struct{}
}

// PositionsPayload is sent to all clients each tick
type PositionsPayload struct {
	Positions map[string]Vec2 `json:"positions"`
	Frozen    []string        `json:"frozen"` // playerIDs that are frozen (ghosts)
	Bodies    map[string]Vec2 `json:"bodies"` // bodyID -> body position (visible to everyone)
}

// MovePayload is sent from client to server
type MovePayload struct {
	DX float64 `json:"dx"`
	DY float64 `json:"dy"`
}

// MapDataPayload sends the map geometry to clients
type MapDataPayload struct {
	Map *GameMap `json:"map"`
}

func NewGameState(gm *GameMap, playerIDs []string, roles map[string]Role, aiIDs []string) *GameState {
	positions := make(map[string]Vec2, len(playerIDs))
	// Spread players in a circle around spawn so they don't overlap
	for i, id := range playerIDs {
		angle := float64(i) * (2 * 3.14159 / float64(len(playerIDs)))
		offset := PlayerRadius * 3
		positions[id] = Vec2{
			X: gm.SpawnPos.X + math.Cos(angle)*offset,
			Y: gm.SpawnPos.Y + math.Sin(angle)*offset,
		}
	}

	tasks := InitTasks(playerIDs, roles)

	brains := make(map[string]*AIBrain, len(aiIDs))
	for _, id := range aiIDs {
		brains[id] = NewAIBrain(id, roles[id])
	}

	return &GameState{
		Map:           gm,
		Positions:     positions,
		MoveInputs:    make(map[string]Vec2),
		Tasks:         tasks,
		Roles:         roles,
		Frozen:        make(map[string]bool),
		BodyPos:       make(map[string]Vec2),
		BodyReported:  make(map[string]bool),
		UsedEmergency: make(map[string]bool),
		AIBrains:      brains,
		stopCh:        make(chan struct{}),
	}
}

// IsAlive returns true if the player is not frozen
func (gs *GameState) IsAlive(playerID string) bool {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return !gs.Frozen[playerID]
}

// AlivePlayers returns IDs of all non-frozen players
func (gs *GameState) AlivePlayers() []string {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	out := make([]string, 0, len(gs.Positions))
	for id := range gs.Positions {
		if !gs.Frozen[id] {
			out = append(out, id)
		}
	}
	return out
}

// CountAlive returns counts of alive crew and tagger
func (gs *GameState) CountAlive() (crew int, tagger int) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	for id := range gs.Positions {
		if gs.Frozen[id] {
			continue
		}
		if gs.Roles[id] == RoleTagger {
			tagger++
		} else {
			crew++
		}
	}
	return
}

// TryTag attempts a tag from selfID to targetID. Returns frozen target ID and ok=true if successful.
func (gs *GameState) TryTag(selfID, targetID string, now time.Time) (Vec2, bool) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	// Caller must be alive tagger
	if gs.Roles[selfID] != RoleTagger || gs.Frozen[selfID] {
		return Vec2{}, false
	}
	// Target must be alive
	if gs.Frozen[targetID] {
		return Vec2{}, false
	}
	// Cannot tag self
	if selfID == targetID {
		return Vec2{}, false
	}
	// Cooldown
	if now.Before(gs.TagCooldown) {
		return Vec2{}, false
	}
	// Range check
	a := gs.Positions[selfID]
	b := gs.Positions[targetID]
	dx := a.X - b.X
	dy := a.Y - b.Y
	if dx*dx+dy*dy > TagRange*TagRange {
		return Vec2{}, false
	}

	// Freeze the target
	gs.Frozen[targetID] = true
	gs.BodyPos[targetID] = b
	gs.TagCooldown = now.Add(TagCooldown)
	// Stop them moving
	gs.MoveInputs[targetID] = Vec2{}

	return b, true
}

// TryReportBody returns true if reporter is near an unreported body
func (gs *GameState) TryReportBody(reporterID string) (string, bool) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if gs.Frozen[reporterID] {
		return "", false
	}
	pos, ok := gs.Positions[reporterID]
	if !ok {
		return "", false
	}

	for bodyID, bodyPos := range gs.BodyPos {
		if gs.BodyReported[bodyID] {
			continue
		}
		dx := pos.X - bodyPos.X
		dy := pos.Y - bodyPos.Y
		if dx*dx+dy*dy <= ReportRange*ReportRange {
			gs.BodyReported[bodyID] = true
			return bodyID, true
		}
	}
	return "", false
}

// TryEmergency checks if a player can call emergency (alive, in cafeteria, hasn't used it)
func (gs *GameState) TryEmergency(callerID string) bool {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if gs.Frozen[callerID] {
		return false
	}
	if gs.UsedEmergency[callerID] {
		return false
	}
	pos, ok := gs.Positions[callerID]
	if !ok {
		return false
	}
	roomID := gs.Map.GetRoomAt(pos)
	if roomID != "cafeteria" {
		return false
	}

	gs.UsedEmergency[callerID] = true
	return true
}

// TeleportToCafeteria moves all alive players (and their bodies stay) to cafeteria spawn
func (gs *GameState) TeleportToCafeteria() {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	// Find cafeteria center
	var caf *MapRoom
	for i := range gs.Map.Rooms {
		if gs.Map.Rooms[i].ID == "cafeteria" {
			caf = &gs.Map.Rooms[i]
			break
		}
	}
	if caf == nil {
		return
	}

	cx := caf.Bounds.X + caf.Bounds.W/2
	cy := caf.Bounds.Y + caf.Bounds.H/2

	// Spread alive players in a circle
	alive := []string{}
	for id := range gs.Positions {
		if !gs.Frozen[id] {
			alive = append(alive, id)
		}
	}

	for i, id := range alive {
		angle := float64(i) * (2 * 3.14159 / float64(len(alive)))
		offset := PlayerRadius * 4
		gs.Positions[id] = Vec2{
			X: cx + math.Cos(angle)*offset,
			Y: cy + math.Sin(angle)*offset,
		}
		gs.MoveInputs[id] = Vec2{}
	}
}

func (gs *GameState) SetInput(playerID string, dx, dy float64) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.MoveInputs[playerID] = Vec2{dx, dy}
}

func (gs *GameState) Tick(dt float64) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	// Update AI brains (they read state and write MoveInputs).
	// During meetings, AI brains still tick to vote/chat but don't move.
	now := time.Now()
	for _, ai := range gs.AIBrains {
		AITick(gs, gs.Room, ai, now)
	}

	// Don't move anyone during a meeting
	if gs.Meeting != nil {
		return
	}

	for id, dir := range gs.MoveInputs {
		if dir.X == 0 && dir.Y == 0 {
			continue
		}

		pos, ok := gs.Positions[id]
		if !ok {
			continue
		}

		// Normalize direction and apply speed
		nx, ny := normalize(dir.X, dir.Y)
		dx := nx * PlayerSpeed * dt
		dy := ny * PlayerSpeed * dt

		if gs.Frozen[id] {
			// Ghosts move freely — no walls, no collision, but stay in map bounds
			newX := pos.X + dx
			newY := pos.Y + dy
			if newX < 0 {
				newX = 0
			}
			if newX > gs.Map.Width {
				newX = gs.Map.Width
			}
			if newY < 0 {
				newY = 0
			}
			if newY > gs.Map.Height {
				newY = gs.Map.Height
			}
			gs.Positions[id] = Vec2{newX, newY}
		} else {
			gs.Positions[id] = ResolveMovement(gs.Map, pos, dx, dy)
		}
	}

	// Push overlapping alive players apart (ghosts pass through everyone)
	gs.separatePlayersAlive()
}

func (gs *GameState) separatePlayersAlive() {
	minDist := PlayerRadius * 2
	ids := make([]string, 0, len(gs.Positions))
	for id := range gs.Positions {
		if !gs.Frozen[id] {
			ids = append(ids, id)
		}
	}

	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			posA := gs.Positions[ids[i]]
			posB := gs.Positions[ids[j]]
			dx := posB.X - posA.X
			dy := posB.Y - posA.Y
			distSq := dx*dx + dy*dy
			if distSq < minDist*minDist && distSq > 0.001 {
				dist := math.Sqrt(distSq)
				overlap := minDist - dist
				nx := dx / dist
				ny := dy / dist
				push := overlap * 0.5
				newA := Vec2{posA.X - nx*push, posA.Y - ny*push}
				newB := Vec2{posB.X + nx*push, posB.Y + ny*push}
				if gs.Map.IsWalkable(newA, PlayerRadius) {
					gs.Positions[ids[i]] = newA
				}
				if gs.Map.IsWalkable(newB, PlayerRadius) {
					gs.Positions[ids[j]] = newB
				}
			}
		}
	}
}

// separatePlayers pushes any overlapping players away from each other
func (gs *GameState) separatePlayers() {
	minDist := PlayerRadius * 2
	ids := make([]string, 0, len(gs.Positions))
	for id := range gs.Positions {
		ids = append(ids, id)
	}

	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			posA := gs.Positions[ids[i]]
			posB := gs.Positions[ids[j]]

			dx := posB.X - posA.X
			dy := posB.Y - posA.Y
			distSq := dx*dx + dy*dy

			if distSq < minDist*minDist && distSq > 0.001 {
				dist := math.Sqrt(distSq)
				overlap := minDist - dist
				// Normalize and push each player half the overlap distance
				nx := dx / dist
				ny := dy / dist
				push := overlap * 0.5

				newA := Vec2{posA.X - nx*push, posA.Y - ny*push}
				newB := Vec2{posB.X + nx*push, posB.Y + ny*push}

				// Only apply if still walkable
				if gs.Map.IsWalkable(newA, PlayerRadius) {
					gs.Positions[ids[i]] = newA
				}
				if gs.Map.IsWalkable(newB, PlayerRadius) {
					gs.Positions[ids[j]] = newB
				}
			}
		}
	}
}

func (gs *GameState) GetPositions() map[string]Vec2 {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	// Copy
	out := make(map[string]Vec2, len(gs.Positions))
	for k, v := range gs.Positions {
		out[k] = v
	}
	return out
}

func (gs *GameState) Stop() {
	select {
	case <-gs.stopCh:
	default:
		close(gs.stopCh)
	}
}

// RunGameLoop starts the 20Hz game loop for a room.
// It ticks movement and broadcasts positions.
func RunGameLoop(room *Room, gs *GameState) {
	ticker := time.NewTicker(TickDuration)
	defer ticker.Stop()

	for {
		select {
		case <-gs.stopCh:
			return
		case <-ticker.C:
			gs.Tick(1.0 / float64(TickRate))

			positions := gs.GetPositions()
			frozen := gs.GetFrozenList()
			bodies := gs.GetBodies()
			room.broadcast(MsgPositions, PositionsPayload{
				Positions: positions,
				Frozen:    frozen,
				Bodies:    bodies,
			})
		}
	}
}

func (gs *GameState) GetBodies() map[string]Vec2 {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	out := make(map[string]Vec2, len(gs.BodyPos))
	for k, v := range gs.BodyPos {
		out[k] = v
	}
	return out
}

func (gs *GameState) GetFrozenList() []string {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	out := make([]string, 0)
	for id, frozen := range gs.Frozen {
		if frozen {
			out = append(out, id)
		}
	}
	return out
}
