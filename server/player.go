package main

type Role string

const (
	RoleCrewmate Role = "crewmate"
	RoleTagger   Role = "tagger"
)

type PlayerColor string

const (
	ColorRed    PlayerColor = "red"
	ColorBlue   PlayerColor = "blue"
	ColorGreen  PlayerColor = "green"
	ColorYellow PlayerColor = "yellow"
	ColorPurple PlayerColor = "purple"
	ColorOrange PlayerColor = "orange"
)

var AllColors = []PlayerColor{
	ColorRed, ColorBlue, ColorGreen, ColorYellow, ColorPurple, ColorOrange,
}

type Player struct {
	ID    string      `json:"id"`
	Name  string      `json:"name"`
	Color PlayerColor `json:"color"`
	Host  bool        `json:"isHost"`
	AI    bool        `json:"isAI"`
	Alive bool        `json:"isAlive"`
	Role  Role        `json:"-"` // never sent in room state
}
