package main

import (
	"math/rand"
	"time"
)

// Personalities make each bot play a little differently. Under Utility AI a
// personality is just a set of score multipliers and meeting habits layered
// on top of the difficulty profile. Each bot rolls one at game start.
//
//	🤝 Buddy     likes tasks near other players (the human most), vouches "with me"
//	🗣️ Chatty    talks a lot and accuses on thinner evidence
//	😰 Nervous   won't stay alone with someone it suspects, skips votes, "not me!" early
//	🦸 Hero      rushes to fix sabotage, votes confidently
//	🐢 Slowpoke  slow at tasks, wanders, votes late

type Personality string

const (
	PersonalityBuddy    Personality = "buddy"
	PersonalityChatty   Personality = "chatty"
	PersonalityNervous  Personality = "nervous"
	PersonalityHero     Personality = "hero"
	PersonalitySlowpoke Personality = "slowpoke"
)

var AllPersonalities = []Personality{
	PersonalityBuddy, PersonalityChatty, PersonalityNervous, PersonalityHero, PersonalitySlowpoke,
}

type personalityWeights struct {
	// Moving around the ship
	TaskWeight    float64 // multiplier on task scores
	FixWeight     float64 // multiplier on sabotage-fix scores
	WanderWeight  float64 // multiplier on the wander score
	NearPlayer    float64 // bonus for a task with another visible player nearby (x2 for a human)
	FleeOne       bool    // walk away when alone with one player it suspects
	ShadowWeight  float64 // tagger: multiplier on loosely following crewmates while on cooldown
	TaskTimeScale float64 // multiplier on how long a bot stands at a task

	// Meetings
	MaxChats     int     // messages per meeting
	ChatChance   float64 // chance per tick to speak during discussion
	AccuseAt     float64 // suspicion needed to say "X is sus"
	VoteSure     float64 // suspicion needed to always vote for someone
	VoteMaybe    float64 // suspicion needed to sometimes vote for someone
	MaybeChance  float64 // chance of voting at the "maybe" level
	NotMeEarly   bool    // says "not me!" in the first message even if nobody accused it
	VouchWithMe  bool    // says "with me" when it had company before the meeting
	VoteDelayMin time.Duration
	VoteDelayMax time.Duration
}

var basePersonality = personalityWeights{
	TaskWeight:    1,
	FixWeight:     1,
	WanderWeight:  1,
	ShadowWeight:  1,
	TaskTimeScale: 1,
	MaxChats:      3,
	ChatChance:    0.03,
	AccuseAt:      aiSusVoteSure,
	VoteSure:      aiSusVoteSure,
	VoteMaybe:     aiSusVoteMaybe,
	MaybeChance:   0.35,
	VoteDelayMin:  2 * time.Second,
	VoteDelayMax:  10 * time.Second,
}

var personalities = map[Personality]personalityWeights{
	PersonalityBuddy: with(func(w *personalityWeights) {
		w.NearPlayer = 0.15
		w.ShadowWeight = 1.4 // a Buddy tagger tags along with someone before striking
		w.VouchWithMe = true
	}),
	PersonalityChatty: with(func(w *personalityWeights) {
		w.MaxChats = 6
		w.ChatChance = 0.06
		w.AccuseAt = 0.4
	}),
	PersonalityNervous: with(func(w *personalityWeights) {
		w.NearPlayer = 0.08 // safety in numbers
		w.FleeOne = true
		w.VoteSure = 0.65
		w.MaybeChance = 0.1
		w.NotMeEarly = true
	}),
	PersonalityHero: with(func(w *personalityWeights) {
		w.FixWeight = 1.3
		w.VoteSure = 0.45
		w.MaybeChance = 0.5
		w.VoteDelayMin = 1 * time.Second
		w.VoteDelayMax = 4 * time.Second
	}),
	PersonalitySlowpoke: with(func(w *personalityWeights) {
		w.TaskTimeScale = 1.25
		w.WanderWeight = 2.0
		w.VoteDelayMin = 9 * time.Second
		w.VoteDelayMax = 16 * time.Second
	}),
}

func with(edit func(*personalityWeights)) personalityWeights {
	w := basePersonality
	edit(&w)
	return w
}

// weights returns the bot's personality weights (base if it has none).
func (ai *AIBrain) weights() personalityWeights {
	if w, ok := personalities[ai.Personality]; ok {
		return w
	}
	return basePersonality
}

// rollPersonalities gives each bot a personality, avoiding duplicates until
// every personality has been used once.
func rollPersonalities(brains map[string]*AIBrain, ids []string) {
	deck := make([]Personality, 0, len(AllPersonalities))
	for _, id := range ids {
		ai, ok := brains[id]
		if !ok {
			continue
		}
		if len(deck) == 0 {
			deck = append(deck, AllPersonalities...)
			rand.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
		}
		ai.Personality = deck[0]
		deck = deck[1:]
	}
}

const (
	aiFleeRange    = 200.0
	aiFleeCooldown = 15 * time.Second
	aiNearTask     = 250.0 // a player this close to a station counts as company
)

// aiCompanyBonus is the Buddy/Nervous bonus for a task station with another
// visible player near it. Humans count double for a Buddy so the kid gets
// a natural alibi.
func aiCompanyBonus(gs *GameState, ai *AIBrain, visible []string, station Vec2) float64 {
	w := ai.weights()
	if w.NearPlayer == 0 {
		return 0
	}
	best := 0.0
	for _, id := range visible {
		if !withinRange(gs.Positions[id], station, aiNearTask) {
			continue
		}
		b := w.NearPlayer
		if _, isBot := gs.AIBrains[id]; !isBot && ai.Personality == PersonalityBuddy {
			b *= 2
		}
		if b > best {
			best = b
		}
	}
	return best
}

// aiVisiblePlayers returns the alive players (other than the bot) in view.
func aiVisiblePlayers(gs *GameState, ai *AIBrain, pos Vec2) []string {
	var out []string
	vision := aiVisionRadius(gs, ai)
	for id, p := range gs.Positions {
		if id != ai.PlayerID && !gs.Frozen[id] && withinRange(pos, p, vision) {
			out = append(out, id)
		}
	}
	return out
}

// aiFleeOption: a Nervous bot alone with exactly one other player it has
// any suspicion of heads somewhere else. (Fleeing from everyone left it
// wandering off alone, which made it an easy target.) Never mid-task: the
// task commitment wins. aiApply starts the cooldown once the bot actually flees.
func aiFleeOption(gs *GameState, ai *AIBrain, pos Vec2, visible []string, now time.Time) (aiOption, bool) {
	if !ai.weights().FleeOne || len(visible) != 1 {
		return aiOption{}, false
	}
	if ai.Goal != GoalLeaveScene && now.Sub(ai.LastFledAt) < aiFleeCooldown {
		return aiOption{}, false
	}
	if ai.Suspicion[visible[0]] <= 0 || !withinRange(pos, gs.Positions[visible[0]], aiFleeRange) {
		return aiOption{}, false
	}
	return aiLeaveSceneOption(ai, pos, 0.6), true
}
