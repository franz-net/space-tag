package main

import "math"

// Pre-defined waypoint positions matching the map layout in gamemap.go.
// Rooms: medbay, cafeteria, navigation, engine, storage, reactor (centers)
// Hallways: 7 connectors between adjacent rooms (centers)
//
// Indices:
//   0 medbay center
//   1 cafeteria center
//   2 navigation center
//   3 engine center
//   4 storage center
//   5 reactor center
//   6 hallway medbay <-> cafeteria
//   7 hallway cafeteria <-> navigation
//   8 hallway engine <-> storage
//   9 hallway storage <-> reactor
//  10 hallway medbay <-> engine
//  11 hallway cafeteria <-> storage
//  12 hallway navigation <-> reactor
// Room waypoints are placed in open areas away from obstacles:
//   Medbay beds:     {150,170,60,70} and {390,170,60,70} — waypoint below beds
//   Cafeteria table: {1040,265,120,40} — waypoint above table
//   Navigation desk: {1820,160,160,25} — waypoint below desk
//   Engine turbines: {170,1040,60,60} and {330,1040,60,60} — waypoint above
//   Storage crates:  {960,980,80,70} and {1120,1080,80,70} — waypoint above
//   Reactor core:    {1860,1030,80,80} — waypoint above
// Main waypoints: room centers (placed in obstacle-free zones) + hallway centers.
// Secondary waypoints (13+) give AI extra routing inside rooms so they can
// navigate around furniture instead of getting stuck on obstacle edges.
var DefaultWaypoints = []Vec2{
	// Primary room waypoints
	{300, 300},   // 0 medbay — below the beds (y:170-240)
	{1100, 200},  // 1 cafeteria — above the table (y:265)
	{1900, 300},  // 2 navigation — below the desk (y:160-185)
	{300, 960},   // 3 engine — above the turbines (y:1040)
	{1100, 960},  // 4 storage — above the crates (y:980)
	{1900, 960},  // 5 reactor — above the core (y:1030)
	// Hallway waypoints
	{700, 250},   // 6 hall medbay-caf
	{1500, 250},  // 7 hall caf-nav
	{700, 1050},  // 8 hall engine-sto
	{1500, 1050}, // 9 hall sto-reactor
	{250, 650},   // 10 hall medbay-engine
	{1100, 650},  // 11 hall caf-storage
	{1850, 650},  // 12 hall nav-reactor
	// Secondary room waypoints — extra points for obstacle avoidance
	{300, 160},   // 13 medbay top (between the two beds)
	{200, 350},   // 14 medbay bottom-left (clear of right hallway)
	{1100, 350},  // 15 cafeteria bottom (below table, above hallway)
	{1750, 250},  // 16 navigation left (beside desk, clear of it)
	{1800, 350},  // 17 navigation bottom
	{170, 960},   // 18 engine left (left of turbines)
	{430, 960},   // 19 engine right (right of turbines, clear of hallway)
	{1100, 1100}, // 20 storage bottom (between crate stacks)
	{1900, 1150}, // 21 reactor bottom (below core)
	{1780, 960},  // 22 reactor left (left of core)
}

var WaypointAdj = map[int][]int{
	0:  {6, 10, 13, 14},
	1:  {6, 7, 11, 15},
	2:  {7, 12, 16, 17},
	3:  {8, 10, 18, 19},
	4:  {8, 9, 11, 20},
	5:  {9, 12, 21, 22},
	6:  {0, 1},
	7:  {1, 2},
	8:  {3, 4},
	9:  {4, 5},
	10: {0, 3},
	11: {1, 4},
	12: {2, 5},
	// Secondary waypoints connect to their room's primary + neighbors
	13: {0, 14},        // medbay top ↔ medbay center, medbay bottom-left
	14: {0, 13, 10},    // medbay bottom-left ↔ medbay center, top, vertical hall
	15: {1, 11},        // cafeteria bottom ↔ caf center, vertical hall
	16: {2, 17},        // nav top ↔ nav center, nav bottom
	17: {2, 16, 12},    // nav bottom ↔ nav center, nav top, vertical hall
	18: {3, 19},        // engine left ↔ engine center, engine right
	19: {3, 18, 8},     // engine right ↔ engine center, left, hallway
	20: {4, 9, 8},      // storage bottom ↔ storage center, hallways
	21: {5, 22},        // reactor bottom ↔ reactor center, reactor left
	22: {5, 21, 12},    // reactor left ↔ reactor center, bottom, vertical hall
}

// nearestWaypoint returns the index of the closest waypoint (Euclidean) to pos
func nearestWaypoint(pos Vec2) int {
	best := 0
	bestDist := math.MaxFloat64
	for i, wp := range DefaultWaypoints {
		dx := pos.X - wp.X
		dy := pos.Y - wp.Y
		d := dx*dx + dy*dy
		if d < bestDist {
			bestDist = d
			best = i
		}
	}
	return best
}

// FindPath returns a list of waypoints + final destination from `from` to `to`.
// Uses BFS on the waypoint graph.
func FindPath(from, to Vec2) []Vec2 {
	start := nearestWaypoint(from)
	end := nearestWaypoint(to)

	if start == end {
		return []Vec2{to}
	}

	// BFS
	visited := map[int]bool{start: true}
	parent := map[int]int{start: -1}
	queue := []int{start}
	found := false

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if curr == end {
			found = true
			break
		}
		for _, neighbor := range WaypointAdj[curr] {
			if !visited[neighbor] {
				visited[neighbor] = true
				parent[neighbor] = curr
				queue = append(queue, neighbor)
			}
		}
	}

	if !found {
		return []Vec2{to}
	}

	// Reconstruct path: end → start, then reverse, then append destination
	pathIdx := []int{}
	for n := end; n != -1; n = parent[n] {
		pathIdx = append([]int{n}, pathIdx...)
	}

	path := make([]Vec2, 0, len(pathIdx)+1)
	for _, idx := range pathIdx {
		path = append(path, DefaultWaypoints[idx])
	}
	path = append(path, to)

	return path
}
