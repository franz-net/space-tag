package main

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

type GameMap struct {
	Rooms     []MapRoom `json:"rooms"`
	Hallways  []Hallway `json:"hallways"`
	Walls     []Rect    `json:"walls"`
	Obstacles []Rect    `json:"obstacles"` // furniture/props that block movement
	SpawnPos  Vec2      `json:"spawnPos"`
	Width     float64   `json:"width"`
	Height    float64   `json:"height"`
}

// BuildMap creates the spaceship map with 6 rooms connected by hallways.
//
// Layout (roughly):
//
//	Medbay -------- Navigation
//	  |                |
//	  |    Cafeteria   |
//	  |   (center)     |
//	  |                |
//	Engine -- Storage -- Reactor
func BuildMap() *GameMap {
	// Layout: 6 rooms in 2 rows of 3, with cafeteria as central hub
	//   Medbay -------- Cafeteria -------- Navigation
	//     |                 |                  |
	//     |                 |                  |
	//   Engine -------- Storage --------- Reactor
	//
	// All hallways overlap rooms by >= 50px so player (radius 16) can transition
	rooms := []MapRoom{
		// Top row
		{ID: "medbay", Label: "Medbay", Bounds: Rect{100, 100, 400, 300}, Color: "#2D6A4F"},
		{ID: "cafeteria", Label: "Cafeteria", Bounds: Rect{900, 100, 400, 300}, Color: "#4A5568"},
		{ID: "navigation", Label: "Navigation", Bounds: Rect{1700, 100, 400, 300}, Color: "#1E40AF"},
		// Bottom row
		{ID: "engine", Label: "Engine", Bounds: Rect{100, 900, 400, 300}, Color: "#92400E"},
		{ID: "storage", Label: "Storage", Bounds: Rect{900, 900, 400, 300}, Color: "#5B21B6"},
		{ID: "reactor", Label: "Reactor", Bounds: Rect{1700, 900, 400, 300}, Color: "#991B1B"},
	}

	// Hallways — each rect overlaps both connected rooms by 60px
	hallways := []Hallway{
		// Top row horizontal connectors
		// Medbay <-> Cafeteria
		{Bounds: Rect{440, 200, 520, 100}},
		// Cafeteria <-> Navigation
		{Bounds: Rect{1240, 200, 520, 100}},

		// Bottom row horizontal connectors
		// Engine <-> Storage
		{Bounds: Rect{440, 1000, 520, 100}},
		// Storage <-> Reactor
		{Bounds: Rect{1240, 1000, 520, 100}},

		// Vertical connectors between top and bottom rows
		// Medbay <-> Engine
		{Bounds: Rect{200, 340, 100, 620}},
		// Cafeteria <-> Storage
		{Bounds: Rect{1050, 340, 100, 620}},
		// Navigation <-> Reactor
		{Bounds: Rect{1800, 340, 100, 620}},
	}

	// Obstacles — collision boxes for room furniture. Each box must keep
	// ≥50px clearance from hallway entrances so players (radius 16) can
	// pass through without getting stuck.
	//
	// Hallway entrances to remember:
	//   Medbay right: x≈440, y:200-300   |  Medbay bottom: x:200-300, y≈340
	//   Caf bottom: x:1050-1150, y≈340   |  Engine right: x≈440, y:1000-1100
	//   Storage left: x≈960, y:1000-1100 |  Storage right: x≈1240, y:1000-1100
	obstacles := []Rect{
		// Medbay: two beds in upper portion (clear of bottom/right hallways)
		{150, 170, 60, 70},
		{390, 170, 60, 70},

		// Cafeteria: table in center (narrow enough to leave side passages,
		// and above the bottom hallway entrance at y:340)
		{1040, 240, 120, 50},

		// Navigation: console desk at top
		{1820, 160, 160, 25},

		// Engine: two turbines centered (right one stays clear of x:440 hallway)
		{170, 1040, 60, 60},
		{330, 1040, 60, 60},

		// Storage: two crate stacks (clear of left x:960 and right x:1240 hallways)
		{960, 980, 80, 70},
		{1120, 1080, 80, 70},

		// Reactor: containment core in center
		{1860, 1030, 80, 80},
	}

	gm := &GameMap{
		Rooms:     rooms,
		Hallways:  hallways,
		Walls:     nil,
		Obstacles: obstacles,
		SpawnPos:  Vec2{1100, 170}, // cafeteria upper area (below label, above table)
		Width:     2200,
		Height:    1300,
	}

	return gm
}

// IsWalkable checks if a circle at pos with given radius is fully inside any
// room or hallway AND does not overlap any obstacle (furniture/props).
func (gm *GameMap) IsWalkable(pos Vec2, radius float64) bool {
	inBounds := false
	for _, r := range gm.Rooms {
		if circleInRect(pos, radius, r.Bounds) {
			inBounds = true
			break
		}
	}
	if !inBounds {
		for _, h := range gm.Hallways {
			if circleInRect(pos, radius, h.Bounds) {
				inBounds = true
				break
			}
		}
	}
	if !inBounds {
		return false
	}
	// Reject if overlapping any obstacle
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
