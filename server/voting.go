package main

import (
	"sync"
	"time"
)

const (
	DiscussionDuration = 30 * time.Second
	VotingDuration     = 20 * time.Second
	TagCooldown        = 25 * time.Second
	TagRange           = 70.0
	ReportRange        = 90.0
	MaxChatMessages    = 8 // per player per meeting
)

// Pre-defined chat messages — kid friendly icons only
var PreDefinedMessages = map[string]bool{
	"sus_red":        true,
	"sus_blue":       true,
	"sus_green":      true,
	"sus_yellow":     true,
	"sus_purple":     true,
	"sus_orange":     true,
	"i_saw":          true,
	"with_me":        true,
	"doing_task":     true,
	"where":          true,
	"idk":            true,
	"vote_skip":      true,
	"trust_me":       true,
	"not_me":         true,
	"loc_cafeteria":  true,
	"loc_medbay":     true,
	"loc_navigation": true,
	"loc_engine":     true,
	"loc_storage":    true,
	"loc_reactor":    true,
}

type Meeting struct {
	CallerID     string
	Reason       string
	BodyID       string
	StartTime    time.Time
	Phase        string                // "discussion" or "voting"
	Votes        map[string]string     // voterID -> targetID ("" = skip)
	ChatCounts   map[string]int        // voterID -> # messages sent
	AlivePlayers []string
	mu           sync.RWMutex
}

func NewMeeting(callerID, reason, bodyID string, alivePlayers []string) *Meeting {
	return &Meeting{
		CallerID:     callerID,
		Reason:       reason,
		BodyID:       bodyID,
		StartTime:    time.Now(),
		Phase:        "discussion",
		Votes:        make(map[string]string),
		ChatCounts:   make(map[string]int),
		AlivePlayers: alivePlayers,
	}
}

// CastVote records a vote. Returns true if accepted.
func (m *Meeting) CastVote(voterID, targetID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Phase != "voting" {
		return false
	}

	// Voter must be alive
	if !m.isAlive(voterID) {
		return false
	}

	// Target must be alive (or empty for skip)
	if targetID != "" && !m.isAlive(targetID) {
		return false
	}

	m.Votes[voterID] = targetID
	return true
}

func (m *Meeting) isAlive(playerID string) bool {
	for _, id := range m.AlivePlayers {
		if id == playerID {
			return true
		}
	}
	return false
}

// CanSendChat returns true if voter hasn't exceeded message limit
func (m *Meeting) CanSendChat(voterID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ChatCounts[voterID] < MaxChatMessages
}

func (m *Meeting) RecordChat(voterID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ChatCounts[voterID]++
}

// TallyVotes returns the player with the most votes (or "" if tied/no votes)
func (m *Meeting) TallyVotes() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counts := make(map[string]int)
	for _, target := range m.Votes {
		counts[target]++ // empty string is "skip"
	}

	maxVotes := 0
	winner := ""
	tied := false

	for target, count := range counts {
		if count > maxVotes {
			maxVotes = count
			winner = target
			tied = false
		} else if count == maxVotes && target != winner {
			tied = true
		}
	}

	if tied || maxVotes == 0 || winner == "" {
		return "" // no ejection
	}
	return winner
}

func (m *Meeting) GetVotes() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]string, len(m.Votes))
	for k, v := range m.Votes {
		out[k] = v
	}
	return out
}

func (m *Meeting) SetPhase(phase string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Phase = phase
}
