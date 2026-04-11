package main

import (
	"log"
	"math"
	"math/rand"
	"time"
)

type AIDifficulty int

const (
	AIEasy AIDifficulty = iota
	AINormal
)

type AIGoal int

const (
	GoalIdle AIGoal = iota
	GoalGoToTask
	GoalDoingTask
	GoalWander
	GoalHunt
	GoalFakeTask
	GoalReportBody
	GoalFixSabotage
)

type AIBrain struct {
	PlayerID    string
	Role        Role
	Difficulty  AIDifficulty
	Goal        AIGoal
	GoalTarget  string // station ID or player ID
	Path        []Vec2
	PathIdx     int
	GoalDeadline time.Time // when current goal expires (e.g. finish task)
	NextDecision time.Time // throttle re-planning

	// Stuck detection
	LastPos    Vec2 // position last tick
	StuckTicks int  // consecutive ticks with <1px movement

	// Awareness — who was near whom (for voting)
	// Updated every ~500ms: tracks which players the AI has seen nearby
	NearbyLog    map[string]time.Time // playerID -> last time seen near this AI
	SeenNearBody map[string]bool      // playerID -> seen near a body (highly sus)

	// Meeting state (reset each meeting)
	MeetingVoted   bool
	MeetingChatted bool
	MeetingChats   int           // number of messages sent this meeting
	VoteDelay      time.Duration // randomized per meeting so AIs don't all vote at once
}

const (
	AIWaypointReach  = 24.0  // distance to consider waypoint reached
	AIDecisionDelay  = 500 * time.Millisecond
	AIStuckThreshold = 20    // ticks (~1 second) before stuck recovery
	AINearbyRange    = 200.0 // range to log nearby players
)

var aiBotNames = []string{
	"Astro", "Cosmo", "Nova", "Pixel", "Zippy", "Blip", "Fizz", "Boop",
	"Bleep", "Whirr", "Spark", "Beam", "Comet", "Orbit",
}

func NewAIBrain(playerID string, role Role) *AIBrain {
	return &AIBrain{
		PlayerID:     playerID,
		Role:         role,
		Difficulty:   AINormal,
		Goal:         GoalIdle,
		NearbyLog:    make(map[string]time.Time),
		SeenNearBody: make(map[string]bool),
	}
}

// AITick is called from the game loop while gs.mu is HELD.
// It updates the AI's MoveInputs and may complete tasks / freeze targets.
func AITick(gs *GameState, room *Room, ai *AIBrain, now time.Time) {
	// Skip AI if frozen — ghosts (especially crewmate ghosts) could still
	// do tasks, but for simplicity we just freeze AI in place when tagged.
	if gs.Frozen[ai.PlayerID] {
		gs.MoveInputs[ai.PlayerID] = Vec2{}
		return
	}

	// During meetings, AI brains pick a vote / send a message
	if gs.Meeting != nil {
		gs.MoveInputs[ai.PlayerID] = Vec2{}
		aiMeetingTick(gs, room, ai, now)
		return
	}

	// Reset meeting flags when no meeting
	if ai.MeetingVoted || ai.MeetingChatted {
		ai.MeetingVoted = false
		ai.MeetingChatted = false
		ai.MeetingChats = 0
		ai.VoteDelay = 0
		// Clear stale suspicion data after each meeting
		ai.SeenNearBody = make(map[string]bool)
	}

	pos := gs.Positions[ai.PlayerID]

	// --- Stuck detection ---
	dx := pos.X - ai.LastPos.X
	dy := pos.Y - ai.LastPos.Y
	movedSq := dx*dx + dy*dy
	if movedSq < 1.0 && ai.Goal != GoalDoingTask && ai.Goal != GoalFakeTask && ai.Goal != GoalIdle {
		ai.StuckTicks++
	} else {
		ai.StuckTicks = 0
	}
	ai.LastPos = pos

	// If stuck too long, abandon current goal and path to nearest waypoint
	if ai.StuckTicks >= AIStuckThreshold {
		ai.StuckTicks = 0
		ai.Goal = GoalIdle
		ai.GoalTarget = ""
		// Find nearest walkable waypoint and path there to unstick
		wpIdx := nearestWaypoint(pos)
		wp := DefaultWaypoints[wpIdx]
		ai.Path = []Vec2{wp}
		ai.PathIdx = 0
		ai.Goal = GoalWander
		ai.NextDecision = now.Add(AIDecisionDelay * 3) // give extra time to reach waypoint
	}

	// --- Awareness: log nearby players (for voting intelligence) ---
	if now.After(ai.NextDecision) {
		for otherID, otherPos := range gs.Positions {
			if otherID == ai.PlayerID || gs.Frozen[otherID] {
				continue
			}
			odx := pos.X - otherPos.X
			ody := pos.Y - otherPos.Y
			if odx*odx+ody*ody <= AINearbyRange*AINearbyRange {
				ai.NearbyLog[otherID] = now
			}
		}
		// Check if any alive player is near a body (sus!)
		for bodyID, bodyPos := range gs.BodyPos {
			if gs.BodyReported[bodyID] {
				continue
			}
			for otherID, otherPos := range gs.Positions {
				if otherID == ai.PlayerID || gs.Frozen[otherID] {
					continue
				}
				bdx := otherPos.X - bodyPos.X
				bdy := otherPos.Y - bodyPos.Y
				if bdx*bdx+bdy*bdy <= 150*150 {
					ai.SeenNearBody[otherID] = true
				}
			}
		}
	}

	// Periodically re-plan / pick a new goal
	if now.After(ai.NextDecision) {
		aiPickGoal(gs, ai, pos)
		ai.NextDecision = now.Add(AIDecisionDelay)
	}

	// Body discovery — only crewmate AI reports bodies
	if ai.Role == RoleCrewmate {
		for bodyID, bodyPos := range gs.BodyPos {
			if gs.BodyReported[bodyID] {
				continue
			}
			dx := pos.X - bodyPos.X
			dy := pos.Y - bodyPos.Y
			if dx*dx+dy*dy <= ReportRange*ReportRange {
				gs.BodyReported[bodyID] = true
				// Trigger meeting in a goroutine (must release lock first)
				go func(r *Room, callerID, bid string) {
					defer func() {
						if rec := recover(); rec != nil {
							log.Printf("ai start meeting panic: %v", rec)
						}
					}()
					startMeeting(r, callerID, "body", bid)
				}(room, ai.PlayerID, bodyID)
				return
			}
		}
	}

	// Tagger: try to tag if conditions are right
	if ai.Role == RoleTagger {
		aiTaggerTryTag(gs, room, ai, pos, now)
	}

	// Doing a task: stand still until duration expires
	if ai.Goal == GoalDoingTask || ai.Goal == GoalFakeTask {
		gs.MoveInputs[ai.PlayerID] = Vec2{}
		if now.After(ai.GoalDeadline) {
			if ai.Goal == GoalDoingTask && ai.Role == RoleCrewmate {
				// Complete the task
				if gs.Tasks.CompleteTask(ai.PlayerID, ai.GoalTarget, RoleCrewmate) {
					progress := gs.Tasks.Progress()
					allDone := gs.Tasks.AllTasksDone()
					// Send progress update to all players (async, panic-safe)
					go func(r *Room, prog float64, done bool) {
						defer func() {
							if rec := recover(); rec != nil {
								log.Printf("ai task complete broadcast panic: %v", rec)
							}
						}()
						for _, id := range r.snapshotPlayerIDs() {
							game := r.Game
							if game == nil {
								return
							}
							tasks := game.Tasks.GetPlayerTasks(id)
							r.sendTo(id, MsgTaskProgress, TaskProgressPayload{
								Progress: prog,
								Tasks:    tasks,
							})
						}
						if done {
							endGame(r, "crew")
						}
					}(room, progress, allDone)
				}
			}
			ai.Goal = GoalIdle
			ai.GoalTarget = ""
			ai.NextDecision = now // re-plan immediately
		}
		return
	}

	// Move along the path
	if ai.PathIdx < len(ai.Path) {
		target := ai.Path[ai.PathIdx]
		dx := target.X - pos.X
		dy := target.Y - pos.Y
		distSq := dx*dx + dy*dy

		if distSq < AIWaypointReach*AIWaypointReach {
			// Reached this waypoint, move to next
			ai.PathIdx++
			if ai.PathIdx >= len(ai.Path) {
				// Arrived at final destination
				gs.MoveInputs[ai.PlayerID] = Vec2{}
				aiOnArrive(gs, ai, now)
				return
			}
			target = ai.Path[ai.PathIdx]
			dx = target.X - pos.X
			dy = target.Y - pos.Y
		}

		// Set movement direction
		mag := math.Sqrt(dx*dx + dy*dy)
		if mag > 0 {
			gs.MoveInputs[ai.PlayerID] = Vec2{dx / mag, dy / mag}
		}
		return
	}

	// No path, idle
	gs.MoveInputs[ai.PlayerID] = Vec2{}
}

// aiPickGoal selects a new goal based on role
func aiPickGoal(gs *GameState, ai *AIBrain, pos Vec2) {
	if ai.Goal != GoalIdle && ai.Goal != GoalWander {
		return
	}

	now := time.Now()

	if ai.Role == RoleCrewmate {
		// Priority 1: fix active sabotage (50% chance to volunteer)
		if gs.Sabotage.Active != "" && rand.Float64() < 0.5 {
			stations := SabotageFixStations[gs.Sabotage.Active]
			if len(stations) > 0 {
				// For meltdown, pick a random station; for others, there's only one
				station := stations[rand.Intn(len(stations))]
				// Don't go to an already-fixed meltdown station
				if gs.Sabotage.Active != SabotageMeltdown || !gs.Sabotage.MeltdownFixed[station.ID] {
					ai.Goal = GoalFixSabotage
					ai.GoalTarget = station.ID
					ai.Path = FindPath(pos, station.Position)
					ai.PathIdx = 0
					return
				}
			}
		}

		// Priority 2: Find an uncompleted task and go to it
		assignments := gs.Tasks.Assignments[ai.PlayerID]
		for _, a := range assignments {
			if !a.Completed {
				station := findStation(gs, a.StationID)
				if station != nil {
					ai.Goal = GoalGoToTask
					ai.GoalTarget = a.StationID
					ai.Path = FindPath(pos, station.Position)
					ai.PathIdx = 0
					return
				}
			}
		}
		// All tasks done — wander
		aiWander(ai, pos)
	} else {
		// Tagger: try to trigger sabotage if conditions allow
		if gs.Sabotage.Active == "" && now.After(gs.SabotageCooldown) {
			chance := 0.15
			// Meltdown is rarer and only if not already used
			if !gs.Sabotage.UsedMeltdown && rand.Float64() < 0.05 {
				aiTriggerSabotage(gs, gs.Room, ai, SabotageMeltdown, now)
				return
			}
			if rand.Float64() < chance {
				types := []SabotageType{SabotageLightsOut, SabotageCommsDown}
				aiTriggerSabotage(gs, gs.Room, ai, types[rand.Intn(2)], now)
				return
			}
		}

		// 50/50 fake a task or wander toward a player
		if rand.Float64() < 0.5 {
			// Fake task: walk to a station
			assignments := gs.Tasks.Assignments[ai.PlayerID]
			if len(assignments) > 0 {
				a := assignments[rand.Intn(len(assignments))]
				station := findStation(gs, a.StationID)
				if station != nil {
					ai.Goal = GoalGoToTask
					ai.GoalTarget = a.StationID
					ai.Path = FindPath(pos, station.Position)
					ai.PathIdx = 0
					return
				}
			}
		}
		// Wander toward another player
		aiHunt(gs, ai, pos)
	}
}

// aiTriggerSabotage activates a sabotage from AI tagger (lock is held)
func aiTriggerSabotage(gs *GameState, room *Room, ai *AIBrain, sabType SabotageType, now time.Time) {
	// Apply sabotage directly (lock is held)
	switch sabType {
	case SabotageLightsOut, SabotageCommsDown:
		gs.Sabotage.Active = sabType
		gs.Sabotage.ExpiresAt = now.Add(SabotageDuration)
		gs.SabotageCooldown = now.Add(SabotageCooldown)
	case SabotageMeltdown:
		if gs.Sabotage.UsedMeltdown {
			return
		}
		gs.Sabotage.Active = sabType
		gs.Sabotage.ExpiresAt = now.Add(MeltdownDuration)
		gs.Sabotage.UsedMeltdown = true
		gs.Sabotage.MeltdownFixed = make(map[string]bool)
	}

	dbg("AI tagger %s triggered %s", ai.PlayerID, sabType)

	duration := SabotageDuration.Seconds()
	if sabType == SabotageMeltdown {
		duration = MeltdownDuration.Seconds()
	}

	go func(r *Room) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("ai sabotage broadcast panic: %v", rec)
			}
		}()
		r.broadcast(MsgSabotageStart, SabotageStartPayload{
			Type:     sabType,
			Duration: duration,
			Stations: SabotageFixStations[sabType],
		})
	}(room)
}

// aiOnArrive handles what happens when AI reaches its destination
func aiOnArrive(gs *GameState, ai *AIBrain, now time.Time) {
	switch ai.Goal {
	case GoalGoToTask:
		// "Perform" the task — stand still for a few seconds
		duration := time.Duration(4+rand.Intn(4)) * time.Second
		ai.GoalDeadline = now.Add(duration)
		if ai.Role == RoleCrewmate {
			ai.Goal = GoalDoingTask
		} else {
			ai.Goal = GoalFakeTask
		}
	case GoalHunt:
		// Just arrived near a target; tagger logic will handle the freeze
		ai.Goal = GoalIdle
	case GoalFixSabotage:
		// AI crewmate arrived at fix station — fix it directly (lock is held)
		stationID := ai.GoalTarget
		if gs.Sabotage.Active != "" {
			stations := SabotageFixStations[gs.Sabotage.Active]
			var station *FixStation
			for i := range stations {
				if stations[i].ID == stationID {
					station = &stations[i]
					break
				}
			}
			if station != nil {
				// Range check
				pos := gs.Positions[ai.PlayerID]
				dx := pos.X - station.Position.X
				dy := pos.Y - station.Position.Y
				if dx*dx+dy*dy <= SabotageFixRange*SabotageFixRange {
					allFixed := false
					if gs.Sabotage.Active == SabotageMeltdown {
						gs.Sabotage.MeltdownFixed[stationID] = true
						if len(gs.Sabotage.MeltdownFixed) >= 2 {
							gs.Sabotage.Active = ""
							allFixed = true
						}
					} else {
						gs.Sabotage.Active = ""
						allFixed = true
					}
					if allFixed {
						go func(r *Room) {
							defer func() {
								if rec := recover(); rec != nil {
									log.Printf("ai fix sabotage broadcast panic: %v", rec)
								}
							}()
							r.broadcast(MsgSabotageEnd, SabotageEndPayload{})
						}(gs.Room)
					}
				}
			}
		}
		ai.Goal = GoalIdle
	default:
		ai.Goal = GoalIdle
	}
}

// aiWander picks a random room to walk to
func aiWander(ai *AIBrain, pos Vec2) {
	// Pick a random room waypoint (indices 0-5), but also consider secondary
	// waypoints (13+) so AI visits different parts of rooms
	allRoom := []int{0, 1, 2, 3, 4, 5, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22}
	target := DefaultWaypoints[allRoom[rand.Intn(len(allRoom))]]
	ai.Goal = GoalWander
	ai.Path = FindPath(pos, target)
	ai.PathIdx = 0
}

// aiHunt picks a random alive player to walk toward
func aiHunt(gs *GameState, ai *AIBrain, pos Vec2) {
	candidates := []string{}
	for id := range gs.Positions {
		if id == ai.PlayerID {
			continue
		}
		if gs.Frozen[id] {
			continue
		}
		candidates = append(candidates, id)
	}
	if len(candidates) == 0 {
		aiWander(ai, pos)
		return
	}
	target := candidates[rand.Intn(len(candidates))]
	targetPos := gs.Positions[target]
	ai.Goal = GoalHunt
	ai.GoalTarget = target
	ai.Path = FindPath(pos, targetPos)
	ai.PathIdx = 0
}

// aiTaggerTryTag attempts to freeze a nearby crewmate if no witnesses are around
func aiTaggerTryTag(gs *GameState, room *Room, ai *AIBrain, pos Vec2, now time.Time) {
	if now.Before(gs.TagCooldown) {
		return
	}

	// Find nearest crewmate within tag range
	var nearestID string
	nearestDist := math.MaxFloat64
	for id, role := range gs.Roles {
		if id == ai.PlayerID || role != RoleCrewmate {
			continue
		}
		if gs.Frozen[id] {
			continue
		}
		other := gs.Positions[id]
		dx := pos.X - other.X
		dy := pos.Y - other.Y
		distSq := dx*dx + dy*dy
		if distSq < nearestDist && distSq <= TagRange*TagRange {
			nearestDist = distSq
			nearestID = id
		}
	}
	if nearestID == "" {
		return
	}

	// Witness check (Normal difficulty): no other crewmate within 250 units
	if ai.Difficulty == AINormal {
		for id, role := range gs.Roles {
			if id == ai.PlayerID || id == nearestID || role != RoleCrewmate {
				continue
			}
			if gs.Frozen[id] {
				continue
			}
			other := gs.Positions[id]
			dx := pos.X - other.X
			dy := pos.Y - other.Y
			if dx*dx+dy*dy <= 250*250 {
				return // witness present, abort
			}
		}
	}

	// Tag it (we already hold the lock so manipulate state directly)
	dbg("AI tagger %s freezing %s", ai.PlayerID, nearestID)
	gs.Frozen[nearestID] = true
	gs.BodyPos[nearestID] = gs.Positions[nearestID]
	gs.MoveInputs[nearestID] = Vec2{}
	gs.TagCooldown = now.Add(TagCooldown)

	// Broadcast freeze event in goroutine to avoid lock issues
	go func(r *Room, victimID string, bodyPos Vec2) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("ai tag broadcast panic: %v", rec)
			}
		}()
		r.broadcast(MsgPlayerFrozen, PlayerFrozenPayload{
			PlayerID: victimID,
			Position: bodyPos,
		})
		checkWinConditions(r)
	}(room, nearestID, gs.Positions[nearestID])
}

// aiMeetingTick handles AI behavior during a meeting
func aiMeetingTick(gs *GameState, room *Room, ai *AIBrain, now time.Time) {
	if gs.Meeting == nil {
		return
	}

	// Send chat messages during discussion phase (up to 3 per meeting)
	if ai.MeetingChats < 3 && gs.Meeting.Phase == "discussion" {
		// Stagger: only send after a delay relative to meeting start
		elapsed := now.Sub(gs.Meeting.StartTime)
		if elapsed > 3*time.Second && rand.Float64() < 0.03 { // ~3% chance per tick
			ai.MeetingChats++
			ai.MeetingChatted = true
			messageID := pickAIChatMessage(gs, ai)
			if messageID != "" && gs.Meeting.CanSendChat(ai.PlayerID) {
				gs.Meeting.RecordChat(ai.PlayerID)
				go func(r *Room, sender, msg string) {
					defer func() {
						if rec := recover(); rec != nil {
							log.Printf("ai chat broadcast panic: %v", rec)
						}
					}()
					r.broadcast(MsgChatMessage, ChatMessagePayload{
						SenderID:  sender,
						MessageID: msg,
					})
				}(room, ai.PlayerID, messageID)
			}
		}
	}

	// Vote during voting phase
	if !ai.MeetingVoted && gs.Meeting.Phase == "voting" {
		// Randomize each AI's vote delay so they don't all vote at the same instant
		if ai.VoteDelay == 0 {
			ai.VoteDelay = time.Duration(2+rand.Intn(8)) * time.Second
		}
		discDur := time.Duration(gs.Settings.DiscussionTime * float64(time.Second))
		elapsed := now.Sub(gs.Meeting.StartTime) - discDur
		if elapsed > ai.VoteDelay {
			targetID := pickAIVoteTarget(gs, ai)
			if gs.Meeting.CastVote(ai.PlayerID, targetID) {
				ai.MeetingVoted = true
				// Broadcast vote_cast in a goroutine (we hold gs.mu here)
				go func(r *Room, voter string) {
					defer func() {
						if rec := recover(); rec != nil {
							log.Printf("ai vote broadcast panic: %v", rec)
						}
					}()
					r.broadcast(MsgVoteCast, VoteCastPayload{VoterID: voter})
				}(room, ai.PlayerID)
			}
		}
	}
}

// pickAIChatMessage returns a contextual quick message ID for the AI to send
func pickAIChatMessage(gs *GameState, ai *AIBrain) string {
	if ai.Role == RoleCrewmate {
		// Crewmate: share useful info
		// If seen someone near a body, accuse them
		for playerID := range ai.SeenNearBody {
			// Find this player's color to send the right "sus" message
			if color, ok := getPlayerColor(gs, playerID); ok {
				msgID := "sus_" + color
				return msgID
			}
		}
		// Share current location
		pos := gs.Positions[ai.PlayerID]
		room := gs.Map.GetRoomAt(pos)
		if room != "" {
			locMsg := "loc_" + room
			// 50% location, 50% other info
			if rand.Float64() < 0.5 {
				return locMsg
			}
		}
		// Mix of defense/info
		options := []string{"doing_task", "where", "idk", "trust_me", "with_me", "i_saw"}
		return options[rand.Intn(len(options))]
	}

	// Tagger: deflect and accuse others
	// Pick a random alive crewmate to accuse
	if rand.Float64() < 0.5 {
		for _, id := range gs.Meeting.AlivePlayers {
			if id == ai.PlayerID || gs.Frozen[id] {
				continue
			}
			if gs.Roles[id] == RoleCrewmate {
				if color, ok := getPlayerColor(gs, id); ok {
					return "sus_" + color
				}
			}
		}
	}

	// Defense messages
	options := []string{"not_me", "doing_task", "trust_me", "where", "idk"}
	return options[rand.Intn(len(options))]
}

// getPlayerColor returns the color name for a player ID from the pre-built map.
// This avoids taking Room.mu while gs.mu is held (which would risk deadlock).
func getPlayerColor(gs *GameState, playerID string) (string, bool) {
	color, ok := gs.PlayerColors[playerID]
	return color, ok
}

// pickAIVoteTarget decides who the AI votes for using suspicion data
func pickAIVoteTarget(gs *GameState, ai *AIBrain) string {
	if gs.Meeting == nil {
		return ""
	}

	alive := gs.Meeting.AlivePlayers
	candidates := []string{}
	for _, id := range alive {
		if id == ai.PlayerID {
			continue
		}
		candidates = append(candidates, id)
	}

	if len(candidates) == 0 {
		return ""
	}

	if ai.Role == RoleCrewmate {
		// --- Crewmate voting logic ---
		// Priority 1: vote for someone seen near a body (very sus)
		var susPlayers []string
		for _, id := range candidates {
			if ai.SeenNearBody[id] {
				susPlayers = append(susPlayers, id)
			}
		}
		if len(susPlayers) > 0 {
			return susPlayers[rand.Intn(len(susPlayers))]
		}

		// Priority 2: vote for someone NOT seen recently (strangers are sus)
		// Players the AI hasn't encountered are more likely to be the tagger
		// skulking in a different part of the map
		var unseenPlayers []string
		recentThreshold := time.Now().Add(-15 * time.Second)
		for _, id := range candidates {
			lastSeen, seen := ai.NearbyLog[id]
			if !seen || lastSeen.Before(recentThreshold) {
				unseenPlayers = append(unseenPlayers, id)
			}
		}
		if len(unseenPlayers) > 0 && rand.Float64() < 0.6 {
			return unseenPlayers[rand.Intn(len(unseenPlayers))]
		}

		// 25% chance to skip if no strong leads
		if rand.Float64() < 0.25 {
			return ""
		}

		// Fallback: random
		return candidates[rand.Intn(len(candidates))]
	}

	// --- Tagger voting logic ---
	// Strategy: deflect suspicion

	// Try to vote for the meeting caller (deflect attention onto them)
	callerID := gs.Meeting.CallerID
	if callerID != ai.PlayerID && rand.Float64() < 0.4 {
		for _, id := range candidates {
			if id == callerID {
				return id
			}
		}
	}

	// Vote for someone who was near the body (make it look like you're "helping")
	for _, id := range candidates {
		if ai.SeenNearBody[id] && rand.Float64() < 0.5 {
			return id
		}
	}

	// 20% chance to skip (taggers skip less to avoid looking passive)
	if rand.Float64() < 0.2 {
		return ""
	}

	return candidates[rand.Intn(len(candidates))]
}

func findStation(gs *GameState, stationID string) *TaskStation {
	for i := range gs.Tasks.Stations {
		if gs.Tasks.Stations[i].ID == stationID {
			return &gs.Tasks.Stations[i]
		}
	}
	return nil
}
