package main

import (
	"sort"
	"testing"
	"time"
)

// newTestGame builds a room + game like Room.startGame, but with explicit
// roles and a chosen subset of AI players. No clients are attached, so
// broadcasts are no-ops.
func newTestGame(roles map[string]Role, aiIDs []string) (*Room, *GameState) {
	ids := make([]string, 0, len(roles))
	for id := range roles {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	room := &Room{
		Code:     "TEST",
		Phase:    PhasePlaying,
		Players:  make(map[string]*Player),
		Order:    ids,
		Clients:  make(map[string]*Client),
		Settings: DefaultSettings(),
	}
	colors := make(map[string]string)
	for i, id := range ids {
		room.Players[id] = &Player{ID: id, Name: id, Color: AllColors[i], Role: roles[id], Alive: true}
		colors[id] = string(AllColors[i])
	}
	gs := NewGameState(BuildMap(), ids, roles, aiIDs, room.Settings, colors)
	gs.Room = room
	room.Game = gs
	return room, gs
}

// simulate ticks the game at 20Hz with a fake clock until done() is true or
// the duration runs out. Returns the simulated time it took.
func simulate(gs *GameState, start time.Time, limit time.Duration, done func() bool) (time.Duration, bool) {
	dt := TickDuration
	for elapsed := time.Duration(0); elapsed < limit; elapsed += dt {
		gs.TickAt(start.Add(elapsed), dt.Seconds())
		gs.mu.RLock()
		ok := done()
		gs.mu.RUnlock()
		if ok {
			return elapsed, true
		}
	}
	return limit, false
}

func TestAICrewFinishesAllTasks(t *testing.T) {
	roles := map[string]Role{}
	var ais []string
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		roles[id] = RoleCrewmate
		ais = append(ais, id)
	}
	_, gs := newTestGame(roles, ais)

	took, ok := simulate(gs, time.Now(), 6*time.Minute, gs.Tasks.AllTasksDone)
	if !ok {
		t.Fatalf("bots only finished %d/%d tasks in 6 simulated minutes", gs.Tasks.DoneTasks, gs.Tasks.TotalTasks)
	}
	t.Logf("5 bots finished %d tasks in %s", gs.Tasks.TotalTasks, took.Round(time.Second))
}

func TestAIGhostsKeepDoingTasks(t *testing.T) {
	roles := map[string]Role{"a": RoleCrewmate, "b": RoleCrewmate, "c": RoleCrewmate}
	_, gs := newTestGame(roles, []string{"a", "b", "c"})
	gs.Frozen["b"] = true
	gs.Frozen["c"] = true

	if _, ok := simulate(gs, time.Now(), 6*time.Minute, gs.Tasks.AllTasksDone); !ok {
		t.Fatalf("ghost bots stalled at %d/%d tasks", gs.Tasks.DoneTasks, gs.Tasks.TotalTasks)
	}
}

func TestAIOnlyWitnessesWhatItCanSee(t *testing.T) {
	// Human tagger "h" freezes "v". Bot "near" is watching; bot "far" is
	// across the ship and must not learn anything.
	roles := map[string]Role{"h": RoleTagger, "v": RoleCrewmate, "near": RoleCrewmate, "far": RoleCrewmate}
	_, gs := newTestGame(roles, []string{"v", "near", "far"})
	gs.Positions["h"] = Vec2{300, 300}
	gs.Positions["v"] = Vec2{340, 300}
	gs.Positions["near"] = Vec2{450, 300}
	gs.Positions["far"] = Vec2{1900, 1150}

	now := time.Now()
	gs.TagCooldown = now
	if _, ok := gs.TryTag("h", "v", now); !ok {
		t.Fatal("setup: tag failed")
	}

	near, far := gs.AIBrains["near"], gs.AIBrains["far"]
	if !near.Witnessed["h"] || near.Suspicion["h"] < aiSusVoteSure {
		t.Errorf("nearby bot should have witnessed the tag, suspicion=%.2f", near.Suspicion["h"])
	}
	if far.Witnessed["h"] || far.Suspicion["h"] != 0 {
		t.Errorf("far bot learned about the tag from across the map, suspicion=%.2f", far.Suspicion["h"])
	}

	// Perceiving from far away doesn't reveal the body or who's near it
	aiPerceive(gs, far, gs.Positions["far"], now)
	if far.KnownBodies["v"] || far.Suspicion["h"] != 0 {
		t.Errorf("far bot perceived a body/suspect outside its vision")
	}

	// With no evidence, the far bot skips instead of guessing
	gs.Meeting = NewMeeting("near", "body", "v", []string{"h", "near", "far"})
	for i := 0; i < 50; i++ {
		if got := pickAIVoteTarget(gs, far); got != "" {
			t.Fatalf("far bot voted for %q with no evidence", got)
		}
	}
	if got := pickAIVoteTarget(gs, near); got != "h" {
		t.Errorf("witness voted for %q, want h", got)
	}
}

func TestAINearBodyIsSuspiciousNotCertain(t *testing.T) {
	roles := map[string]Role{"t": RoleTagger, "v": RoleCrewmate, "x": RoleCrewmate, "bot": RoleCrewmate}
	_, gs := newTestGame(roles, []string{"bot"})
	gs.Frozen["v"] = true
	gs.BodyPos["v"] = Vec2{1100, 960}
	gs.Positions["x"] = Vec2{1150, 960} // innocent standing by the body
	gs.Positions["t"] = Vec2{300, 300}  // tagger long gone
	gs.Positions["bot"] = Vec2{1100, 1100}

	bot := gs.AIBrains["bot"]
	aiPerceive(gs, bot, gs.Positions["bot"], time.Now())
	if !bot.KnownBodies["v"] {
		t.Fatal("bot should see the body")
	}
	if s := bot.Suspicion["x"]; s != aiSusNearBody {
		t.Errorf("bystander suspicion = %.2f, want %.2f", s, aiSusNearBody)
	}
	if bot.Suspicion["t"] != 0 {
		t.Errorf("bot suspects the tagger without seeing them")
	}
	// Seeing the same person at the same body again doesn't stack
	aiPerceive(gs, bot, gs.Positions["bot"], time.Now())
	if s := bot.Suspicion["x"]; s != aiSusNearBody {
		t.Errorf("suspicion stacked on repeat sighting: %.2f", s)
	}
}

func TestAITaggerWaitsForNoWitnesses(t *testing.T) {
	roles := map[string]Role{"tag": RoleTagger, "v": RoleCrewmate, "w": RoleCrewmate}
	room, gs := newTestGame(roles, []string{"tag"})
	now := time.Now()
	gs.TagCooldown = now.Add(-time.Second)
	gs.Positions["tag"] = Vec2{1100, 960}
	gs.Positions["v"] = Vec2{1140, 960}
	gs.Positions["w"] = Vec2{1100, 1100} // within vision of the tagger

	ai := gs.AIBrains["tag"]
	ai.Careless = false
	aiTaggerTryTag(gs, room, ai, gs.Positions["tag"], now)
	if gs.Frozen["v"] {
		t.Fatal("tagger froze someone in front of a witness")
	}

	gs.Positions["w"] = Vec2{1900, 300} // witness walks away
	aiTaggerTryTag(gs, room, ai, gs.Positions["tag"], now)
	if !gs.Frozen["v"] {
		t.Fatal("tagger should freeze an isolated crewmate")
	}
	wantCD := now.Add(time.Duration(gs.Settings.TagCooldown * float64(time.Second)))
	if !gs.TagCooldown.Equal(wantCD) {
		t.Errorf("AI tag cooldown = %s, want lobby setting %.0fs", gs.TagCooldown.Sub(now), gs.Settings.TagCooldown)
	}
}

func TestAICrewFixesMeltdown(t *testing.T) {
	roles := map[string]Role{"t": RoleTagger, "a": RoleCrewmate, "b": RoleCrewmate, "c": RoleCrewmate}
	_, gs := newTestGame(roles, []string{"a", "b", "c"}) // human tagger stands still
	now := time.Now()
	gs.mu.Lock()
	ok := gs.trySabotageLocked("t", SabotageMeltdown, now)
	gs.mu.Unlock()
	if !ok {
		t.Fatal("setup: meltdown failed")
	}

	took, fixed := simulate(gs, now, MeltdownDuration-5*time.Second, func() bool { return gs.Sabotage.Active == "" })
	if !fixed {
		t.Fatalf("bots didn't fix meltdown in time (fixed stations: %v)", gs.Sabotage.MeltdownFixed)
	}
	t.Logf("meltdown fixed after %s", took.Round(time.Second))
}

func TestAIHearsChatButIgnoresSuspects(t *testing.T) {
	roles := map[string]Role{"s": RoleCrewmate, "liar": RoleTagger, "x": RoleCrewmate, "bot": RoleCrewmate}
	_, gs := newTestGame(roles, []string{"bot"})
	bot := gs.AIBrains["bot"]
	xMsg := susMessage(gs, "x")

	now := time.Now()
	aiNotifyChat(gs, "s", xMsg, now)
	aiNotifyChat(gs, "s", xMsg, now) // repeats don't count twice
	if got := bot.Suspicion["x"]; got != aiSusChat {
		t.Errorf("suspicion after accusation = %.2f, want %.2f", got, aiSusChat)
	}

	bot.Suspicion["liar"] = 1.0 // bot saw the liar tag someone
	aiNotifyChat(gs, "liar", xMsg, now)
	if got := bot.Suspicion["x"]; got != aiSusChat {
		t.Errorf("bot listened to a suspect's accusation: %.2f", got)
	}
}

func TestAILastSeenWithVictim(t *testing.T) {
	// Bot sees the tagger walking with "v", wanders off; later v is found
	// frozen. The tagger should become the bot's top suspect.
	roles := map[string]Role{"t": RoleTagger, "v": RoleCrewmate, "x": RoleCrewmate, "bot": RoleCrewmate}
	_, gs := newTestGame(roles, []string{"bot"})
	bot := gs.AIBrains["bot"]
	start := time.Now()

	gs.Positions["bot"] = Vec2{300, 300}
	gs.Positions["t"] = Vec2{350, 320}
	gs.Positions["v"] = Vec2{380, 320}
	gs.Positions["x"] = Vec2{1900, 1150} // never near the victim
	aiPerceive(gs, bot, gs.Positions["bot"], start)

	// 10s later v is found frozen in another room
	gs.Frozen["v"] = true
	gs.BodyPos["v"] = Vec2{300, 960}
	gs.Meeting = NewMeeting("x", "body", "v", []string{"t", "x", "bot"})
	aiOnMeetingStart(gs, bot, start.Add(10*time.Second))

	if bot.BodyRoom != "engine" {
		t.Errorf("BodyRoom = %q, want engine", bot.BodyRoom)
	}
	suspect, level := bot.topSuspect(gs.Meeting.AlivePlayers)
	if suspect != "t" || level <= 0 {
		t.Errorf("top suspect = %q (%.2f), want t", suspect, level)
	}
	if bot.Suspicion["x"] != 0 {
		t.Errorf("x was never with the victim but has suspicion %.2f", bot.Suspicion["x"])
	}
}

func TestAICatchesLocationLies(t *testing.T) {
	roles := map[string]Role{"liar": RoleTagger, "honest": RoleCrewmate, "bot": RoleCrewmate}
	_, gs := newTestGame(roles, []string{"bot"})
	bot := gs.AIBrains["bot"]
	now := time.Now()

	// Bot sees both players in Reactor
	gs.Positions["bot"] = Vec2{1800, 960}
	gs.Positions["liar"] = Vec2{1850, 960}
	gs.Positions["honest"] = Vec2{1750, 1000}
	aiPerceive(gs, bot, gs.Positions["bot"], now)

	gs.Meeting = NewMeeting("honest", "emergency", "", []string{"liar", "honest", "bot"})
	gs.Meeting.StartTime = now.Add(2 * time.Second)
	// Messages arrive well into the discussion — still judged vs meeting start
	later := now.Add(40 * time.Second)
	aiNotifyChat(gs, "liar", "loc_engine", later)
	aiNotifyChat(gs, "honest", "loc_reactor", later)

	if s := bot.Suspicion["liar"]; s != aiSusLying {
		t.Errorf("liar suspicion = %.2f, want %.2f", s, aiSusLying)
	}
	if s := bot.Suspicion["honest"]; s != 0 {
		t.Errorf("honest suspicion = %.2f, want 0", s)
	}
	// A second claim from the liar doesn't stack
	aiNotifyChat(gs, "liar", "loc_medbay", later)
	if s := bot.Suspicion["liar"]; s != aiSusLying {
		t.Errorf("repeat claim stacked: %.2f", s)
	}
}

func TestAICarelessTaggerGetsWitnessed(t *testing.T) {
	roles := map[string]Role{"tag": RoleTagger, "v": RoleCrewmate, "w": RoleCrewmate}
	room, gs := newTestGame(roles, []string{"tag", "w"})
	now := time.Now()
	gs.TagCooldown = now.Add(-time.Second)
	gs.Positions["tag"] = Vec2{1100, 960}
	gs.Positions["v"] = Vec2{1140, 960}
	gs.Positions["w"] = Vec2{1100, 1100}

	ai := gs.AIBrains["tag"]
	ai.Careless = true
	aiTaggerTryTag(gs, room, ai, gs.Positions["tag"], now)
	if !gs.Frozen["v"] {
		t.Fatal("careless tagger should tag even with a witness around")
	}
	if !gs.AIBrains["w"].Witnessed["tag"] {
		t.Error("witness bot didn't see the careless tag")
	}
}
