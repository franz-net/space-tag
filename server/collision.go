package main

import "math"

const PlayerRadius = 16.0
const PlayerSpeed = 200.0 // units per second

func pointInRect(p Vec2, r Rect) bool {
	return p.X >= r.X && p.X <= r.X+r.W && p.Y >= r.Y && p.Y <= r.Y+r.H
}

func circleInRect(center Vec2, radius float64, r Rect) bool {
	return center.X-radius >= r.X &&
		center.X+radius <= r.X+r.W &&
		center.Y-radius >= r.Y &&
		center.Y+radius <= r.Y+r.H
}

// ResolveMovement tries to move from `from` by `delta`, checking walkability.
// Uses slide-along-wall approach: try full move, then X-only, then Y-only.
func ResolveMovement(gm *GameMap, from Vec2, dx, dy float64) Vec2 {
	// Try full movement
	full := Vec2{from.X + dx, from.Y + dy}
	if gm.IsWalkable(full, PlayerRadius) {
		return full
	}

	// Try X only
	xOnly := Vec2{from.X + dx, from.Y}
	if gm.IsWalkable(xOnly, PlayerRadius) {
		return xOnly
	}

	// Try Y only
	yOnly := Vec2{from.X, from.Y + dy}
	if gm.IsWalkable(yOnly, PlayerRadius) {
		return yOnly
	}

	// Can't move
	return from
}

// Normalize a direction vector to unit length
func normalize(x, y float64) (float64, float64) {
	mag := math.Sqrt(x*x + y*y)
	if mag == 0 {
		return 0, 0
	}
	return x / mag, y / mag
}
