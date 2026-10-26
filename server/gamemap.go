package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

// Vec2 represents a 2D position
type Vec2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Rect is an axis-aligned rectangle
type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type MapRoom struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Bounds Rect   `json:"bounds"`
	Color  string `json:"color"` // hex fill color for the client
}

type Hallway struct {
	Bounds Rect `json:"bounds"`
}

// Waypoint is a spot bots can wander to. Room is "" in hallways.
type Waypoint struct {
	Pos    Vec2
	Room   string
	Center bool // the middle of its room
}

type GameMap struct {
	Rooms     []MapRoom `json:"rooms"`
	Hallways  []Hallway `json:"hallways"`
	Walls     []Rect    `json:"walls"`
	Obstacles []Rect    `json:"obstacles"` // furniture/props that block movement
	SpawnPos  Vec2      `json:"spawnPos"`
	Width     float64   `json:"width"`
	Height    float64   `json:"height"`

	// Server-only data
	TaskStations []TaskStation                `json:"-"`
	FixStations  map[SabotageType][]FixStation `json:"-"`
	Waypoints    []Waypoint                   `json:"-"`
	nav          *navGrid
}

// shipmap.json is exported from assets/maps/ship.tmx by assets/maps/export.py.
//
//go:embed shipmap.json
var shipMapJSON []byte

var (
	shipMapOnce sync.Once
	shipMap     *GameMap
)

// BuildMap returns the spaceship map. It is parsed once and shared: treat
// it as read-only.
func BuildMap() *GameMap {
	shipMapOnce.Do(func() {
		gm, err := parseShipMap(shipMapJSON)
		if err != nil {
			panic(fmt.Sprintf("shipmap.json: %v", err))
		}
		shipMap = gm
	})
	return shipMap
}

func parseShipMap(data []byte) (*GameMap, error) {
	var raw struct {
		Width     float64      `json:"width"`
		Height    float64      `json:"height"`
		Rooms     []MapRoom    `json:"rooms"`
		Hallways  []Hallway    `json:"hallways"`
		Walls     [][4]float64 `json:"walls"`
		Obstacles [][4]float64 `json:"obstacles"`
		Spawn     [2]float64   `json:"spawn"`
		Tasks     []struct {
			Type TaskType `json:"type"`
			Room string   `json:"room"`
			X    float64  `json:"x"`
			Y    float64  `json:"y"`
		} `json:"tasks"`
		Fixes []struct {
			Sabotage SabotageType `json:"sabotage"`
			ID       string       `json:"id"`
			Room     string       `json:"room"`
			X        float64      `json:"x"`
			Y        float64      `json:"y"`
		} `json:"fixes"`
		Waypoints []struct {
			X    float64 `json:"x"`
			Y    float64 `json:"y"`
			Room string  `json:"room"`
		} `json:"waypoints"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	rects := func(in [][4]float64) []Rect {
		out := make([]Rect, len(in))
		for i, r := range in {
			out[i] = Rect{r[0], r[1], r[2], r[3]}
		}
		return out
	}
	gm := &GameMap{
		Rooms:       raw.Rooms,
		Hallways:    raw.Hallways,
		Walls:       rects(raw.Walls),
		Obstacles:   rects(raw.Obstacles),
		SpawnPos:    Vec2{raw.Spawn[0], raw.Spawn[1]},
		Width:       raw.Width,
		Height:      raw.Height,
		FixStations: map[SabotageType][]FixStation{},
	}
	for i, t := range raw.Tasks {
		gm.TaskStations = append(gm.TaskStations, TaskStation{
			ID: fmt.Sprintf("task-%d", i+1), Type: t.Type, RoomID: t.Room, Position: Vec2{t.X, t.Y},
		})
	}
	for _, f := range raw.Fixes {
		gm.FixStations[f.Sabotage] = append(gm.FixStations[f.Sabotage], FixStation{
			ID: f.ID, RoomID: f.Room, Position: Vec2{f.X, f.Y},
		})
	}
	seen := map[string]bool{}
	for _, w := range raw.Waypoints {
		center := w.Room != "" && !seen[w.Room]
		seen[w.Room] = true
		gm.Waypoints = append(gm.Waypoints, Waypoint{Pos: Vec2{w.X, w.Y}, Room: w.Room, Center: center})
	}
	if len(gm.TaskStations) == 0 || len(gm.Waypoints) == 0 {
		return nil, fmt.Errorf("map has no task stations or waypoints")
	}
	gm.nav = newNavGrid(gm)
	return gm, nil
}

// IsWalkable checks if a circle at pos with given radius stays inside the map
// and doesn't touch a wall or a piece of furniture.
func (gm *GameMap) IsWalkable(pos Vec2, radius float64) bool {
	if pos.X-radius < 0 || pos.Y-radius < 0 || pos.X+radius > gm.Width || pos.Y+radius > gm.Height {
		return false
	}
	for _, w := range gm.Walls {
		if circleOverlapsRect(pos, radius, w) {
			return false
		}
	}
	for _, obs := range gm.Obstacles {
		if circleOverlapsRect(pos, radius, obs) {
			return false
		}
	}
	return true
}

// GetRoomAt returns the room ID the position is in, or "" if in a hallway
func (gm *GameMap) GetRoomAt(pos Vec2) string {
	for _, r := range gm.Rooms {
		if pointInRect(pos, r.Bounds) {
			return r.ID
		}
	}
	return ""
}
