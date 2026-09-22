package common

import (
	"rpg/src/d1"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/util"
)

// Hit zones for spells whose shape the original six could not express: a bolt that STOPS on what it hits, a ring,
// a box and a forward cone. They follow the same contract as the zones in def_hitzones.go - a name, a parameter
// list, and positions from an origin and a target - and are registered alongside them.

// BoltAoE (bolt_aoe(maxLen[, burstRadius])) is a flying bolt: the cells it passes through, ending on the first
// cell holding an actor or a solid object, or on the wall that stops it. ray_aoe and bresenham_aoe fly straight
// through actors, so neither can express a missile that is consumed by its first victim. With a burst radius it
// also covers the square of that radius around where it stopped, which is how Fireball and Elemental explode.
func BoltAoE(gridMap func() *gridmap.GridMap[*Actor, *Item, Object], args util.Arguments) *AoEResolver {
	maxLen := args.GetIntOrDefault(0, 20)
	burstRadius := args.GetIntOrDefault(1, 0)
	return &AoEResolver{
		GetHitPositions: func(origin, target geometry.Point) []geometry.Point {
			currentMap := gridMap()
			var result []geometry.Point
			impact := origin
			for _, p := range projectileLine(currentMap, origin, target, maxLen) {
				result = append(result, p)
				impact = p
				if currentMap.IsActorAt(p) || isSolidObjectAt(currentMap, p) {
					break
				}
			}
			for dy := -burstRadius; dy <= burstRadius; dy++ {
				for dx := -burstRadius; dx <= burstRadius; dx++ {
					p := impact.Add(geometry.Point{X: dx, Y: dy})
					if currentMap.Contains(p) && lineIsClear(currentMap, impact, p) {
						result = append(result, p)
					}
				}
			}
			return result
		},
	}
}

// HomingBoltAoE (homing_bolt_aoe(maxLen)) is Bone Spirit's flight (MI_Bonespirit): a bolt to the target cell that,
// reaching it without hitting anything, turns towards the closest actor within 18 CrawlTable rings in clear line
// (FindClosest), or flies on the way it came when there is none. Either way it stops at the first actor or wall.
func HomingBoltAoE(gridMap func() *gridmap.GridMap[*Actor, *Item, Object], args util.Arguments) *AoEResolver {
	maxLen := args.GetIntOrDefault(0, 112)
	return &AoEResolver{
		GetHitPositions: func(origin, target geometry.Point) []geometry.Point {
			m := gridMap()
			var result []geometry.Point
			fly := func(from, to geometry.Point, stopAtTo bool) bool { // true once something stopped it
				for _, p := range projectileLine(m, from, to, maxLen) {
					result = append(result, p)
					if m.IsActorAt(p) || isSolidObjectAt(m, p) {
						return true
					}
					if stopAtTo && p == to {
						return false
					}
				}
				return true
			}
			if fly(origin, target, true) {
				return result
			}
			for _, ring := range d1.CrawlRings[1:19] {
				for _, o := range ring {
					p := target.Add(geometry.Point{X: o[0], Y: o[1]})
					if p != origin && m.Contains(p) && m.IsActorAt(p) && lineIsClear(m, target, p) {
						fly(target, p, false)
						return result
					}
				}
			}
			fly(target, target.Add(target.Sub(origin)), false)
			return result
		},
	}
}

// RingAoE (ring_aoe(radius)) is the hollow square ring `radius` cells out from the origin, keeping only the cells
// the origin has a clear line to. Ring of Fire lays its wall on exactly this shape.
func RingAoE(gridMap func() *gridmap.GridMap[*Actor, *Item, Object], args util.Arguments) *AoEResolver {
	radius := args.GetIntOrDefault(0, 3)
	return &AoEResolver{
		GetHitPositions: func(origin, target geometry.Point) []geometry.Point {
			currentMap := gridMap()
			var result []geometry.Point
			for _, p := range d1Ring(origin, radius) {
				if currentMap.Contains(p) && lineIsClear(currentMap, origin, p) {
					result = append(result, p)
				}
			}
			return result
		},
	}
}

// BoxAoE (box_aoe(radius[|hi])) is the filled square from -radius to +hi (default radius) around the origin. Apocalypse blasts every
// monster in such a box, and Flash lights up the one square around the caster.
func BoxAoE(gridMap func() *gridmap.GridMap[*Actor, *Item, Object], args util.Arguments) *AoEResolver {
	radius := args.GetIntOrDefault(0, 1)
	hi := args.GetIntOrDefault(1, radius) // box_aoe(8|7): Apocalypse's -8 .. +7
	return &AoEResolver{
		GetHitPositions: func(origin, target geometry.Point) []geometry.Point {
			currentMap := gridMap()
			var result []geometry.Point
			for dy := -radius; dy <= hi; dy++ {
				for dx := -radius; dx <= hi; dx++ {
					if p := (geometry.Point{X: origin.X + dx, Y: origin.Y + dy}); currentMap.Contains(p) {
						result = append(result, p)
					}
				}
			}
			return result
		},
	}
}

// ConeAoE (cone_aoe(length, halfWidth)) is a front that sets off from the origin towards the target and widens as
// it goes: Flame Wave's rolling wall of fire. Cells behind a wall are dropped, so the wave does not wash through
// rock.
func ConeAoE(gridMap func() *gridmap.GridMap[*Actor, *Item, Object], args util.Arguments) *AoEResolver {
	length := args.GetIntOrDefault(0, 8)
	halfWidth := args.GetIntOrDefault(1, 3)
	return &AoEResolver{
		GetHitPositions: func(origin, target geometry.Point) []geometry.Point {
			currentMap := gridMap()
			dir := D1DirTo(origin, target)
			forward, left, right := D1Dirs[dir], D1Dirs[(dir+2)%8], D1Dirs[(dir+6)%8]
			var result []geometry.Point
			for step := 1; step <= length; step++ {
				centre := origin.Add(geometry.Point{X: forward.X * step, Y: forward.Y * step})
				if !currentMap.Contains(centre) || !currentMap.IsTransparent(centre) {
					break
				}
				result = append(result, centre)
				spread := min(halfWidth, step/2+1)
				for _, side := range []geometry.Point{left, right} {
					for w := 1; w <= spread; w++ {
						p := centre.Add(geometry.Point{X: side.X * w, Y: side.Y * w})
						if !currentMap.Contains(p) || !currentMap.IsTransparent(p) {
							break
						}
						result = append(result, p)
					}
				}
			}
			return result
		},
	}
}

// ChainAoE (chain_aoe(jumps, reach)) is a beam that re-aims. It lays the cells from the origin to the target,
// and then keeps going: from the actor it just reached to the nearest actor it has not reached yet, within
// `reach` cells and in clear line, for at most `jumps` further legs. Chain Lightning is exactly this shape, and
// so is anything else that wants to arc from victim to victim; bolt_aoe stops at one, ray_aoe never turns.
func ChainAoE(gridMap func() *gridmap.GridMap[*Actor, *Item, Object], args util.Arguments) *AoEResolver {
	jumps := args.GetIntOrDefault(0, 6)
	reach := args.GetIntOrDefault(1, 7)
	return &AoEResolver{
		GetHitPositions: func(origin, target geometry.Point) []geometry.Point {
			currentMap := gridMap()
			var result []geometry.Point
			reached := map[geometry.Point]bool{origin: true}
			from, to := origin, target
			for leg := 0; leg <= jumps; leg++ {
				result = append(result, projectileLine(currentMap, from, to, 20)...)
				reached[to] = true
				next, found := geometry.Point{}, false
				for _, actor := range currentMap.Actors() {
					p := actor.MapPosition()
					d := geometry.DistanceChebyshev(to, p)
					if reached[p] || d > reach || !lineIsClear(currentMap, to, p) {
						continue
					}
					if !found || d < geometry.DistanceChebyshev(to, next) {
						next, found = p, true
					}
				}
				if !found {
					break
				}
				from, to = to, next
			}
			return result
		},
	}
}

// projectileLine is the cells a missile from `origin` towards `target` crosses, up to maxLen or the first wall.
func projectileLine(currentMap *gridmap.GridMap[*Actor, *Item, Object], origin, target geometry.Point, maxLen int) []geometry.Point {
	open := func(p geometry.Point) bool { return currentMap.Contains(p) && currentMap.IsTransparent(p) }
	d := target.Sub(origin)
	n := max(geometry.Abs(d.X), geometry.Abs(d.Y))
	if n == 0 {
		return nil
	}
	k := (maxLen + n - 1) / n
	far := origin.Add(geometry.Point{X: d.X * k, Y: d.Y * k})
	var cells []geometry.Point
	for _, p := range geometry.LineOfSight(origin, far, func(p geometry.Point) bool { return p == origin || open(p) })[1:] {
		if len(cells) == maxLen || !open(p) {
			break
		}
		cells = append(cells, p)
	}
	return cells
}

// lineIsClear: no wall stands between the two cells.
func lineIsClear(currentMap *gridmap.GridMap[*Actor, *Item, Object], from, to geometry.Point) bool {
	open := func(p geometry.Point) bool { return currentMap.Contains(p) && currentMap.IsTransparent(p) }
	los := geometry.LineOfSight(from, to, func(p geometry.Point) bool { return p == from || p == to || open(p) })
	return len(los) > 0 && los[len(los)-1] == to
}
