package main

// AI players run on a small Utility AI:
//
//   - ai.go            brain state, the per-tick loop, path following
//   - ai_perception.go what a bot can actually see and hear (suspicion)
//   - ai_utility.go    scoring every possible action and picking the best
//   - ai_meeting.go    chatting and voting during meetings
//
// Every AIDecisionDelay a bot lists the actions open to it (do this task,
// report that body, hunt that player, ...), scores each one from 0 to 1,
// adds a little noise and a bonus for whatever it's already doing, and
// commits to the winner. Tuning is a matter of adjusting the score formulas
// in ai_utility.go.

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
	AIHard
)

// ParseDifficulty maps the lobby setting to a profile (unknown -> Normal).
func ParseDifficulty(s string) AIDifficulty {
	switch s {
	case "easy":
		return AIEasy
	case "hard":
		return AIHard
	}
	return AINormal
}

// aiProfile holds the knobs that differ between difficulty levels. Bots
// never see more than a human could at any level; harder bots just pay
// closer attention, remember longer, and make fewer mistakes.
type aiProfile struct {
	NoticeChance  float64 // chance a crewmate bot registers a clue it could see
	Carelessness  float64 // chance a tagger bot ignores witnesses for its next tag
	ScoreNoise    float64 // random jitter on utility scores (higher = sloppier)
	SusDecay      float64 // suspicion kept after each meeting (witnessed tags stay)
	TaskTimeScale float64 // multiplier on how long a bot stands at a task
}

var aiProfiles = map[AIDifficulty]aiProfile{
	AIEasy:   {NoticeChance: 0.5, Carelessness: 1.0, ScoreNoise: 0.3, SusDecay: 0.4, TaskTimeScale: 1.25},
	AINormal: {NoticeChance: 1.0, Carelessness: 0.35, ScoreNoise: 0.1, SusDecay: 0.5, TaskTimeScale: 1.0},
	AIHard:   {NoticeChance: 1.0, Carelessness: 0.0, ScoreNoise: 0.05, SusDecay: 0.75, TaskTimeScale: 0.8},
}

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
	GoalLeaveScene
	GoalSabotage // instant: fires a sabotage, then the bot keeps its current goal
)

type AIBrain struct {
	PlayerID     string
	Role         Role
	Difficulty   AIDifficulty
	Personality  Personality
	Goal         AIGoal
	GoalTarget   string // station ID, player ID, body ID, or waypoint index
	GoalPos      Vec2   // where the goal was when we planned the path
	Path         []Vec2
	PathIdx      int
	GoalDeadline time.Time // when current goal expires (e.g. finish task)
	NextDecision time.Time // throttle re-planning

	// Stuck detection
	LastPos    Vec2                 // position last tick
	StuckTicks int                  // consecutive ticks with <1px movement
	Avoid      map[string]time.Time // option key -> skip it until this time

	// Perception — only what this bot could plausibly know
	Suspicion   map[string]float64              // playerID -> 0..1+ how sus they look
	Witnessed   map[string]bool                 // playerID -> saw them tag someone
	KnownBodies map[string]bool                 // unreported bodies this bot has seen
	SeenAtBody  map[string]map[string]bool      // bodyID -> players already counted near it
	NearbyLog   map[string]time.Time            // playerID -> last time seen nearby
	LastRoom    string                          // last room the bot was in (for "I was in X")
	SeenInRoom  map[string]string               // playerID -> room the bot last saw them in
	SeenWith    map[string]map[string]time.Time // a -> b -> last time seen close together

	// Tagger teamwork: partners are known, but taggers can't talk privately,
	// so all cooperation comes from what the bot sees and hears in chat.
	Partners       []string
	PartnerTagSeen map[string]time.Time // partner -> last time I saw them tag
	RoundStart     time.Time            // game start or last meeting end (everyone's cooldown reset)

	// Tagger timing
	LastTagAt      time.Time
	Careless       bool // this tag, ignore witnesses (rolled after each tag)
	LastSabotageAt time.Time

	LastFledAt time.Time // Nervous: last time it walked away from someone

	// Meeting state (reset each meeting)
	InMeeting    bool
	MeetingVoted bool
	MeetingChats int
	LastChat     string
	VoteDelay    time.Duration              // randomized so bots don't all vote at once
	Accusations  map[string]int             // playerID -> times others accused them this meeting
	Accusers     map[string]map[string]bool // target -> who accused them this meeting
	HeardFrom    map[string]bool            // "sender>target" pairs already counted
	AccusedMe    int                        // times someone accused this bot this meeting
	BodyRoom     string                     // where this meeting's body was found
	SaidISaw     bool
	SaidNotMe    bool
	SaidDefend   bool // already stuck up for a partner this meeting
	SaidWithMe   bool
}

const (
	AIWaypointReach  = 24.0 // distance to consider the destination reached
	AICornerReach    = 8.0  // corners on the way must be hit closely, or bots clip furniture
	AIDecisionDelay  = 500 * time.Millisecond
	AIStuckThreshold = 20 // ticks (~1 second) before stuck recovery
	AIStuckAvoidFor  = 8 * time.Second
	AIVisionRadius   = 250.0 // matches the client's fog of war
	AIReducedVision  = 100.0 // during Lights Out
	AINearBodyRange  = 150.0 // standing this close to a body looks sus

	// How long a bot stands at a task. Roughly how long a young kid takes
	// on the mini-games — shorter makes the bot crew win before a human
	// tagger gets a chance to play.
	AITaskTimeMin = 7 * time.Second
	AITaskTimeMax = 12 * time.Second
)

func NewAIBrain(playerID string, role Role, difficulty AIDifficulty) *AIBrain {
	return &AIBrain{
		Careless:       rand.Float64() < aiProfiles[difficulty].Carelessness,
		PlayerID:       playerID,
		Role:           role,
		Difficulty:     difficulty,
		Goal:           GoalIdle,
		Avoid:          make(map[string]time.Time),
		Suspicion:      make(map[string]float64),
		Witnessed:      make(map[string]bool),
		KnownBodies:    make(map[string]bool),
		SeenAtBody:     make(map[string]map[string]bool),
		NearbyLog:      make(map[string]time.Time),
		SeenInRoom:     make(map[string]string),
		SeenWith:       make(map[string]map[string]time.Time),
		LastSabotageAt: time.Now(),
		RoundStart:     time.Now(),
		PartnerTagSeen: make(map[string]time.Time),
		Accusations:    make(map[string]int),
		Accusers:       make(map[string]map[string]bool),
		HeardFrom:      make(map[string]bool),
	}
}

// AITick is called from the game loop while gs.mu is HELD.
// It updates the AI's MoveInputs and may complete tasks / freeze targets.
func AITick(gs *GameState, room *Room, ai *AIBrain, now time.Time) {
	frozen := gs.Frozen[ai.PlayerID]

	// A frozen tagger has nothing left to do. Frozen crewmates keep going
	// as ghosts so their tasks still count toward a crew win.
	if frozen && ai.Role == RoleTagger {
		gs.MoveInputs[ai.PlayerID] = Vec2{}
		return
	}

	// During meetings, AI brains pick a vote / send a message
	if gs.Meeting != nil {
		if !ai.InMeeting {
			aiOnMeetingStart(gs, ai, now)
		}
		gs.MoveInputs[ai.PlayerID] = Vec2{}
		if !frozen {
			aiMeetingTick(gs, room, ai, now)
		}
		return
	}
	if ai.InMeeting {
		aiOnMeetingEnd(gs, ai, now)
	}

	pos := gs.Positions[ai.PlayerID]
	aiCheckStuck(ai, pos, now)
	// Updated every tick so "I was in X" matches what others last saw
	if room := gs.Map.GetRoomAt(pos); room != "" {
		ai.LastRoom = room
	}

	if !now.Before(ai.NextDecision) {
		if !frozen {
			aiPerceive(gs, ai, pos, now)
		}
		aiDecide(gs, room, ai, pos, now)
		ai.NextDecision = now.Add(AIDecisionDelay)
	}

	if ai.Role == RoleTagger {
		aiTaggerTryTag(gs, room, ai, pos, now)
	}

	// Doing a task: stand still until duration expires
	if ai.Goal == GoalDoingTask || ai.Goal == GoalFakeTask {
		gs.MoveInputs[ai.PlayerID] = Vec2{}
		if now.After(ai.GoalDeadline) {
			if ai.Goal == GoalDoingTask {
				aiFinishTask(gs, room, ai)
			}
			ai.Goal = GoalIdle
			ai.GoalTarget = ""
			ai.NextDecision = now // re-plan immediately
		}
		return
	}

	aiFollowPath(gs, room, ai, pos, now)
}

// aiCheckStuck notices when a bot has been pressing into a wall and makes
// it give up on that action for a while so the utility picker chooses
// something else instead of retrying the same blocked route forever.
func aiCheckStuck(ai *AIBrain, pos Vec2, now time.Time) {
	dx := pos.X - ai.LastPos.X
	dy := pos.Y - ai.LastPos.Y
	ai.LastPos = pos
	moving := ai.PathIdx < len(ai.Path) && ai.Goal != GoalDoingTask && ai.Goal != GoalFakeTask
	if !moving || dx*dx+dy*dy >= 1.0 {
		ai.StuckTicks = 0
		return
	}
	ai.StuckTicks++
	if ai.StuckTicks < AIStuckThreshold {
		return
	}
	dbg("AI %s stuck at (%.0f,%.0f) on goal %d/%s", ai.PlayerID, pos.X, pos.Y, ai.Goal, ai.GoalTarget)
	ai.StuckTicks = 0
	ai.Avoid[aiOptionKey(ai.Goal, ai.GoalTarget)] = now.Add(AIStuckAvoidFor)
	ai.Goal = GoalIdle
	ai.GoalTarget = ""
	ai.Path = nil
	ai.PathIdx = 0
	ai.NextDecision = now
}

// aiSetPath plans a route to `to`. Ghosts fly straight through walls.
func aiSetPath(gs *GameState, ai *AIBrain, from, to Vec2) {
	if gs.Frozen[ai.PlayerID] {
		ai.Path = []Vec2{to}
	} else {
		ai.Path = FindPath(from, to)
	}
	ai.PathIdx = 0
	ai.GoalPos = to
}

func aiFollowPath(gs *GameState, room *Room, ai *AIBrain, pos Vec2, now time.Time) {
	if ai.PathIdx >= len(ai.Path) {
		gs.MoveInputs[ai.PlayerID] = Vec2{}
		return
	}

	target := ai.Path[ai.PathIdx]
	dx := target.X - pos.X
	dy := target.Y - pos.Y
	reach := AICornerReach
	if ai.PathIdx == len(ai.Path)-1 {
		reach = AIWaypointReach
	}
	if dx*dx+dy*dy < reach*reach {
		// Reached this waypoint, move to next
		ai.PathIdx++
		if ai.PathIdx >= len(ai.Path) {
			// Arrived at final destination
			gs.MoveInputs[ai.PlayerID] = Vec2{}
			aiOnArrive(gs, room, ai, pos, now)
			return
		}
		target = ai.Path[ai.PathIdx]
		dx = target.X - pos.X
		dy = target.Y - pos.Y
	}

	mag := math.Sqrt(dx*dx + dy*dy)
	if mag > 0 {
		gs.MoveInputs[ai.PlayerID] = Vec2{dx / mag, dy / mag}
	}
}

// aiOnArrive handles what happens when AI reaches its destination
func aiOnArrive(gs *GameState, room *Room, ai *AIBrain, pos Vec2, now time.Time) {
	switch ai.Goal {
	case GoalGoToTask:
		// "Perform" the task — stand still for a few seconds
		taskTime := AITaskTimeMin + time.Duration(rand.Int63n(int64(AITaskTimeMax-AITaskTimeMin)))
		ai.GoalDeadline = now.Add(time.Duration(float64(taskTime) * aiProfiles[ai.Difficulty].TaskTimeScale * ai.weights().TaskTimeScale))
		if ai.Role == RoleCrewmate {
			ai.Goal = GoalDoingTask
		} else {
			ai.Goal = GoalFakeTask
		}
		return

	case GoalReportBody:
		bodyID := ai.GoalTarget
		bodyPos, exists := gs.BodyPos[bodyID]
		if exists && !gs.BodyReported[bodyID] && withinRange(pos, bodyPos, ReportRange) {
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
		}

	case GoalFixSabotage:
		if allFixed, ok := gs.fixSabotageLocked(ai.PlayerID, ai.GoalTarget); ok && allFixed {
			go func(r *Room) {
				defer func() {
					if rec := recover(); rec != nil {
						log.Printf("ai fix sabotage broadcast panic: %v", rec)
					}
				}()
				r.broadcast(MsgSabotageEnd, SabotageEndPayload{})
			}(room)
		}
	}

	ai.Goal = GoalIdle
	ai.GoalTarget = ""
	ai.NextDecision = now
}

// aiFinishTask marks the bot's current task complete (ghosts included) and
// broadcasts progress, ending the game if that was the last task.
func aiFinishTask(gs *GameState, room *Room, ai *AIBrain) {
	if !gs.Tasks.CompleteTask(ai.PlayerID, ai.GoalTarget, RoleCrewmate) {
		return
	}
	allDone := gs.Tasks.AllTasksDone()
	// Snapshot while we hold the lock — the goroutine must not touch gs.Tasks
	payloads := gs.taskProgressLocked()
	go func(r *Room, done bool) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("ai task complete broadcast panic: %v", rec)
			}
		}()
		r.sendTaskProgress(payloads)
		if done {
			endGame(r, "crew")
		}
	}(room, allDone)
}

// aiTriggerSabotage activates a sabotage from AI tagger (lock is held)
func aiTriggerSabotage(gs *GameState, room *Room, ai *AIBrain, sabType SabotageType, now time.Time) bool {
	if !gs.trySabotageLocked(ai.PlayerID, sabType, now) {
		return false
	}
	ai.LastSabotageAt = now
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
	return true
}

// aiTaggerTryTag freezes a crewmate in range if the coast is clear
func aiTaggerTryTag(gs *GameState, room *Room, ai *AIBrain, pos Vec2, now time.Time) {
	if now.Before(gs.TagCooldowns[ai.PlayerID]) {
		return
	}

	// Find nearest crewmate within tag range
	var nearestID string
	nearestDist := math.MaxFloat64
	for id, role := range gs.Roles {
		if id == ai.PlayerID || role != RoleCrewmate || gs.Frozen[id] {
			continue
		}
		d := distSq(pos, gs.Positions[id])
		if d < nearestDist && d <= TagRange*TagRange {
			nearestDist = d
			nearestID = id
		}
	}
	if nearestID == "" {
		return
	}

	// Witness check: no other crewmate close enough to see it happen
	// (unless this is one of the tagger's careless moments)
	if !ai.Careless && aiCrewWatching(gs, pos, ai.PlayerID, nearestID) > 0 {
		return
	}

	bodyPos, ok := gs.tagLocked(ai.PlayerID, nearestID, now)
	if !ok {
		return
	}
	ai.LastTagAt = now
	ai.Careless = rand.Float64() < aiProfiles[ai.Difficulty].Carelessness
	partnerCDs := gs.partnerCooldownsLocked(ai.PlayerID, now)
	dbg("AI tagger %s froze %s", ai.PlayerID, nearestID)

	// Broadcast freeze event in goroutine to avoid lock issues
	go func(r *Room, victimID string, bodyPos Vec2) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("ai tag broadcast panic: %v", rec)
			}
		}()
		if r.Game == nil {
			return
		}
		r.broadcast(MsgPlayerFrozen, PlayerFrozenPayload{
			PlayerID: victimID,
			Position: bodyPos,
		})
		r.sendPartnerCooldowns(partnerCDs) // a human partner's button waits too
		checkWinConditions(r)
	}(room, nearestID, bodyPos)
}

// aiCrewWatching counts alive crewmates (other than the excluded IDs) within
// vision range of pos — i.e. people who would see something happen there.
func aiCrewWatching(gs *GameState, pos Vec2, exclude ...string) int {
	return aiCrewWithin(gs, pos, AIVisionRadius, exclude...)
}

// aiCrewWithin counts alive crewmates (other than the excluded IDs) within r of pos.
func aiCrewWithin(gs *GameState, pos Vec2, r float64, exclude ...string) int {
	n := 0
outer:
	for id, role := range gs.Roles {
		if role != RoleCrewmate || gs.Frozen[id] {
			continue
		}
		for _, ex := range exclude {
			if id == ex {
				continue outer
			}
		}
		if withinRange(pos, gs.Positions[id], r) {
			n++
		}
	}
	return n
}

func findStation(gs *GameState, stationID string) *TaskStation {
	for i := range gs.Tasks.Stations {
		if gs.Tasks.Stations[i].ID == stationID {
			return &gs.Tasks.Stations[i]
		}
	}
	return nil
}

func distSq(a, b Vec2) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return dx*dx + dy*dy
}

func withinRange(a, b Vec2, r float64) bool {
	return distSq(a, b) <= r*r
}
