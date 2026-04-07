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
var DefaultWaypoints = []Vec2{
	{300, 250},   // 0 medbay
	{1100, 250},  // 1 cafeteria
	{1900, 250},  // 2 navigation
	{300, 1050},  // 3 engine
	{1100, 1050}, // 4 storage
	{1900, 1050}, // 5 reactor
	{700, 250},   // 6 hall medbay-caf
	{1500, 250},  // 7 hall caf-nav
	{700, 1050},  // 8 hall engine-sto
	{1500, 1050}, // 9 hall sto-reactor
	{250, 650},   // 10 hall medbay-engine
	{1100, 650},  // 11 hall caf-storage
	{1850, 650},  // 12 hall nav-reactor
}

var WaypointAdj = map[int][]int{
	0:  {6, 10},
	1:  {6, 7, 11},
	2:  {7, 12},
	3:  {8, 10},
	4:  {8, 9, 11},
	5:  {9, 12},
	6:  {0, 1},
	7:  {1, 2},
	8:  {3, 4},
	9:  {4, 5},
	10: {0, 3},
	11: {1, 4},
	12: {2, 5},
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
