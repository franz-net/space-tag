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

// circleOverlapsRect returns true if a circle touches or overlaps a rectangle.
// Used for obstacle collision — if the player circle overlaps any obstacle,
// the position is blocked.
func circleOverlapsRect(center Vec2, radius float64, r Rect) bool {
	// Find the closest point on the rectangle to the circle center
	closestX := math.Max(r.X, math.Min(center.X, r.X+r.W))
	closestY := math.Max(r.Y, math.Min(center.Y, r.Y+r.H))
	dx := center.X - closestX
	dy := center.Y - closestY
	return dx*dx+dy*dy <= radius*radius
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
