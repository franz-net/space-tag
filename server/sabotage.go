package main

import "time"

// Sabotage types the tagger can trigger.
type SabotageType string

const (
	SabotageLightsOut SabotageType = "lights_out"
	SabotageCommsDown SabotageType = "comms_down"
	SabotageMeltdown  SabotageType = "meltdown"
)

// Durations and ranges
const (
	SabotageDuration = 30 * time.Second  // lights_out / comms_down auto-expire
	SabotageCooldown = 45 * time.Second  // cooldown between minor sabotages
	MeltdownDuration = 2 * time.Minute   // crew must fix before this expires
	SabotageFixRange = 60.0              // same as TaskInteractionRange
)

// Fix stations — where crew must go to repair each sabotage.
// Meltdown requires BOTH engine AND reactor to be fixed.
// Fix station positions are placed away from obstacles:
//   Reactor core obstacle: {1860, 1030, 80, 80} — fix station above it
//   Engine turbines: y:1040-1100 — fix station above them
var SabotageFixStations = map[SabotageType][]FixStation{
	SabotageLightsOut: {
		{ID: "reactor", RoomID: "reactor", Position: Vec2{1900, 960}},
	},
	SabotageCommsDown: {
		{ID: "navigation", RoomID: "navigation", Position: Vec2{1900, 250}},
	},
	SabotageMeltdown: {
		{ID: "engine", RoomID: "engine", Position: Vec2{300, 960}},
		{ID: "reactor", RoomID: "reactor", Position: Vec2{1900, 960}},
	},
}

// FixStation is a location where a crewmate can repair a sabotage.
type FixStation struct {
	ID       string `json:"id"`
	RoomID   string `json:"roomId"`
	Position Vec2   `json:"position"`
}

// SabotageState tracks the current active sabotage.
type SabotageState struct {
	Active       SabotageType      // "" if no sabotage active
	ExpiresAt    time.Time         // when sabotage auto-expires or meltdown triggers
	UsedMeltdown bool              // true once meltdown has been used (once per game)
	MeltdownFixed map[string]bool  // stationID -> fixed (for meltdown's two stations)
}

func NewSabotageState() *SabotageState {
	return &SabotageState{
		MeltdownFixed: make(map[string]bool),
	}
}

// SabotageStartPayload is broadcast when a sabotage begins.
type SabotageStartPayload struct {
	Type     SabotageType `json:"type"`
	Duration float64      `json:"duration"` // seconds
	Stations []FixStation `json:"stations"` // where crew must go to fix
}

// SabotageEndPayload signals sabotage cleared.
type SabotageEndPayload struct{}

// SabotagePayload is sent by tagger to activate a sabotage.
type SabotagePayload struct {
	Type SabotageType `json:"type"`
}

// SabotageFixPayload is sent by crewmate fixing a station.
type SabotageFixPayload struct {
	StationID string `json:"stationId"`
}
