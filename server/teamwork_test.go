package main

import (
	"testing"
	"time"
)

// Two taggers ("t1", "t2") and four crewmates, all bots unless noted.
func twoTaggerRoles() map[string]Role {
	return map[string]Role{
		"t1": RoleTagger, "t2": RoleTagger,
		"c1": RoleCrewmate, "c2": RoleCrewmate, "c3": RoleCrewmate, "c4": RoleCrewmate,
	}
}

func TestTaggerCount(t *testing.T) {
	s := DefaultSettings()
	if got := s.taggerCount(6); got != 1 {
		t.Errorf("default settings gave %d taggers, want 1", got)
	}
	s.Taggers = 2
	if got := s.taggerCount(MinPlayersForTwoTaggers - 1); got != 1 {
		t.Errorf("%d players gave %d taggers, want 1 (room not full enough)", MinPlayersForTwoTaggers-1, got)
	}
	if got := s.taggerCount(MinPlayersForTwoTaggers); got != 2 {
		t.Errorf("%d players gave %d taggers, want 2", MinPlayersForTwoTaggers, got)
	}
}

func TestStartGameAssignsTwoTaggers(t *testing.T) {
	room := &Room{Code: "TWO", Phase: PhaseLobby, Players: map[string]*Player{}, Clients: map[string]*Client{}}
	room.Settings = DefaultSettings()
	room.Settings.Taggers = 2
	for i, id := range []string{"a", "b", "c", "d", "e", "f"} {
		room.Players[id] = &Player{ID: id, Name: id, Color: AllColors[i], AI: true, Alive: true}
		room.Order = append(room.Order, id)
	}
	room.startGame()
	defer room.Game.Stop()

	room.Game.mu.RLock()
	defer room.Game.mu.RUnlock()
	var taggers []string
	for id, role := range room.Game.Roles {
		if role == RoleTagger {
			taggers = append(taggers, id)
		}
	}
	if len(taggers) != 2 {
		t.Fatalf("got %d taggers, want 2", len(taggers))
	}
	for _, id := range taggers {
		partners := room.Game.AIBrains[id].Partners
		if len(partners) != 1 || partners[0] == id || room.Game.Roles[partners[0]] != RoleTagger {
			t.Errorf("tagger %s has partners %v, want the other tagger", id, partners)
		}
		if !time.Now().Before(room.Game.TagCooldowns[id]) {
			t.Errorf("tagger %s can tag the moment the game starts", id)
		}
	}
}

func TestTaggersCannotTagEachOther(t *testing.T) {
	_, gs := newTestGame(twoTaggerRoles(), nil)
	gs.Positions["t1"] = Vec2{300, 300}
	gs.Positions["t2"] = Vec2{330, 300}
	if _, ok := gs.TryTag("t1", "t2", time.Now()); ok {
		t.Fatal("a tagger froze their partner")
	}
}

func TestTagCooldownsAreIndependent(t *testing.T) {
	_, gs := newTestGame(twoTaggerRoles(), nil)
	gs.Positions["t1"] = Vec2{300, 300}
	gs.Positions["c1"] = Vec2{330, 300}
	gs.Positions["t2"] = Vec2{1900, 1000}
	gs.Positions["c2"] = Vec2{1930, 1000}

	now := time.Now()
	if _, ok := gs.TryTag("t1", "c1", now); !ok {
		t.Fatal("setup: first tag failed")
	}
	if _, ok := gs.TryTag("t2", "c2", now.Add(gs.teamTagGap())); !ok {
		t.Fatal("one tagger's cooldown blocked the other tagger past the team gap")
	}
	gs.Positions["c3"] = Vec2{330, 300}
	if _, ok := gs.TryTag("t1", "c3", now.Add(gs.teamTagGap()+time.Second)); ok {
		t.Fatal("tagger ignored their own cooldown")
	}
}

func TestPartnerWaitsAfterTeamTag(t *testing.T) {
	_, gs := newTestGame(twoTaggerRoles(), nil)
	gs.Positions["t1"] = Vec2{300, 300}
	gs.Positions["c1"] = Vec2{330, 300}
	gs.Positions["t2"] = Vec2{1900, 1000}
	gs.Positions["c2"] = Vec2{1930, 1000}

	now := time.Now()
	if _, ok := gs.TryTag("t1", "c1", now); !ok {
		t.Fatal("setup: first tag failed")
	}
	if _, ok := gs.TryTag("t2", "c2", now.Add(gs.teamTagGap()/2)); ok {
		t.Fatal("partner tagged right after a team tag")
	}
	if _, ok := gs.TryTag("t2", "c2", now.Add(gs.teamTagGap())); !ok {
		t.Fatal("partner still blocked after the team gap")
	}
}

func TestTaggerBotWaitsForPartnerCooldownBeforeAssist(t *testing.T) {
	_, gs := newTestGame(twoTaggerRoles(), []string{"t2"})
	now := time.Now()
	bot := gs.AIBrains["t2"]
	bot.RoundStart = now.Add(-time.Minute)
	bot.PartnerTagSeen["t1"] = now.Add(-5 * time.Second) // just saw them tag
	if aiPartnerProbablyReady(gs, bot, "t1", now) {
		t.Error("bot thinks its partner is ready 5s after seeing them tag")
	}
	bot.RoundStart = now.Add(-2 * time.Second)
	delete(bot.PartnerTagSeen, "t1")
	if aiPartnerProbablyReady(gs, bot, "t1", now) {
		t.Error("bot thinks its partner is ready 2s into the round")
	}
}

func TestTwoTaggersNeedThreeTags(t *testing.T) {
	room, gs := newTestGame(twoTaggerRoles(), nil)
	gs.Frozen["c1"], gs.Frozen["c2"] = true, true // 2 taggers vs 2 crew
	if checkWinConditions(room) {
		t.Fatal("taggers won at 2 vs 2; the crew should still be able to finish tasks")
	}
	gs.Frozen["c3"] = true
	if !checkWinConditions(room) {
		t.Fatal("taggers should win with one crewmate left")
	}
}

func TestCrewWinsOnlyWhenAllTaggersOut(t *testing.T) {
	room, gs := newTestGame(twoTaggerRoles(), nil)
	gs.Frozen["t1"] = true
	if checkWinConditions(room) {
		t.Fatal("game ended with one tagger still in")
	}
	gs.Frozen["t2"] = true
	if !checkWinConditions(room) {
		t.Fatal("crew should win once both taggers are out")
	}
}

// meetingWith starts a meeting so every bot has reset its meeting state.
func meetingWith(gs *GameState, now time.Time) {
	alive := make([]string, 0, len(gs.Roles))
	for id := range gs.Roles {
		alive = append(alive, id)
	}
	gs.Meeting = NewMeeting("c1", "emergency", "", alive)
	for _, ai := range gs.AIBrains {
		aiOnMeetingStart(gs, ai, now)
	}
}

func TestTaggerBotDefendsHumanPartner(t *testing.T) {
	// t1 is the human tagger; t2 is the bot partner
	_, gs := newTestGame(twoTaggerRoles(), []string{"t2"})
	now := time.Now()
	meetingWith(gs, now)

	// Two crewmates call out the human; the bot must not pile on
	aiNotifyChat(gs, "c1", susMessage(gs, "t1"), now)
	aiNotifyChat(gs, "c2", susMessage(gs, "t1"), now)

	bot := gs.AIBrains["t2"]
	bot.MeetingChats = 1
	msg := pickAIChatMessage(gs, bot)
	if msg != susMessage(gs, "c1") && msg != susMessage(gs, "c2") {
		t.Errorf("bot said %q, want it to turn suspicion on an accuser", msg)
	}
	for i := 0; i < 20; i++ {
		if v := pickAIVoteTarget(gs, bot); v == "t1" {
			t.Fatal("bot voted out its partner")
		} else if v != "c1" && v != "c2" {
			t.Fatalf("bot voted %q, want an accuser", v)
		}
	}
}

func TestTaggerBotBacksUpPartnerAccusation(t *testing.T) {
	_, gs := newTestGame(twoTaggerRoles(), []string{"t2"})
	now := time.Now()
	meetingWith(gs, now)

	aiNotifyChat(gs, "t1", susMessage(gs, "c3"), now) // the human tagger blames c3
	aiNotifyChat(gs, "c1", susMessage(gs, "c4"), now) // someone else blames c4

	bot := gs.AIBrains["t2"]
	if msg := pickAIChatMessage(gs, bot); msg != susMessage(gs, "c3") {
		t.Errorf("bot said %q, want it to echo its partner (%s)", msg, susMessage(gs, "c3"))
	}
	if v := pickAIVoteTarget(gs, bot); v != "c3" {
		t.Errorf("bot voted %q, want its partner's pick c3", v)
	}
}

func TestTaggerBotUsesLightsToHelpPartner(t *testing.T) {
	_, gs := newTestGame(twoTaggerRoles(), []string{"t2"})
	now := time.Now()
	bot := gs.AIBrains["t2"]
	gs.TagCooldowns["t2"] = now.Add(time.Minute) // not hunting itself
	bot.RoundStart = now.Add(-time.Minute)       // partner's tag should be ready

	// Human partner t1 is next to c1, but c2 is watching from 200 away —
	// close enough to see a tag now, too far once the lights go out.
	gs.Positions["t1"] = Vec2{1000, 1000}
	gs.Positions["c1"] = Vec2{1040, 1000}
	gs.Positions["c2"] = Vec2{1000, 1200}
	gs.Positions["t2"] = Vec2{1150, 1050} // can see its partner
	gs.Positions["c3"] = Vec2{300, 250}
	gs.Positions["c4"] = Vec2{1900, 250}

	aiDecide(gs, gs.Room, bot, gs.Positions["t2"], now)
	if gs.Sabotage.Active != SabotageLightsOut {
		t.Fatalf("bot didn't cut the lights for its partner (sabotage=%q)", gs.Sabotage.Active)
	}

	// Nobody watching: no need to help, so the lights stay on
	_, gs = newTestGame(twoTaggerRoles(), []string{"t2"})
	bot = gs.AIBrains["t2"]
	gs.TagCooldowns["t2"] = now.Add(time.Minute)
	bot.RoundStart = now.Add(-time.Minute)
	gs.Positions["t1"] = Vec2{1000, 1000}
	gs.Positions["c1"] = Vec2{1040, 1000}
	gs.Positions["c2"] = Vec2{300, 250}
	gs.Positions["t2"] = Vec2{1150, 1050}
	gs.Positions["c3"] = Vec2{300, 300}
	gs.Positions["c4"] = Vec2{1900, 250}
	aiDecide(gs, gs.Room, bot, gs.Positions["t2"], now)
	if gs.Sabotage.Active != "" {
		t.Errorf("bot sabotaged (%q) with no witnesses to blind", gs.Sabotage.Active)
	}
}

func TestDifficultySetting(t *testing.T) {
	for setting, want := range map[string]AIDifficulty{"easy": AIEasy, "normal": AINormal, "hard": AIHard, "": AINormal, "bogus": AINormal} {
		if got := ParseDifficulty(setting); got != want {
			t.Errorf("ParseDifficulty(%q) = %d, want %d", setting, got, want)
		}
	}

	settings := DefaultSettings()
	settings.Difficulty = "hard"
	_, gs := newTestGameWith(map[string]Role{"t": RoleTagger, "a": RoleCrewmate}, []string{"t", "a"}, settings)
	for id, ai := range gs.AIBrains {
		if ai.Difficulty != AIHard {
			t.Errorf("bot %s got difficulty %d, want Hard", id, ai.Difficulty)
		}
	}
	if gs.AIBrains["t"].Careless {
		t.Error("a Hard tagger should never be careless")
	}
}
