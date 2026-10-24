package main

import (
	"testing"
	"time"
)

func TestRollPersonalitiesAvoidsDuplicates(t *testing.T) {
	roles := map[string]Role{}
	var ais []string
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		roles[id] = RoleCrewmate
		ais = append(ais, id)
	}
	roles["h"] = RoleTagger // human, no brain
	_, gs := newTestGame(roles, ais)

	seen := map[Personality]bool{}
	for _, ai := range gs.AIBrains {
		if _, ok := personalities[ai.Personality]; !ok {
			t.Fatalf("bot %s has unknown personality %q", ai.PlayerID, ai.Personality)
		}
		if seen[ai.Personality] {
			t.Errorf("personality %s rolled twice with only 5 bots", ai.Personality)
		}
		seen[ai.Personality] = true
	}
}

// Every personality must still get its tasks done.
func TestEachPersonalityFinishesTasks(t *testing.T) {
	if testing.Short() {
		t.Skip("long simulation")
	}
	for _, p := range AllPersonalities {
		t.Run(string(p), func(t *testing.T) {
			roles := map[string]Role{}
			var ais []string
			for _, id := range []string{"a", "b", "c", "d", "e"} {
				roles[id] = RoleCrewmate
				ais = append(ais, id)
			}
			_, gs := newTestGame(roles, ais)
			for _, ai := range gs.AIBrains {
				ai.Personality = p
			}
			took, ok := simulate(gs, time.Now(), 6*time.Minute, gs.Tasks.AllTasksDone)
			if !ok {
				t.Fatalf("%s bots only finished %d/%d tasks in 6 simulated minutes", p, gs.Tasks.DoneTasks, gs.Tasks.TotalTasks)
			}
			t.Logf("5 %s bots finished %d tasks in %s", p, gs.Tasks.TotalTasks, took.Round(time.Second))
		})
	}
}

func TestBuddyPrefersTasksNearTheHuman(t *testing.T) {
	roles := map[string]Role{"buddy": RoleCrewmate, "h": RoleCrewmate, "bot": RoleCrewmate, "t": RoleTagger}
	_, gs := newTestGame(roles, []string{"buddy", "bot", "t"})
	ai := gs.AIBrains["buddy"]
	ai.Personality = PersonalityBuddy
	station := Vec2{1000, 600}

	gs.Positions["h"] = Vec2{1050, 600}
	gs.Positions["bot"] = Vec2{2000, 1200}
	if got := aiCompanyBonus(gs, ai, []string{"h"}, station); got != 0.30 {
		t.Errorf("human next to the station: bonus %.2f, want 0.30", got)
	}
	gs.Positions["h"] = Vec2{2000, 1200}
	gs.Positions["bot"] = Vec2{1050, 600}
	if got := aiCompanyBonus(gs, ai, []string{"bot"}, station); got != 0.15 {
		t.Errorf("bot next to the station: bonus %.2f, want 0.15", got)
	}
	if got := aiCompanyBonus(gs, ai, nil, station); got != 0 {
		t.Errorf("nobody around: bonus %.2f, want 0", got)
	}
	ai.Personality = PersonalityHero
	if got := aiCompanyBonus(gs, ai, []string{"bot"}, station); got != 0 {
		t.Errorf("non-buddy got a company bonus %.2f", got)
	}
}

func TestNervousWalksAwayWhenAloneWithOne(t *testing.T) {
	roles := map[string]Role{"n": RoleCrewmate, "x": RoleCrewmate, "y": RoleCrewmate, "t": RoleTagger}
	_, gs := newTestGame(roles, []string{"n"})
	ai := gs.AIBrains["n"]
	ai.Personality = PersonalityNervous
	now := time.Now()
	pos := Vec2{1000, 600}
	gs.Positions["x"] = Vec2{1100, 600}

	if _, ok := aiFleeOption(gs, ai, pos, []string{"x"}, now); ok {
		t.Error("nervous bot fled from someone it has no reason to suspect")
	}
	ai.Suspicion["x"] = 0.15
	if _, ok := aiFleeOption(gs, ai, pos, []string{"x"}, now); !ok {
		t.Error("nervous bot alone with one player should want to leave")
	}
	if _, ok := aiFleeOption(gs, ai, pos, []string{"x", "y"}, now); ok {
		t.Error("nervous bot in a group should stay")
	}

	// Once it flees it calms down for a while
	ai.LastFledAt = now
	if _, ok := aiFleeOption(gs, ai, pos, []string{"x"}, now.Add(5*time.Second)); ok {
		t.Error("nervous bot fled again during its cooldown")
	}

	ai.Personality = PersonalityHero
	if _, ok := aiFleeOption(gs, ai, pos, []string{"x"}, now.Add(time.Minute)); ok {
		t.Error("hero should not flee")
	}
}

func TestPersonalityVoting(t *testing.T) {
	roles := map[string]Role{"bot": RoleCrewmate, "s": RoleCrewmate, "c": RoleCrewmate, "t": RoleTagger}
	_, gs := newTestGame(roles, []string{"bot"})
	ai := gs.AIBrains["bot"]
	now := time.Now()
	meetingWith(gs, now)
	gs.Meeting.SetPhase("voting")
	ai.Suspicion["s"] = 0.47 // between Hero's and Normal's "sure"

	ai.Personality = PersonalityHero
	for i := 0; i < 20; i++ {
		if got := pickAIVoteTarget(gs, ai); got != "s" {
			t.Fatalf("hero voted %q at suspicion 0.47, want s", got)
		}
	}
	ai.Personality = PersonalityNervous
	skips := 0
	for i := 0; i < 200; i++ {
		if pickAIVoteTarget(gs, ai) == "" {
			skips++
		}
	}
	if skips < 150 {
		t.Errorf("nervous bot only skipped %d/200 times at suspicion 0.47", skips)
	}
}

func TestChattyAccusesOnLessEvidence(t *testing.T) {
	roles := map[string]Role{"bot": RoleCrewmate, "s": RoleCrewmate, "c": RoleCrewmate, "t": RoleTagger}
	_, gs := newTestGame(roles, []string{"bot"})
	ai := gs.AIBrains["bot"]
	meetingWith(gs, time.Now())
	ai.Suspicion["s"] = 0.42
	ai.MeetingChats = 2 // past the opening location claim

	ai.Personality = PersonalityChatty
	if got := pickAIChatMessage(gs, ai); got != susMessage(gs, "s") {
		t.Errorf("chatty said %q at suspicion 0.42, want %q", got, susMessage(gs, "s"))
	}
	ai.Personality = PersonalityHero
	if got := pickAIChatMessage(gs, ai); got == susMessage(gs, "s") {
		t.Errorf("hero accused at suspicion 0.42 (only chatty should)")
	}
}

func TestSlowpokeStillVotesInTime(t *testing.T) {
	roles := map[string]Role{"bot": RoleCrewmate, "c": RoleCrewmate, "t": RoleTagger}
	settings := DefaultSettings()
	settings.VotingTime = 10
	_, gs := newTestGameWith(roles, []string{"bot"}, settings)
	ai := gs.AIBrains["bot"]
	ai.Personality = PersonalitySlowpoke
	for i := 0; i < 50; i++ {
		meetingWith(gs, time.Now())
		if ai.VoteDelay > 8*time.Second {
			t.Fatalf("slowpoke vote delay %s exceeds a 10s voting phase", ai.VoteDelay)
		}
	}
}
