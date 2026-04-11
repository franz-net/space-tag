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
	case MsgRoomSettings:
		c.handleRoomSettings(env.Payload)
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
	case MsgSabotage:
		c.handleSabotage(env.Payload)
	case MsgSabotageFix:
		c.handleSabotageFix(env.Payload)
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
	for _, pid := range room.snapshotPlayerIDs() {
		room.sendTo(pid, MsgRoomState, room.roomStatePayload(pid))
	}
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

	// Notify remaining players (if any)
	roomsMu.RLock()
	_, stillExists := rooms[room.Code]
	roomsMu.RUnlock()

	if !stillExists {
		return
	}

	ids := room.snapshotPlayerIDs()
	for _, pid := range ids {
		room.sendTo(pid, MsgRoomState, room.roomStatePayload(pid))
	}

	room.mu.RLock()
	inGame := room.Game != nil && room.Phase == PhasePlaying
	room.mu.RUnlock()

	// If a game was in progress, the win conditions may have changed
	// (e.g. tagger left → crew wins) and tasks may need re-broadcasting.
	if inGame {
		for _, id := range ids {
			tasks := room.Game.Tasks.GetPlayerTasks(id)
			room.sendTo(id, MsgTaskProgress, TaskProgressPayload{
				Progress: room.Game.Tasks.Progress(),
				Tasks:    tasks,
			})
		}
		checkWinConditions(room)
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
	phase := room.Phase
	room.mu.RUnlock()

	if !isHost {
		c.sendEnvelope(MsgError, ErrorPayload{Message: "Only the host can start the game"})
		return
	}

	if phase != PhaseLobby {
		c.sendEnvelope(MsgError, ErrorPayload{Message: "Game already in progress"})
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
	ids := room.snapshotPlayerIDs()
	for _, id := range ids {
		room.mu.RLock()
		p := room.Players[id]
		role := Role("")
		if p != nil {
			role = p.Role
		}
		room.mu.RUnlock()

		room.sendTo(id, MsgGameStarted, GameStartedPayload{
			Role: role,
			You:  id,
		})
		// Send task assignments
		tasks := room.Game.Tasks.GetPlayerTasks(id)
		room.sendTo(id, MsgTaskProgress, TaskProgressPayload{
			Progress: 0,
			Tasks:    tasks,
		})
		// Send initial cooldown to the tagger
		if role == RoleTagger {
			room.sendTo(id, MsgCooldown, CooldownPayload{Seconds: 10})
		}
	}

	// Set initial tag cooldown so tagger can't immediately tag at start
	room.Game.mu.Lock()
	room.Game.TagCooldown = time.Now().Add(10 * time.Second)
	room.Game.mu.Unlock()
}

func (c *Client) handleRoomSettings(payload json.RawMessage) {
	room := c.getRoom()
	if room == nil {
		return
	}

	room.mu.RLock()
	isHost := room.HostID == c.id
	phase := room.Phase
	room.mu.RUnlock()

	if !isHost {
		c.sendEnvelope(MsgError, ErrorPayload{Message: "Only the host can change settings"})
		return
	}
	if phase != PhaseLobby {
		return
	}

	var s RoomSettings
	if err := json.Unmarshal(payload, &s); err != nil {
		return
	}

	// Clamp values to allowed ranges
	if s.TasksPerPlayer < 2 {
		s.TasksPerPlayer = 2
	} else if s.TasksPerPlayer > 6 {
		s.TasksPerPlayer = 6
	}
	if s.DiscussionTime < 15 {
		s.DiscussionTime = 15
	} else if s.DiscussionTime > 60 {
		s.DiscussionTime = 60
	}
	if s.VotingTime < 10 {
		s.VotingTime = 10
	} else if s.VotingTime > 30 {
		s.VotingTime = 30
	}
	if s.TagCooldown < 10 {
		s.TagCooldown = 10
	} else if s.TagCooldown > 45 {
		s.TagCooldown = 45
	}

	room.mu.Lock()
	room.Settings = s
	room.mu.Unlock()

	// Broadcast updated room state so everyone sees the new settings
	for _, pid := range room.snapshotPlayerIDs() {
		room.sendTo(pid, MsgRoomState, room.roomStatePayload(pid))
	}
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
	for _, pid := range room.snapshotPlayerIDs() {
		room.sendTo(pid, MsgRoomState, room.roomStatePayload(pid))
	}
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

	for _, pid := range room.snapshotPlayerIDs() {
		room.sendTo(pid, MsgRoomState, room.roomStatePayload(pid))
	}
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

	// Reject task completion during meetings — prevents a task-induced
	// crew win from running endGame while runMeetingTimeline is still
	// scheduled (which would race on room.Game becoming nil).
	room.mu.RLock()
	phase := room.Phase
	room.mu.RUnlock()
	if phase != PhasePlaying {
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
	for _, id := range room.snapshotPlayerIDs() {
		tasks := room.Game.Tasks.GetPlayerTasks(id)
		room.sendTo(id, MsgTaskProgress, TaskProgressPayload{
			Progress: progress,
			Tasks:    tasks,
		})
	}

	// Check win condition
	if allDone {
		c.handleCrewWin(room)
	}
}

func (c *Client) handleCrewWin(room *Room) {
	endGame(room, "crew")
}

func endGame(room *Room, winner string) {
	dbg("endGame called: winner=%s, room=%s", winner, room.Code)
	room.mu.Lock()
	if room.Phase == PhaseEnded || room.Phase == PhaseLobby {
		dbg("endGame: already ended/lobby, skipping")
		room.mu.Unlock()
		return
	}
	room.Phase = PhaseEnded
	room.mu.Unlock()
	dbg("endGame: phase set to ended")

	if room.Game != nil {
		room.Game.Stop()
		dbg("endGame: game loop stopped")
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
	dbg("endGame: broadcasted game_over")

	// Reset the room back to lobby so players can start another round
	// without recreating the room. Players stay; their game state clears.
	resetRoomToLobby(room)
	dbg("endGame: resetRoomToLobby returned")
}

func resetRoomToLobby(room *Room) {
	dbg("resetRoomToLobby: %s", room.Code)
	room.mu.Lock()
	room.Phase = PhaseLobby
	room.Game = nil
	for _, p := range room.Players {
		p.Alive = true
		p.Role = ""
	}
	room.mu.Unlock()
	dbg("resetRoomToLobby: state cleared, broadcasting room_state")

	// Broadcast new room state to all remaining players
	for _, pid := range room.snapshotPlayerIDs() {
		room.sendTo(pid, MsgRoomState, room.roomStatePayload(pid))
	}
	dbg("resetRoomToLobby: done")
}

// checkWinConditions checks if either side has won. Call after any state change.
func checkWinConditions(room *Room) bool {
	game := room.Game
	if game == nil {
		dbg("checkWinConditions: game is nil, skipping")
		return false
	}

	crew, tagger := game.CountAlive()
	dbg("checkWinConditions: crew=%d, tagger=%d", crew, tagger)

	// Tagger wins if they outnumber or equal the crew (1 vs 1 or fewer)
	if tagger > 0 && tagger >= crew {
		dbg("TAGGER WINS — calling endGame")
		endGame(room, "tagger")
		return true
	}

	// Crew wins if all taggers are eliminated
	if tagger == 0 {
		dbg("CREW WINS — calling endGame")
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

// ===== Sabotage handlers =====

func (c *Client) handleSabotage(payload json.RawMessage) {
	room := c.getRoom()
	if room == nil || room.Game == nil {
		return
	}

	var p SabotagePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return
	}

	now := time.Now()
	if !room.Game.TrySabotage(c.id, p.Type, now) {
		c.sendEnvelope(MsgError, ErrorPayload{Message: "Cannot sabotage right now"})
		return
	}

	// Calculate duration for the client
	duration := SabotageDuration.Seconds()
	if p.Type == SabotageMeltdown {
		duration = MeltdownDuration.Seconds()
	}

	room.broadcast(MsgSabotageStart, SabotageStartPayload{
		Type:     p.Type,
		Duration: duration,
		Stations: SabotageFixStations[p.Type],
	})

	// Send sabotage cooldown to tagger (not for meltdown since it's once per game)
	if p.Type != SabotageMeltdown {
		c.sendEnvelope(MsgCooldown, CooldownPayload{Seconds: SabotageCooldown.Seconds()})
	}
}

func (c *Client) handleSabotageFix(payload json.RawMessage) {
	room := c.getRoom()
	if room == nil || room.Game == nil {
		return
	}

	var p SabotageFixPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return
	}

	allFixed, ok := room.Game.TryFixSabotage(c.id, p.StationID)
	if !ok {
		return
	}

	if allFixed {
		room.broadcast(MsgSabotageEnd, SabotageEndPayload{})
	}
	// For meltdown partial fix, we don't broadcast end yet — the positions
	// payload's meltdownTimer keeps ticking until both are done.
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

	dbg("tag attempt: %s -> %s in room %s", c.id, p.TargetID, room.Code)
	bodyPos, ok := room.Game.TryTag(c.id, p.TargetID, time.Now())
	if !ok {
		dbg("tag rejected (cooldown/range/role)")
		return
	}
	dbg("tag SUCCESS: %s frozen at (%.0f, %.0f)", p.TargetID, bodyPos.X, bodyPos.Y)

	// Broadcast the freeze event
	room.broadcast(MsgPlayerFrozen, PlayerFrozenPayload{
		PlayerID: p.TargetID,
		Position: bodyPos,
	})
	dbg("broadcasted player_frozen")

	// Send cooldown to tagger (using room settings)
	room.Game.mu.RLock()
	tagCD := room.Game.Settings.TagCooldown
	room.Game.mu.RUnlock()
	c.sendEnvelope(MsgCooldown, CooldownPayload{
		Seconds: tagCD,
	})

	// Check win conditions
	dbg("calling checkWinConditions after tag")
	checkWinConditions(room)
	dbg("checkWinConditions returned")
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

	// Block emergency during active sabotage
	room.Game.mu.RLock()
	sabActive := room.Game.Sabotage.Active != ""
	room.Game.mu.RUnlock()
	if sabActive {
		c.sendEnvelope(MsgError, ErrorPayload{Message: "Fix the sabotage first!"})
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

	if room.Game.Meeting.CastVote(c.id, p.TargetID) {
		// Broadcast that this player voted (without revealing the target)
		room.broadcast(MsgVoteCast, VoteCastPayload{VoterID: c.id})
	}
}

// startMeeting begins a meeting in the given room. Runs the discussion+voting timeline in a goroutine.
func startMeeting(room *Room, callerID, reason, bodyID string) {
	game := room.Game
	if game == nil {
		return
	}

	alive := game.AlivePlayers()
	meeting := NewMeeting(callerID, reason, bodyID, alive)

	game.mu.Lock()
	if game.Meeting != nil {
		// already in a meeting
		game.mu.Unlock()
		return
	}
	game.Meeting = meeting
	game.mu.Unlock()

	// Teleport everyone to cafeteria
	game.TeleportToCafeteria()

	// Mark room phase
	room.mu.Lock()
	room.Phase = PhaseVoting
	room.mu.Unlock()

	// Use room settings for meeting durations
	game.mu.RLock()
	discTime := game.Settings.DiscussionTime
	voteTime := game.Settings.VotingTime
	game.mu.RUnlock()

	// Notify all players
	room.broadcast(MsgMeetingStart, MeetingStartPayload{
		CallerID:       callerID,
		Reason:         reason,
		BodyID:         bodyID,
		DiscussionTime: discTime,
		VotingTime:     voteTime,
		AlivePlayers:   alive,
	})

	// Run timeline in a goroutine
	go runMeetingTimeline(room, meeting, discTime, voteTime)
}

func runMeetingTimeline(room *Room, meeting *Meeting, discTime, voteTime float64) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("meeting timeline panic in room %s: %v", room.Code, r)
		}
	}()

	// Discussion phase
	time.Sleep(time.Duration(discTime * float64(time.Second)))
	meeting.SetPhase("voting")

	// Voting phase
	time.Sleep(time.Duration(voteTime * float64(time.Second)))

	// Tally
	ejectedID := meeting.TallyVotes()
	wasTagger := false

	// Capture the game ref locally — room.Game could be set to nil
	// concurrently if endGame fires (e.g. crewmate AI completed the
	// last task during the meeting).
	game := room.Game
	if game == nil {
		// Game already ended; meeting result is moot.
		return
	}

	if ejectedID != "" {
		game.mu.Lock()
		if game.Roles[ejectedID] == RoleTagger {
			wasTagger = true
		}
		game.Frozen[ejectedID] = true
		delete(game.BodyPos, ejectedID)
		game.mu.Unlock()
	}

	// Send results
	room.broadcast(MsgMeetingEnd, MeetingEndPayload{
		Votes:     meeting.GetVotes(),
		EjectedID: ejectedID,
		WasTagger: wasTagger,
	})

	// Brief pause for the reveal animation
	time.Sleep(3 * time.Second)

	// Clear meeting (re-check game ref in case endGame fired during sleep)
	game = room.Game
	if game != nil {
		game.mu.Lock()
		game.Meeting = nil
		game.TagCooldown = time.Now().Add(10 * time.Second)
		game.mu.Unlock()
	}

	// Resume gameplay
	room.mu.Lock()
	if room.Phase == PhaseVoting {
		room.Phase = PhasePlaying
	}
	room.mu.Unlock()

	// Check win conditions
	checkWinConditions(room)
}
