package main

import (
	"container/heap"
	"math"
)

// Bots find their way with A* over a grid laid on the map, then pull the
// route tight so they walk straight lines between corners. The grid is
// built from the same collision the players use, so any map exported from
// Tiled works without hand-placed waypoints.

const (
	navCell      = 16.0              // grid spacing in game units
	navClearance = PlayerRadius + 3  // keep routes a little off walls and furniture
	navLineStep  = 6.0               // sampling step when checking a straight line
)

type navGrid struct {
	gm   *GameMap
	cols int
	rows int
	open []bool // cell centre is walkable with clearance
}

func newNavGrid(gm *GameMap) *navGrid {
	n := &navGrid{gm: gm, cols: int(gm.Width / navCell), rows: int(gm.Height / navCell)}
	n.open = make([]bool, n.cols*n.rows)
	for r := 0; r < n.rows; r++ {
		for c := 0; c < n.cols; c++ {
			n.open[r*n.cols+c] = gm.IsWalkable(n.center(c, r), navClearance)
		}
	}
	return n
}

func (n *navGrid) center(c, r int) Vec2 {
	return Vec2{(float64(c) + 0.5) * navCell, (float64(r) + 0.5) * navCell}
}

func (n *navGrid) cellOf(p Vec2) (int, int) {
	c := int(p.X / navCell)
	r := int(p.Y / navCell)
	return max(0, min(n.cols-1, c)), max(0, min(n.rows-1, r))
}

func (n *navGrid) isOpen(c, r int) bool {
	return c >= 0 && r >= 0 && c < n.cols && r < n.rows && n.open[r*n.cols+c]
}

// nearestOpen finds the open cell closest to p (searching outward in rings).
func (n *navGrid) nearestOpen(p Vec2) (int, int, bool) {
	c0, r0 := n.cellOf(p)
	if n.isOpen(c0, r0) {
		return c0, r0, true
	}
	for ring := 1; ring < 12; ring++ {
		best, bc, br := math.MaxFloat64, 0, 0
		for r := r0 - ring; r <= r0+ring; r++ {
			for c := c0 - ring; c <= c0+ring; c++ {
				if (r != r0-ring && r != r0+ring && c != c0-ring && c != c0+ring) || !n.isOpen(c, r) {
					continue
				}
				q := n.center(c, r)
				if d := (q.X-p.X)*(q.X-p.X) + (q.Y-p.Y)*(q.Y-p.Y); d < best {
					best, bc, br = d, c, r
				}
			}
		}
		if best < math.MaxFloat64 {
			return bc, br, true
		}
	}
	return 0, 0, false
}

// clearLine reports whether a player circle can walk straight from a to b.
func (n *navGrid) clearLine(a, b Vec2) bool {
	dx, dy := b.X-a.X, b.Y-a.Y
	dist := math.Sqrt(dx*dx + dy*dy)
	steps := int(dist/navLineStep) + 1
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		if !n.gm.IsWalkable(Vec2{a.X + dx*t, a.Y + dy*t}, PlayerRadius+1) {
			return false
		}
	}
	return true
}

type navItem struct {
	cell int
	f    float64
}
type navHeap []navItem

func (h navHeap) Len() int           { return len(h) }
func (h navHeap) Less(i, j int) bool { return h[i].f < h[j].f }
func (h navHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *navHeap) Push(x any)        { *h = append(*h, x.(navItem)) }
func (h *navHeap) Pop() any {
	old := *h
	it := old[len(old)-1]
	*h = old[:len(old)-1]
	return it
}

// route runs A* between two cells and returns the cell centres on the way.
func (n *navGrid) route(sc, sr, ec, er int) []Vec2 {
	start, end := sr*n.cols+sc, er*n.cols+ec
	g := map[int]float64{start: 0}
	parent := map[int]int{start: -1}
	closed := map[int]bool{}
	h := func(cell int) float64 {
		c, r := cell%n.cols, cell/n.cols
		dx, dy := math.Abs(float64(c-ec)), math.Abs(float64(r-er))
		return math.Max(dx, dy) + (math.Sqrt2-1)*math.Min(dx, dy) // octile
	}
	pq := &navHeap{{start, h(start)}}
	for pq.Len() > 0 {
		cur := heap.Pop(pq).(navItem).cell
		if cur == end {
			var cells []Vec2
			for at := end; at != -1; at = parent[at] {
				cells = append(cells, n.center(at%n.cols, at/n.cols))
			}
			for i, j := 0, len(cells)-1; i < j; i, j = i+1, j-1 {
				cells[i], cells[j] = cells[j], cells[i]
			}
			return cells
		}
		if closed[cur] {
			continue
		}
		closed[cur] = true
		c, r := cur%n.cols, cur/n.cols
		for dr := -1; dr <= 1; dr++ {
			for dc := -1; dc <= 1; dc++ {
				if (dc == 0 && dr == 0) || !n.isOpen(c+dc, r+dr) {
					continue
				}
				// No cutting corners: diagonal steps need both sides open
				if dc != 0 && dr != 0 && (!n.isOpen(c+dc, r) || !n.isOpen(c, r+dr)) {
					continue
				}
				next := (r+dr)*n.cols + c + dc
				cost := g[cur] + 1
				if dc != 0 && dr != 0 {
					cost = g[cur] + math.Sqrt2
				}
				if old, ok := g[next]; ok && old <= cost {
					continue
				}
				g[next] = cost
				parent[next] = cur
				heap.Push(pq, navItem{next, cost + h(next)})
			}
		}
	}
	return nil
}

// FindPath returns the points to walk through from `from` to `to`, ending
// at `to`. If there's no route it returns just `to` (the bot's stuck
// detection then gives up on that goal).
func FindPath(from, to Vec2) []Vec2 {
	n := BuildMap().nav
	if n.clearLine(from, to) {
		return []Vec2{to}
	}
	sc, sr, ok1 := n.nearestOpen(from)
	ec, er, ok2 := n.nearestOpen(to)
	if !ok1 || !ok2 {
		return []Vec2{to}
	}
	cells := n.route(sc, sr, ec, er)
	if cells == nil {
		return []Vec2{to}
	}
	cells = append(cells, to)

	// Pull the route tight: from each corner, jump to the farthest point
	// still reachable in a straight line.
	path := []Vec2{}
	cur := from
	i := 0
	for i < len(cells) {
		j := len(cells) - 1
		for j > i && !n.clearLine(cur, cells[j]) {
			j--
		}
		path = append(path, cells[j])
		cur = cells[j]
		i = j + 1
	}
	return path
}
