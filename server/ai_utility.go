package main

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"time"
)

// Utility scoring. Each option gets a score in roughly 0..1; the bot picks
// the highest after noise and a commitment bonus. Rough priority ladder:
//
//	1.00  report a body I know about (crewmate)
//	0.95  get away from a body I just made (tagger)
//	0.90  meltdown fix · 0.75 lights fix · 0.60 comms fix (crewmate)
//	0.85  hunt an isolated crewmate when tag is ready (tagger)
//	0.55  nearest unfinished task (crewmate, ghost)
//	0.40  fake a task (tagger)
//	0.10  wander
//
// Distance scales most scores by 0.5..1 so closer options win ties.

const (
	aiCommitBonus     = 0.15 // keep doing what we're doing unless clearly worse
	aiTaskCommitBonus = 0.40 // don't abandon a task halfway for small reasons
	aiHelperPenalty   = 0.30 // per other bot already heading to the same fix station
	aiLeaveSceneFor   = 6 * time.Second
	aiSabotageRamp    = 90 * time.Second // urge to sabotage builds over this long
)

type aiOption struct {
	Goal     AIGoal
	Target   string
	Pos      Vec2
	Score    float64
	Sabotage SabotageType // only for GoalSabotage
}

func aiOptionKey(goal AIGoal, target string) string {
	return fmt.Sprintf("%d:%s", goal, target)
}

// proximity maps distance to a 0.5..1 multiplier (1 = right here).
func proximity(from, to Vec2) float64 {
	d := math.Sqrt(distSq(from, to))
	return 1 - 0.5*math.Min(d/1500, 1)
}

// isCurrent reports whether an option is the action the bot is already on.
func (ai *AIBrain) isCurrent(o aiOption) bool {
	if o.Target != ai.GoalTarget {
		return false
	}
	if o.Goal == ai.Goal {
		return true
	}
	// Walking to a task and standing at it are the same commitment
	return o.Goal == GoalGoToTask && (ai.Goal == GoalDoingTask || ai.Goal == GoalFakeTask)
}

// aiDecide scores every available option and commits to the best one.
func aiDecide(gs *GameState, room *Room, ai *AIBrain, pos Vec2, now time.Time) {
	var opts []aiOption
	switch {
	case gs.Frozen[ai.PlayerID]:
		opts = aiGhostOptions(gs, ai, pos)
	case ai.Role == RoleCrewmate:
		opts = aiCrewmateOptions(gs, ai, pos)
	default:
		opts = aiTaggerOptions(gs, ai, pos, now)
	}

	noise := aiProfiles[ai.Difficulty].ScoreNoise
	best := -1
	bestScore := math.Inf(-1)
	for i, o := range opts {
		if until, ok := ai.Avoid[aiOptionKey(o.Goal, o.Target)]; ok && now.Before(until) {
			continue
		}
		s := o.Score + (rand.Float64()-0.5)*noise
		if ai.isCurrent(o) {
			if ai.Goal == GoalDoingTask || ai.Goal == GoalFakeTask {
				s += aiTaskCommitBonus
			} else {
				s += aiCommitBonus
			}
		}
		if s > bestScore {
			best, bestScore = i, s
		}
	}
	if best < 0 {
		return
	}
	aiApply(gs, room, ai, opts[best], pos, now)
}

func aiApply(gs *GameState, room *Room, ai *AIBrain, o aiOption, pos Vec2, now time.Time) {
	if o.Goal == GoalSabotage {
		aiTriggerSabotage(gs, room, ai, o.Sabotage, now)
		return
	}
	if ai.isCurrent(o) {
		// Chasing a moving player: refresh the route once they've drifted
		if o.Goal == GoalHunt && !withinRange(o.Pos, ai.GoalPos, 40) {
			aiSetPath(gs, ai, pos, o.Pos)
		}
		return
	}
	ai.Goal = o.Goal
	ai.GoalTarget = o.Target
	ai.StuckTicks = 0
	aiSetPath(gs, ai, pos, o.Pos)
}

// ===== Option builders =====

func aiCrewmateOptions(gs *GameState, ai *AIBrain, pos Vec2) []aiOption {
	var opts []aiOption

	// Report bodies I've seen
	for bodyID := range ai.KnownBodies {
		bodyPos, ok := gs.BodyPos[bodyID]
		if !ok || gs.BodyReported[bodyID] {
			continue
		}
		opts = append(opts, aiOption{Goal: GoalReportBody, Target: bodyID, Pos: bodyPos, Score: 1.0})
	}

	// Fix sabotage — spread out so bots don't all pile onto one station
	if gs.Sabotage.Active != "" {
		base := map[SabotageType]float64{
			SabotageMeltdown:  0.90,
			SabotageLightsOut: 0.75,
			SabotageCommsDown: 0.60,
		}[gs.Sabotage.Active]
		for _, st := range SabotageFixStations[gs.Sabotage.Active] {
			if gs.Sabotage.Active == SabotageMeltdown && gs.Sabotage.MeltdownFixed[st.ID] {
				continue
			}
			helpers := 0
			for id, other := range gs.AIBrains {
				if id != ai.PlayerID && !gs.Frozen[id] && other.Goal == GoalFixSabotage && other.GoalTarget == st.ID {
					helpers++
				}
			}
			score := base*proximity(pos, st.Position) - aiHelperPenalty*float64(helpers)
			opts = append(opts, aiOption{Goal: GoalFixSabotage, Target: st.ID, Pos: st.Position, Score: score})
		}
	}

	opts = append(opts, aiTaskOptions(gs, ai, pos, 0.55)...)
	opts = append(opts, aiWanderOption(ai, pos, 0.10))
	return opts
}

// Ghost crewmates fly straight to their remaining tasks.
func aiGhostOptions(gs *GameState, ai *AIBrain, pos Vec2) []aiOption {
	opts := aiTaskOptions(gs, ai, pos, 0.55)
	return append(opts, aiWanderOption(ai, pos, 0.05))
}

func aiTaggerOptions(gs *GameState, ai *AIBrain, pos Vec2, now time.Time) []aiOption {
	var opts []aiOption

	// Just froze someone: get away from the scene before anyone shows up
	if !ai.LastTagAt.IsZero() && now.Sub(ai.LastTagAt) < aiLeaveSceneFor {
		opts = append(opts, aiLeaveSceneOption(ai, pos, 0.95))
	}

	// Hunt: go after crewmates, preferring ones who are alone
	cooldownLeft := gs.TagCooldown.Sub(now)
	for id, role := range gs.Roles {
		if role != RoleCrewmate || gs.Frozen[id] {
			continue
		}
		targetPos := gs.Positions[id]
		var score float64
		if cooldownLeft <= 3*time.Second {
			isolated := 0.0
			if ai.Careless || aiCrewWatching(gs, targetPos, id) == 0 {
				isolated = 1.0
			}
			score = (0.45 + 0.40*isolated) * proximity(pos, targetPos)
		} else {
			score = 0.20 * proximity(pos, targetPos) // loosely shadow people
		}
		opts = append(opts, aiOption{Goal: GoalHunt, Target: id, Pos: targetPos, Score: score})
	}

	// Sabotage: the urge builds up over time after the last one
	if gs.Sabotage.Active == "" {
		urge := math.Min(now.Sub(ai.LastSabotageAt).Seconds()/aiSabotageRamp.Seconds(), 1)
		if !now.Before(gs.SabotageCooldown) {
			minor := []SabotageType{SabotageLightsOut, SabotageCommsDown}[rand.Intn(2)]
			opts = append(opts, aiOption{Goal: GoalSabotage, Target: string(minor), Sabotage: minor, Score: 0.55 * urge})
		}
		if !gs.Sabotage.UsedMeltdown {
			// Save meltdown for when the crew is close to winning
			score := 0.75 * urge * gs.Tasks.Progress()
			opts = append(opts, aiOption{Goal: GoalSabotage, Target: string(SabotageMeltdown), Sabotage: SabotageMeltdown, Score: score})
		}
	}

	opts = append(opts, aiTaskOptions(gs, ai, pos, 0.40)...) // fake tasks
	opts = append(opts, aiWanderOption(ai, pos, 0.15))
	return opts
}

// aiTaskOptions lists the bot's unfinished tasks. Taggers get fake tasks
// (all of them stay "unfinished" forever, which is fine for faking).
func aiTaskOptions(gs *GameState, ai *AIBrain, pos Vec2, base float64) []aiOption {
	var opts []aiOption
	for _, a := range gs.Tasks.Assignments[ai.PlayerID] {
		if a.Completed {
			continue
		}
		station := findStation(gs, a.StationID)
		if station == nil {
			continue
		}
		opts = append(opts, aiOption{
			Goal:   GoalGoToTask,
			Target: a.StationID,
			Pos:    station.Position,
			Score:  base * proximity(pos, station.Position),
		})
	}
	return opts
}

// Room waypoints a bot may wander to (centers + in-room secondaries).
var aiWanderSpots = []int{0, 1, 2, 3, 4, 5, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22}

func aiWanderOption(ai *AIBrain, pos Vec2, score float64) aiOption {
	// Keep the same destination while already wandering so the commitment
	// bonus applies; otherwise pick a fresh random spot.
	if ai.Goal == GoalWander && ai.GoalTarget != "" {
		return aiOption{Goal: GoalWander, Target: ai.GoalTarget, Pos: ai.GoalPos, Score: score}
	}
	idx := aiWanderSpots[rand.Intn(len(aiWanderSpots))]
	return aiOption{Goal: GoalWander, Target: strconv.Itoa(idx), Pos: DefaultWaypoints[idx], Score: score}
}

// aiLeaveSceneOption heads for a room center far from where the bot is.
func aiLeaveSceneOption(ai *AIBrain, pos Vec2, score float64) aiOption {
	if ai.Goal == GoalLeaveScene && ai.GoalTarget != "" {
		return aiOption{Goal: GoalLeaveScene, Target: ai.GoalTarget, Pos: ai.GoalPos, Score: score}
	}
	var far []int
	for i := 0; i <= 5; i++ {
		if !withinRange(pos, DefaultWaypoints[i], 600) {
			far = append(far, i)
		}
	}
	idx := rand.Intn(6)
	if len(far) > 0 {
		idx = far[rand.Intn(len(far))]
	}
	return aiOption{Goal: GoalLeaveScene, Target: strconv.Itoa(idx), Pos: DefaultWaypoints[idx], Score: score}
}
