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

type Room struct {
	Code    string
	Phase   RoomPhase
	Players map[string]*Player // playerID -> Player
	Order   []string           // insertion order of player IDs
	Clients map[string]*Client // playerID -> WS Client (nil for AI)
	HostID  string
	Game    *GameState         // nil when in lobby
	mu      sync.RWMutex
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
		Code:    code,
		Phase:   PhaseLobby,
		Players: map[string]*Player{host.id: player},
		Order:   []string{host.id},
		Clients: map[string]*Client{host.id: host},
		HostID:  host.id,
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

	// Clean up game state too if a game is in progress
	hasGame := r.Game != nil
	r.mu.Unlock()

	if hasGame {
		r.Game.mu.Lock()
		delete(r.Game.Positions, playerID)
		delete(r.Game.MoveInputs, playerID)
		delete(r.Game.Frozen, playerID)
		delete(r.Game.BodyPos, playerID)
		delete(r.Game.BodyReported, playerID)
		delete(r.Game.UsedEmergency, playerID)
		// Decrement task total if they had crewmate tasks
		role := r.Game.Roles[playerID]
		if role == RoleCrewmate && r.Game.Tasks != nil {
			tasks := r.Game.Tasks.Assignments[playerID]
			for _, t := range tasks {
				if !t.Completed {
					r.Game.Tasks.TotalTasks--
				}
			}
			delete(r.Game.Tasks.Assignments, playerID)
		}
		delete(r.Game.Roles, playerID)
		r.Game.mu.Unlock()
	}

	// Clean up empty rooms
	r.mu.Lock()
	if len(r.Players) == 0 {
		r.mu.Unlock()
		if r.Game != nil {
			r.Game.Stop()
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
		Code:    r.Code,
		State:   r.Phase,
		Players: r.playerList(),
		HostID:  r.HostID,
		You:     forPlayerID,
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

func (r *Room) startGame() {
	r.mu.Lock()

	r.Phase = PhasePlaying

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

	// Build roles map for task assignment
	roles := make(map[string]Role, len(ids))
	aiIDs := []string{}
	for _, id := range ids {
		roles[id] = r.Players[id].Role
		if r.Players[id].AI {
			aiIDs = append(aiIDs, id)
		}
	}

	// Initialize game state
	gm := BuildMap()
	r.Game = NewGameState(gm, ids, roles, aiIDs)
	r.Game.Room = r // back-pointer for AI tick callbacks

	r.mu.Unlock()

	// Start the game loop in a goroutine
	go RunGameLoop(r, r.Game)
}
