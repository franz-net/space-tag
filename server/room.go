package main

import (
	"encoding/json"
	"math/rand"
	"strings"
	"sync"
)

type RoomPhase string

const (
	PhaseLobby   RoomPhase = "lobby"
	PhasePlaying RoomPhase = "playing"
	PhaseVoting  RoomPhase = "voting"
	PhaseEnded   RoomPhase = "ended"
)

const MaxPlayers = 6

// RoomSettings holds configurable game settings the host can change in the lobby.
type RoomSettings struct {
	TasksPerPlayer int     `json:"tasksPerPlayer"` // 2-6, default 4
	DiscussionTime float64 `json:"discussionTime"` // seconds, 15-60, default 30
	VotingTime     float64 `json:"votingTime"`     // seconds, 10-30, default 20
	TagCooldown    float64 `json:"tagCooldown"`    // seconds, 10-45, default 25
}

func DefaultSettings() RoomSettings {
	return RoomSettings{
		TasksPerPlayer: 4,
		DiscussionTime: 30,
		VotingTime:     20,
		TagCooldown:    25,
	}
}

type Room struct {
	Code     string
	Phase    RoomPhase
	Players  map[string]*Player // playerID -> Player
	Order    []string           // insertion order of player IDs
	Clients  map[string]*Client // playerID -> WS Client (nil for AI)
	HostID   string
	Settings RoomSettings       // configurable game settings
	Game     *GameState         // nil when in lobby
	mu       sync.RWMutex
}

var (
	rooms   = make(map[string]*Room)
	roomsMu sync.RWMutex
)

var aiNames = []string{"Astro", "Cosmo", "Nova", "Pixel", "Zippy", "Blip", "Fizz", "Boop"}

func generateRoomCode() string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ" // no I or O to avoid confusion
	for {
		var b strings.Builder
		for i := 0; i < 4; i++ {
			b.WriteByte(letters[rand.Intn(len(letters))])
		}
		code := b.String()
		roomsMu.RLock()
		_, exists := rooms[code]
		roomsMu.RUnlock()
		if !exists {
			return code
		}
	}
}

func nextAvailableColor(room *Room) PlayerColor {
	used := make(map[PlayerColor]bool)
	for _, p := range room.Players {
		used[p.Color] = true
	}
	for _, c := range AllColors {
		if !used[c] {
			return c
		}
	}
	return ColorRed // fallback
}

func createRoom(host *Client, name string) *Room {
	code := generateRoomCode()

	player := &Player{
		ID:    host.id,
		Name:  name,
		Color: AllColors[0],
		Host:  true,
		AI:    false,
		Alive: true,
	}

	room := &Room{
		Code:     code,
		Phase:    PhaseLobby,
		Players:  map[string]*Player{host.id: player},
		Order:    []string{host.id},
		Clients:  map[string]*Client{host.id: host},
		HostID:   host.id,
		Settings: DefaultSettings(),
	}

	roomsMu.Lock()
	rooms[code] = room
	roomsMu.Unlock()

	host.roomCode = code

	return room
}

func joinRoom(client *Client, code string, name string) (*Room, error) {
	roomsMu.RLock()
	room, exists := rooms[code]
	roomsMu.RUnlock()
	if !exists {
		return nil, errRoomNotFound
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	if room.Phase != PhaseLobby {
		return nil, errGameInProgress
	}
	if len(room.Players) >= MaxPlayers {
		return nil, errRoomFull
	}

	player := &Player{
		ID:    client.id,
		Name:  name,
		Color: nextAvailableColor(room),
		Host:  false,
		AI:    false,
		Alive: true,
	}

	room.Players[client.id] = player
	room.Order = append(room.Order, client.id)
	room.Clients[client.id] = client
	client.roomCode = code

	return room, nil
}

func (r *Room) removePlayer(playerID string) {
	r.mu.Lock()

	delete(r.Players, playerID)
	delete(r.Clients, playerID)

	newOrder := make([]string, 0, len(r.Order))
	for _, id := range r.Order {
		if id != playerID {
			newOrder = append(newOrder, id)
		}
	}
	r.Order = newOrder

	// If host left, promote next human player
	if r.HostID == playerID && len(r.Players) > 0 {
		for _, id := range r.Order {
			if p, ok := r.Players[id]; ok && !p.AI {
				r.HostID = id
				p.Host = true
				break
			}
		}
	}

	// Capture game pointer under room lock — even if r.Game is set to nil
	// concurrently by endGame, the captured pointer remains valid.
	game := r.Game
	r.mu.Unlock()

	if game != nil {
		game.mu.Lock()
		delete(game.Positions, playerID)
		delete(game.MoveInputs, playerID)
		delete(game.Frozen, playerID)
		delete(game.BodyPos, playerID)
		delete(game.BodyReported, playerID)
		delete(game.UsedEmergency, playerID)
		// Decrement task total if they had crewmate tasks
		role := game.Roles[playerID]
		if role == RoleCrewmate && game.Tasks != nil {
			tasks := game.Tasks.Assignments[playerID]
			for _, t := range tasks {
				if !t.Completed {
					game.Tasks.TotalTasks--
				}
			}
			delete(game.Tasks.Assignments, playerID)
		}
		delete(game.Roles, playerID)
		game.mu.Unlock()
	}

	// Clean up empty rooms
	r.mu.Lock()
	if len(r.Players) == 0 {
		gameRef := r.Game
		r.mu.Unlock()
		if gameRef != nil {
			gameRef.Stop()
		}
		roomsMu.Lock()
		delete(rooms, r.Code)
		roomsMu.Unlock()
		return
	}
	r.mu.Unlock()
}

func (r *Room) addAI() *Player {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.Players) >= MaxPlayers {
		return nil
	}

	// Pick an unused AI name
	usedNames := make(map[string]bool)
	for _, p := range r.Players {
		if p.AI {
			usedNames[p.Name] = true
		}
	}

	name := "Bot"
	for _, n := range aiNames {
		if !usedNames[n] {
			name = n
			break
		}
	}

	id := "ai-" + generateID()
	player := &Player{
		ID:    id,
		Name:  name,
		Color: nextAvailableColor(r),
		Host:  false,
		AI:    true,
		Alive: true,
	}

	r.Players[id] = player
	r.Order = append(r.Order, id)

	return player
}

func (r *Room) removeLastAI() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find last AI in order
	for i := len(r.Order) - 1; i >= 0; i-- {
		id := r.Order[i]
		if p, ok := r.Players[id]; ok && p.AI {
			delete(r.Players, id)
			r.Order = append(r.Order[:i], r.Order[i+1:]...)
			return id
		}
	}
	return ""
}

func (r *Room) playerList() []Player {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Player, 0, len(r.Order))
	for _, id := range r.Order {
		if p, ok := r.Players[id]; ok {
			list = append(list, *p)
		}
	}
	return list
}

func (r *Room) roomStatePayload(forPlayerID string) RoomStatePayload {
	return RoomStatePayload{
		Code:     r.Code,
		State:    r.Phase,
		Players:  r.playerList(),
		HostID:   r.HostID,
		You:      forPlayerID,
		Settings: r.Settings,
	}
}

func (r *Room) broadcast(msgType MsgType, payload interface{}) {
	data, _ := json.Marshal(payload)
	env := Envelope{Type: msgType, Payload: data}
	msg, _ := json.Marshal(env)

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, client := range r.Clients {
		if client != nil {
			select {
			case client.send <- msg:
			default:
			}
		}
	}
}

func (r *Room) sendTo(playerID string, msgType MsgType, payload interface{}) {
	r.mu.RLock()
	client, ok := r.Clients[playerID]
	r.mu.RUnlock()

	if !ok || client == nil {
		return
	}

	data, _ := json.Marshal(payload)
	env := Envelope{Type: msgType, Payload: data}
	msg, _ := json.Marshal(env)

	select {
	case client.send <- msg:
	default:
	}
}

// snapshotPlayerIDs returns a copy of the room's player order under a brief
// read lock. Use this when you need to iterate players and call methods that
// take their own locks (sendTo, roomStatePayload, etc.) — calling those from
// inside a held RLock can deadlock against pending writers.
func (r *Room) snapshotPlayerIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.Order))
	copy(out, r.Order)
	return out
}

func (r *Room) startGame() {
	r.mu.Lock()

	r.Phase = PhasePlaying
	settings := r.Settings

	// Collect all player IDs
	ids := make([]string, 0, len(r.Order))
	for _, id := range r.Order {
		ids = append(ids, id)
	}

	// Pick a random tagger
	taggerIdx := rand.Intn(len(ids))

	for i, id := range ids {
		p := r.Players[id]
		if i == taggerIdx {
			p.Role = RoleTagger
		} else {
			p.Role = RoleCrewmate
		}
		p.Alive = true
	}

	// Build roles map and color map for task assignment and AI chat
	roles := make(map[string]Role, len(ids))
	playerColors := make(map[string]string, len(ids))
	aiIDs := []string{}
	for _, id := range ids {
		roles[id] = r.Players[id].Role
		playerColors[id] = string(r.Players[id].Color)
		if r.Players[id].AI {
			aiIDs = append(aiIDs, id)
		}
	}

	// Initialize game state using room settings
	gm := BuildMap()
	r.Game = NewGameState(gm, ids, roles, aiIDs, settings, playerColors)
	r.Game.Room = r // back-pointer for AI tick callbacks

	r.mu.Unlock()

	// Start the game loop in a goroutine
	go RunGameLoop(r, r.Game)
}
