package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	id       string
	roomCode string
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		c.handleMessage(message)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) sendEnvelope(msgType MsgType, payload interface{}) {
	data, _ := json.Marshal(payload)
	env := Envelope{Type: msgType, Payload: data}
	msg, _ := json.Marshal(env)
	select {
	case c.send <- msg:
	default:
	}
}

func (c *Client) handleMessage(raw []byte) {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		log.Printf("invalid message from %s: %v", c.id, err)
		return
	}

	switch env.Type {
	case MsgCreateRoom:
		c.handleCreateRoom(env.Payload)
	case MsgJoinRoom:
		c.handleJoinRoom(env.Payload)
	case MsgLeaveRoom:
		c.handleLeaveRoom()
	case MsgStartGame:
		c.handleStartGame()
	case MsgAddAI:
		c.handleAddAI()
	case MsgRemoveAI:
		c.handleRemoveAI()
	case MsgMove:
		c.handleMove(env.Payload)
	case MsgTaskStart:
		c.handleTaskStart(env.Payload)
	case MsgTaskComplete:
		c.handleTaskComplete(env.Payload)
	case MsgTagPlayer:
		c.handleTagPlayer(env.Payload)
	case MsgReportBody:
		c.handleReportBody()
	case MsgEmergency:
		c.handleEmergency()
	case MsgChatMessage:
		c.handleChatMessage(env.Payload)
	case MsgCastVote:
		c.handleCastVote(env.Payload)
	default:
		log.Printf("unknown message type from %s: %s", c.id, env.Type)
	}
}

func (c *Client) handleCreateRoom(payload json.RawMessage) {
	var p CreateRoomPayload
	if err := json.Unmarshal(payload, &p); err != nil || p.PlayerName == "" {
		c.sendEnvelope(MsgError, ErrorPayload{Message: "Please enter your name"})
		return
	}

	if c.roomCode != "" {
		c.sendEnvelope(MsgError, ErrorPayload{Message: "You are already in a room"})
		return
	}

	room := createRoom(c, p.PlayerName)
	c.sendEnvelope(MsgRoomState, room.roomStatePayload(c.id))
}

func (c *Client) handleJoinRoom(payload json.RawMessage) {
	var p JoinRoomPayload
	if err := json.Unmarshal(payload, &p); err != nil || p.PlayerName == "" || p.RoomCode == "" {
		c.sendEnvelope(MsgError, ErrorPayload{Message: "Please enter your name and room code"})
		return
	}

	if c.roomCode != "" {
		c.sendEnvelope(MsgError, ErrorPayload{Message: "You are already in a room"})
		return
	}

	room, err := joinRoom(c, p.RoomCode, p.PlayerName)
	if err != nil {
		c.sendEnvelope(MsgError, ErrorPayload{Message: err.Error()})
		return
	}

	// Send room state to all players (each gets their own "you" field)
	room.mu.RLock()
	for pid := range room.Clients {
		room.sendTo(pid, MsgRoomState, room.roomStatePayload(pid))
	}
	room.mu.RUnlock()
}

func (c *Client) handleLeaveRoom() {
	if c.roomCode == "" {
		return
	}

	roomsMu.RLock()
	room, exists := rooms[c.roomCode]
	roomsMu.RUnlock()

	if !exists {
		c.roomCode = ""
		return
	}

	room.removePlayer(c.id)
	c.roomCode = ""

	// Notify remaining players
	roomsMu.RLock()
	_, stillExists := rooms[room.Code]
	roomsMu.RUnlock()

	if stillExists {
		room.mu.RLock()
		for pid := range room.Clients {
			room.sendTo(pid, MsgRoomState, room.roomStatePayload(pid))
		}
		room.mu.RUnlock()
	}
}

func (c *Client) handleStartGame() {
	if c.roomCode == "" {
		return
	}

	roomsMu.RLock()
	room, exists := rooms[c.roomCode]
	roomsMu.RUnlock()

	if !exists {
		return
	}

	room.mu.RLock()
	isHost := room.HostID == c.id
	playerCount := len(room.Players)
	room.mu.RUnlock()

	if !isHost {
		c.sendEnvelope(MsgError, ErrorPayload{Message: "Only the host can start the game"})
		return
	}

	if playerCount < 2 {
		c.sendEnvelope(MsgError, ErrorPayload{Message: "Need at least 2 players to start"})
		return
	}

	room.startGame()

	// Send map data to all players
	room.broadcast(MsgMapData, MapDataPayload{Map: room.Game.Map})

	// Send each player their role and task assignments privately
	room.mu.RLock()
	for _, id := range room.Order {
		p := room.Players[id]
		room.sendTo(id, MsgGameStarted, GameStartedPayload{
			Role: p.Role,
			You:  id,
		})
		// Send task assignments
		tasks := room.Game.Tasks.GetPlayerTasks(id)
		room.sendTo(id, MsgTaskProgress, TaskProgressPayload{
			Progress: 0,
			Tasks:    tasks,
		})
		// Send initial cooldown to the tagger
		if p.Role == RoleTagger {
			room.sendTo(id, MsgCooldown, CooldownPayload{Seconds: 10})
		}
	}
	room.mu.RUnlock()

	// Set initial tag cooldown so tagger can't immediately tag at start
	room.Game.mu.Lock()
	room.Game.TagCooldown = time.Now().Add(10 * time.Second)
	room.Game.mu.Unlock()
}

func (c *Client) handleAddAI() {
	if c.roomCode == "" {
		return
	}

	roomsMu.RLock()
	room, exists := rooms[c.roomCode]
	roomsMu.RUnlock()

	if !exists {
		return
	}

	room.mu.RLock()
	isHost := room.HostID == c.id
	room.mu.RUnlock()

	if !isHost {
		c.sendEnvelope(MsgError, ErrorPayload{Message: "Only the host can add AI players"})
		return
	}

	player := room.addAI()
	if player == nil {
		c.sendEnvelope(MsgError, ErrorPayload{Message: "Room is full"})
		return
	}

	// Notify all players
	room.mu.RLock()
	for pid := range room.Clients {
		room.sendTo(pid, MsgRoomState, room.roomStatePayload(pid))
	}
	room.mu.RUnlock()
}

func (c *Client) handleRemoveAI() {
	if c.roomCode == "" {
		return
	}

	roomsMu.RLock()
	room, exists := rooms[c.roomCode]
	roomsMu.RUnlock()

	if !exists {
		return
	}

	room.mu.RLock()
	isHost := room.HostID == c.id
	room.mu.RUnlock()

	if !isHost {
		c.sendEnvelope(MsgError, ErrorPayload{Message: "Only the host can remove AI players"})
		return
	}

	removed := room.removeLastAI()
	if removed == "" {
		return
	}

	room.mu.RLock()
	for pid := range room.Clients {
		room.sendTo(pid, MsgRoomState, room.roomStatePayload(pid))
	}
	room.mu.RUnlock()
}

func (c *Client) handleMove(payload json.RawMessage) {
	if c.roomCode == "" {
		return
	}

	var p MovePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return
	}

	roomsMu.RLock()
	room, exists := rooms[c.roomCode]
	roomsMu.RUnlock()

	if !exists || room.Game == nil {
		return
	}

	room.Game.SetInput(c.id, p.DX, p.DY)
}

func (c *Client) handleTaskStart(payload json.RawMessage) {
	if c.roomCode == "" {
		return
	}

	var p TaskStartPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return
	}

	roomsMu.RLock()
	room, exists := rooms[c.roomCode]
	roomsMu.RUnlock()

	if !exists || room.Game == nil {
		return
	}

	// Validate player is near the station
	room.Game.mu.RLock()
	playerPos := room.Game.Positions[c.id]
	room.Game.mu.RUnlock()

	var station *TaskStation
	for i := range room.Game.Tasks.Stations {
		if room.Game.Tasks.Stations[i].ID == p.StationID {
			station = &room.Game.Tasks.Stations[i]
			break
		}
	}

	if station == nil {
		return
	}

	dx := playerPos.X - station.Position.X
	dy := playerPos.Y - station.Position.Y
	if dx*dx+dy*dy > TaskInteractionRange*TaskInteractionRange {
		c.sendEnvelope(MsgError, ErrorPayload{Message: "Too far from task"})
		return
	}

	// Send task data with randomized params
	c.sendEnvelope(MsgTaskData, TaskDataPayload{
		StationID: station.ID,
		Type:      station.Type,
		Params:    generateTaskParams(station.Type),
	})
}

func (c *Client) handleTaskComplete(payload json.RawMessage) {
	if c.roomCode == "" {
		return
	}

	var p TaskCompletePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return
	}

	roomsMu.RLock()
	room, exists := rooms[c.roomCode]
	roomsMu.RUnlock()

	if !exists || room.Game == nil {
		return
	}

	// Get player role
	room.mu.RLock()
	player, ok := room.Players[c.id]
	room.mu.RUnlock()
	if !ok {
		return
	}

	// Complete the task
	room.Game.mu.Lock()
	valid := room.Game.Tasks.CompleteTask(c.id, p.StationID, player.Role)
	progress := room.Game.Tasks.Progress()
	allDone := room.Game.Tasks.AllTasksDone()
	room.Game.mu.Unlock()

	if !valid {
		return
	}

	// Send updated progress to all players (each gets their own task list)
	room.mu.RLock()
	for _, id := range room.Order {
		tasks := room.Game.Tasks.GetPlayerTasks(id)
		room.sendTo(id, MsgTaskProgress, TaskProgressPayload{
			Progress: progress,
			Tasks:    tasks,
		})
	}
	room.mu.RUnlock()

	// Check win condition
	if allDone {
		c.handleCrewWin(room)
	}
}

func (c *Client) handleCrewWin(room *Room) {
	endGame(room, "crew")
}

func endGame(room *Room, winner string) {
	room.mu.Lock()
	if room.Phase == PhaseEnded {
		room.mu.Unlock()
		return
	}
	room.Phase = PhaseEnded
	room.mu.Unlock()

	if room.Game != nil {
		room.Game.Stop()
	}

	// Build roles list
	room.mu.RLock()
	roles := make([]PlayerRoleInfo, 0, len(room.Order))
	for _, id := range room.Order {
		p := room.Players[id]
		roles = append(roles, PlayerRoleInfo{
			ID:   id,
			Name: p.Name,
			Role: p.Role,
		})
	}
	room.mu.RUnlock()

	room.broadcast(MsgGameOver, GameOverPayload{
		Winner: winner,
		Roles:  roles,
	})
}

// checkWinConditions checks if either side has won. Call after any state change.
func checkWinConditions(room *Room) bool {
	if room.Game == nil {
		return false
	}

	crew, tagger := room.Game.CountAlive()

	// Tagger wins if they outnumber or equal the crew (1 vs 1 or fewer)
	if tagger > 0 && tagger >= crew {
		endGame(room, "tagger")
		return true
	}

	// Crew wins if all taggers are eliminated
	if tagger == 0 {
		endGame(room, "crew")
		return true
	}

	return false
}

// generateTaskParams creates random parameters for each task type
func generateTaskParams(taskType TaskType) interface{} {
	switch taskType {
	case TaskTapTargets:
		// 5 random target positions within a 300x300 area
		targets := make([]Vec2, 5)
		for i := range targets {
			targets[i] = Vec2{
				X: 30 + rand.Float64()*240,
				Y: 30 + rand.Float64()*240,
			}
		}
		return map[string]interface{}{"targets": targets}

	case TaskConnectWires:
		// 4 colors in random order on the right side
		colors := []string{"red", "blue", "green", "yellow"}
		perm := rand.Perm(4)
		rightOrder := make([]string, 4)
		for i, j := range perm {
			rightOrder[i] = colors[j]
		}
		return map[string]interface{}{
			"leftColors":  colors,
			"rightColors": rightOrder,
		}

	case TaskMatchColors:
		// 3 pairs = 6 cards, shuffled positions
		colors := []string{"red", "blue", "green"}
		cards := append(colors, colors...)
		perm := rand.Perm(6)
		shuffled := make([]string, 6)
		for i, j := range perm {
			shuffled[i] = cards[j]
		}
		return map[string]interface{}{"cards": shuffled}

	case TaskSimonSays:
		// Sequence of 3 colors
		options := []string{"red", "blue", "green", "yellow"}
		seq := make([]string, 3)
		for i := range seq {
			seq[i] = options[rand.Intn(4)]
		}
		return map[string]interface{}{"sequence": seq}
	}

	return nil
}

// ===== Tagger / voting handlers =====

func (c *Client) getRoom() *Room {
	if c.roomCode == "" {
		return nil
	}
	roomsMu.RLock()
	defer roomsMu.RUnlock()
	return rooms[c.roomCode]
}

func (c *Client) handleTagPlayer(payload json.RawMessage) {
	room := c.getRoom()
	if room == nil || room.Game == nil {
		return
	}

	var p TagPlayerPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return
	}

	bodyPos, ok := room.Game.TryTag(c.id, p.TargetID, time.Now())
	if !ok {
		return
	}

	// Broadcast the freeze event
	room.broadcast(MsgPlayerFrozen, PlayerFrozenPayload{
		PlayerID: p.TargetID,
		Position: bodyPos,
	})

	// Send cooldown to tagger
	c.sendEnvelope(MsgCooldown, CooldownPayload{
		Seconds: TagCooldown.Seconds(),
	})

	// Check win conditions
	checkWinConditions(room)
}

func (c *Client) handleReportBody() {
	room := c.getRoom()
	if room == nil || room.Game == nil {
		return
	}

	bodyID, ok := room.Game.TryReportBody(c.id)
	if !ok {
		return
	}

	startMeeting(room, c.id, "body", bodyID)
}

func (c *Client) handleEmergency() {
	room := c.getRoom()
	if room == nil || room.Game == nil {
		return
	}

	if !room.Game.TryEmergency(c.id) {
		c.sendEnvelope(MsgError, ErrorPayload{Message: "Cannot call emergency right now"})
		return
	}

	startMeeting(room, c.id, "emergency", "")
}

func (c *Client) handleChatMessage(payload json.RawMessage) {
	room := c.getRoom()
	if room == nil || room.Game == nil || room.Game.Meeting == nil {
		return
	}

	var p ChatMessagePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return
	}

	// Validate message ID is in pre-defined list
	if !PreDefinedMessages[p.MessageID] {
		return
	}

	// Validate sender is alive
	if !room.Game.IsAlive(c.id) {
		return
	}

	// Rate-limit
	if !room.Game.Meeting.CanSendChat(c.id) {
		return
	}
	room.Game.Meeting.RecordChat(c.id)

	// Broadcast
	room.broadcast(MsgChatMessage, ChatMessagePayload{
		SenderID:  c.id,
		MessageID: p.MessageID,
	})
}

func (c *Client) handleCastVote(payload json.RawMessage) {
	room := c.getRoom()
	if room == nil || room.Game == nil || room.Game.Meeting == nil {
		return
	}

	var p CastVotePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return
	}

	room.Game.Meeting.CastVote(c.id, p.TargetID)
}

// startMeeting begins a meeting in the given room. Runs the discussion+voting timeline in a goroutine.
func startMeeting(room *Room, callerID, reason, bodyID string) {
	if room.Game == nil {
		return
	}

	alive := room.Game.AlivePlayers()
	meeting := NewMeeting(callerID, reason, bodyID, alive)

	room.Game.mu.Lock()
	if room.Game.Meeting != nil {
		// already in a meeting
		room.Game.mu.Unlock()
		return
	}
	room.Game.Meeting = meeting
	room.Game.mu.Unlock()

	// Teleport everyone to cafeteria
	room.Game.TeleportToCafeteria()

	// Mark room phase
	room.mu.Lock()
	room.Phase = PhaseVoting
	room.mu.Unlock()

	// Notify all players
	room.broadcast(MsgMeetingStart, MeetingStartPayload{
		CallerID:       callerID,
		Reason:         reason,
		BodyID:         bodyID,
		DiscussionTime: DiscussionDuration.Seconds(),
		VotingTime:     VotingDuration.Seconds(),
		AlivePlayers:   alive,
	})

	// Run timeline in a goroutine
	go runMeetingTimeline(room, meeting)
}

func runMeetingTimeline(room *Room, meeting *Meeting) {
	// Discussion phase
	time.Sleep(DiscussionDuration)
	meeting.SetPhase("voting")

	// Voting phase
	time.Sleep(VotingDuration)

	// Tally
	ejectedID := meeting.TallyVotes()
	wasTagger := false

	if ejectedID != "" {
		room.Game.mu.Lock()
		// Eject = freeze
		if room.Game.Roles[ejectedID] == RoleTagger {
			wasTagger = true
		}
		room.Game.Frozen[ejectedID] = true
		// Remove their body since they're "sent home" not killed
		delete(room.Game.BodyPos, ejectedID)
		room.Game.mu.Unlock()
	}

	// Send results
	room.broadcast(MsgMeetingEnd, MeetingEndPayload{
		Votes:     meeting.GetVotes(),
		EjectedID: ejectedID,
		WasTagger: wasTagger,
	})

	// Brief pause for the reveal animation
	time.Sleep(3 * time.Second)

	// Clear meeting
	room.Game.mu.Lock()
	room.Game.Meeting = nil
	// Reset tag cooldown so the tagger can't immediately tag after meeting
	room.Game.TagCooldown = time.Now().Add(10 * time.Second)
	room.Game.mu.Unlock()

	// Resume gameplay
	room.mu.Lock()
	if room.Phase == PhaseVoting {
		room.Phase = PhasePlaying
	}
	room.mu.Unlock()

	// Check win conditions
	checkWinConditions(room)
}
