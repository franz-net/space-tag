package main

import "testing"

// walkLikeAI moves a player circle from `from` toward `to` the same way the
// game does for AI bots (straight line + wall sliding) and reports whether it
// arrived within AIWaypointReach before stalling for AIStuckThreshold ticks.
func walkLikeAI(gm *GameMap, from, to Vec2) (Vec2, bool) {
	return walkLeg(gm, from, to, AIWaypointReach)
}

func walkLeg(gm *GameMap, from, to Vec2, reach float64) (Vec2, bool) {
	pos := from
	dt := 1.0 / float64(TickRate)
	stuck := 0
	for i := 0; i < 2000; i++ {
		dx, dy := to.X-pos.X, to.Y-pos.Y
		if dx*dx+dy*dy < reach*reach {
			return pos, true
		}
		nx, ny := normalize(dx, dy)
		next := ResolveMovement(gm, pos, nx*PlayerSpeed*dt, ny*PlayerSpeed*dt)
		mx, my := next.X-pos.X, next.Y-pos.Y
		if mx*mx+my*my < 1 {
			stuck++
			if stuck >= AIStuckThreshold {
				return pos, false
			}
		} else {
			stuck = 0
		}
		pos = next
	}
	return pos, false
}

// walkPath follows a FindPath route leg by leg.
func walkPath(gm *GameMap, from, to Vec2) (Vec2, bool) {
	pos := from
	path := FindPath(from, to)
	for i, leg := range path {
		reach := AICornerReach
		if i == len(path)-1 {
			reach = AIWaypointReach
		}
		var ok bool
		pos, ok = walkLeg(gm, pos, leg, reach)
		if !ok {
			return pos, false
		}
	}
	return pos, true
}

func TestWaypointsWalkable(t *testing.T) {
	gm := BuildMap()
	for i, wp := range gm.Waypoints {
		if !gm.IsWalkable(wp.Pos, PlayerRadius) {
			t.Errorf("waypoint %d at %v is not walkable", i, wp.Pos)
		}
	}
}

func TestStationsWalkable(t *testing.T) {
	gm := BuildMap()
	for _, s := range gm.TaskStations {
		if !gm.IsWalkable(s.Position, PlayerRadius) {
			t.Errorf("task station %s at %v is not walkable", s.ID, s.Position)
		}
	}
	for typ, fs := range gm.FixStations {
		for _, f := range fs {
			if !gm.IsWalkable(f.Position, PlayerRadius) {
				t.Errorf("%s fix station %s at %v is not walkable", typ, f.ID, f.Position)
			}
		}
	}
}

// Everyone starts in a row centred on the spawn point; the row must fit
// for a full lobby.
func TestSpawnRowWalkable(t *testing.T) {
	gm := BuildMap()
	ids := make([]string, 10)
	for i := range ids {
		ids[i] = string(rune('a' + i))
	}
	gs := NewGameState(gm, ids, map[string]Role{}, nil, DefaultSettings(), map[string]string{})
	for id, p := range gs.Positions {
		if !gm.IsWalkable(p, PlayerRadius) {
			t.Errorf("spawn slot for %s at %v is not walkable", id, p)
		}
	}
}

// Every room and hallway waypoint can reach every other one.
func TestWaypointsConnected(t *testing.T) {
	gm := BuildMap()
	for i, a := range gm.Waypoints {
		for j, b := range gm.Waypoints {
			if i == j {
				continue
			}
			if end, ok := walkPath(gm, a.Pos, b.Pos); !ok {
				t.Errorf("waypoint %d -> %d stalls at (%.0f, %.0f)", i, j, end.X, end.Y)
			}
		}
	}
}

// TestAIRoutesBetweenAllStations checks that a bot standing at any task or
// fix station can reach every other one. This is what keeps bots from
// "giving up" on tasks after map or waypoint changes.
func TestAIRoutesBetweenAllStations(t *testing.T) {
	gm := BuildMap()
	type stop struct {
		name string
		pos  Vec2
	}
	var stops []stop
	for _, s := range buildTaskStations() {
		stops = append(stops, stop{s.ID, s.Position})
	}
	for typ, fs := range SabotageFixStations {
		for _, f := range fs {
			stops = append(stops, stop{string(typ) + ":" + f.ID, f.Position})
		}
	}

	for _, from := range stops {
		// Start where a bot would actually be standing after arriving.
		start, ok := walkPath(gm, gm.SpawnPos, from.pos)
		if !ok {
			t.Errorf("spawn -> %s unreachable (stalled at %.0f, %.0f)", from.name, start.X, start.Y)
			continue
		}
		for _, to := range stops {
			if from.name == to.name {
				continue
			}
			if end, ok := walkPath(gm, start, to.pos); !ok {
				t.Errorf("%s -> %s stalls at (%.0f, %.0f)", from.name, to.name, end.X, end.Y)
			}
		}
	}
}
