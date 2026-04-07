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

	// Meeting state (reset each meeting)
	MeetingVoted bool
	MeetingChatted bool
	VoteDelay      time.Duration // randomized per meeting so AIs don't all vote at once
}

const (
	AIWaypointReach = 24.0  // distance to consider waypoint reached
	AIDecisionDelay = 500 * time.Millisecond
)

var aiBotNames = []string{
	"Astro", "Cosmo", "Nova", "Pixel", "Zippy", "Blip", "Fizz", "Boop",
	"Bleep", "Whirr", "Spark", "Beam", "Comet", "Orbit",
}

func NewAIBrain(playerID string, role Role) *AIBrain {
	return &AIBrain{
		PlayerID:   playerID,
		Role:       role,
		Difficulty: AINormal,
		Goal:       GoalIdle,
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
		ai.VoteDelay = 0
	}

	pos := gs.Positions[ai.PlayerID]

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

	if ai.Role == RoleCrewmate {
		// Find an uncompleted task and go to it
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
		// Tagger: 50/50 fake a task or wander toward a player
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
	default:
		ai.Goal = GoalIdle
	}
}

// aiWander picks a random room to walk to
func aiWander(ai *AIBrain, pos Vec2) {
	target := DefaultWaypoints[rand.Intn(6)] // pick a random room
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

	// Send one quick chat during discussion phase
	if !ai.MeetingChatted && gs.Meeting.Phase == "discussion" {
		// Stagger: only send after a delay relative to meeting start
		elapsed := now.Sub(gs.Meeting.StartTime)
		if elapsed > 5*time.Second && rand.Float64() < 0.02 { // ~2% chance per tick
			ai.MeetingChatted = true
			messageID := pickAIChatMessage(ai)
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
		elapsed := now.Sub(gs.Meeting.StartTime) - DiscussionDuration
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

// pickAIChatMessage returns a quick message ID for the AI to send
func pickAIChatMessage(ai *AIBrain) string {
	// Crewmate AI: defends or asks where
	// Tagger AI: blends in with defense
	options := []string{"doing_task", "where", "idk", "trust_me", "with_me"}
	return options[rand.Intn(len(options))]
}

// pickAIVoteTarget decides who the AI votes for
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

	// Tagger AI: never votes for self, sometimes accuses based on chat
	// Crewmate AI: random pick if no info, with skip bias
	if len(candidates) == 0 {
		return ""
	}

	// 40% chance to skip (especially Normal)
	if rand.Float64() < 0.4 {
		return ""
	}

	// Tagger should not vote for fellow tagger (only one tagger anyway, so just exclude self)
	if ai.Role == RoleTagger {
		// Avoid voting for self, pick any other player
		return candidates[rand.Intn(len(candidates))]
	}

	// Crewmate: random
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
