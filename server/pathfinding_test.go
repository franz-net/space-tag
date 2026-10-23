package main

import "testing"

// walkLikeAI moves a player circle from `from` toward `to` the same way the
// game does for AI bots (straight line + wall sliding) and reports whether it
// arrived within AIWaypointReach before stalling for AIStuckThreshold ticks.
func walkLikeAI(gm *GameMap, from, to Vec2) (Vec2, bool) {
	pos := from
	dt := 1.0 / float64(TickRate)
	stuck := 0
	for i := 0; i < 2000; i++ {
		dx, dy := to.X-pos.X, to.Y-pos.Y
		if dx*dx+dy*dy < AIWaypointReach*AIWaypointReach {
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
	for _, leg := range FindPath(from, to) {
		var ok bool
		pos, ok = walkLikeAI(gm, pos, leg)
		if !ok {
			return pos, false
		}
	}
	return pos, true
}

func TestWaypointsWalkable(t *testing.T) {
	gm := BuildMap()
	for i, wp := range DefaultWaypoints {
		if !gm.IsWalkable(wp, PlayerRadius) {
			t.Errorf("waypoint %d at %v is not walkable", i, wp)
		}
	}
}

func TestWaypointAdjSymmetric(t *testing.T) {
	for a, ns := range WaypointAdj {
		for _, b := range ns {
			found := false
			for _, back := range WaypointAdj[b] {
				if back == a {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("waypoint %d lists %d but %d does not list %d", a, b, b, a)
			}
		}
	}
	for i := range DefaultWaypoints {
		if len(WaypointAdj[i]) == 0 {
			t.Errorf("waypoint %d has no neighbors", i)
		}
	}
}

func TestWaypointEdgesWalkable(t *testing.T) {
	gm := BuildMap()
	for a, ns := range WaypointAdj {
		for _, b := range ns {
			if end, ok := walkLikeAI(gm, DefaultWaypoints[a], DefaultWaypoints[b]); !ok {
				t.Errorf("edge %d -> %d stalls at (%.0f, %.0f)", a, b, end.X, end.Y)
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
