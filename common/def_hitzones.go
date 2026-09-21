package common

import (
	"math"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/util"
)

func GetAllHitZones() map[string]func(gridMap func() *gridmap.GridMap[*Actor, *Item, Object], args util.Arguments) *AoEResolver {
	return map[string]func(gridMap func() *gridmap.GridMap[*Actor, *Item, Object], args util.Arguments) *AoEResolver{
		"target_only":     OneTargetTile,
		"ray_aoe":         RayAoE,
		"bresenham_aoe":   BresenhamAoE,
		"dijkstra_aoe":    DijkstraAoE,
		"wall_aoe":        WallAoE,
		"neighbors_only":  TilesAroundTarget,
		"bolt_aoe":        BoltAoE,
		"homing_bolt_aoe": HomingBoltAoE,
		"ring_aoe":        RingAoE,
		"box_aoe":         BoxAoE,
		"cone_aoe":        ConeAoE,
		"chain_aoe":       ChainAoE,
	}
}

// These determine aoe zones for spells and throwable items
func HitZoneFromString(gridMap func() *gridmap.GridMap[*Actor, *Item, Object], zone string) *AoEResolver {
	allHitZones := GetAllHitZones()
	if util.LooksLikeAFunction(zone) {
		parametrizedHitZone, args := util.GetNameAndArgs(zone)
		if resolver, ok := allHitZones[parametrizedHitZone]; ok {
			return resolver(gridMap, args)
		}
	}
	// a bare name is a zone that takes no parameters, the way NewTargetingRulesFromString reads one
	if resolver, ok := allHitZones[zone]; ok {
		return resolver(gridMap, util.Arguments{})
	}
	return OneTargetTile(gridMap, util.Arguments{})
}
func DijkstraAoE(gridMap func() *gridmap.GridMap[*Actor, *Item, Object], args util.Arguments) *AoEResolver {
	maxDistance := args.GetInt(0)
	return &AoEResolver{
		GetHitPositions: func(origin, target geometry.Point) []geometry.Point {
			currentMap := gridMap()
			return toList(currentMap.GetDijkstraMapWithActorsNotBlocking(target, maxDistance))
		},
	}
}
func RayAoE(gridMap func() *gridmap.GridMap[*Actor, *Item, Object], args util.Arguments) *AoEResolver {
	maxDistance := args.GetInt(0)
	return &AoEResolver{
		GetHitPositions: func(originGrid, targetGrid geometry.Point) []geometry.Point {
			currentMap := gridMap()
			var result []geometry.Point
			origin := originGrid.ToCenteredPointF()
			target := targetGrid.ToCenteredPointF()

			rayDirection := target.Sub(origin)
			hitInfo := currentMap.RayCast(origin, rayDirection, func(pos geometry.Point) bool {
				return !currentMap.IsWalkable(pos)
			})

			distCounter := 0
			for _, arrayPos := range hitInfo.TraversedGridCells {
				mapPos := geometry.Point{X: int(arrayPos[0]), Y: int(arrayPos[1])}
				if mapPos == originGrid {
					continue
				}
				result = append(result, mapPos)
				distCounter++
				if distCounter >= maxDistance {
					break
				}
			}
			return result
		},
	}
}
func BresenhamAoE(gridMap func() *gridmap.GridMap[*Actor, *Item, Object], args util.Arguments) *AoEResolver {
	maxDistance := args.GetInt(0)
	return &AoEResolver{
		GetHitPositions: func(originGrid, targetGrid geometry.Point) []geometry.Point {
			currentMap := gridMap()
			var result []geometry.Point
			traversedGridCells := currentMap.BresenhamLine(originGrid, targetGrid, func(pos geometry.Point) bool {
				return !currentMap.IsWalkable(pos)
			})

			distCounter := 0
			for _, mapPos := range traversedGridCells {
				if mapPos == originGrid {
					continue
				}
				result = append(result, mapPos)
				distCounter++
				if distCounter >= maxDistance {
					break
				}
			}
			return result
		},
	}
}

func WallAoE(gridMap func() *gridmap.GridMap[*Actor, *Item, Object], args util.Arguments) *AoEResolver {
	wallParts := args.GetInt(0)
	return &AoEResolver{
		GetHitPositions: func(origin, target geometry.Point) []geometry.Point {
			//                   x
			//                   x
			// player --> @      X <-- Target was here
			//                   x
			//                   x

			// we want a wall of fire on either the X or Y axis, above or below the below
			// depending on where he selected the target to be
			var result []geometry.Point

			directionToTarget := target.Sub(origin)
			halfParts := wallParts / 2
			// find the largest component of the direction vector to determine the axis

			if math.Abs(float64(directionToTarget.X)) > math.Abs(float64(directionToTarget.Y)) {
				// X axis
				// we want to place the wall left or right of the target
				// so check the direction of the X component
				if directionToTarget.X > 0 {
					// right
					for i := -halfParts; i <= halfParts; i++ {
						result = append(result, target.Add(geometry.Point{X: 0, Y: i}))
					}
				} else {
					// left
					for i := -halfParts; i <= halfParts; i++ {
						result = append(result, target.Add(geometry.Point{X: 0, Y: i}))
					}
				}
			} else {
				// Y axis
				// we want to place the wall above or below the target
				// so check the direction of the Y component
				if directionToTarget.Y > 0 {
					// below
					for i := -halfParts; i <= halfParts; i++ {
						result = append(result, target.Add(geometry.Point{X: i, Y: 0}))
					}
				} else {
					// above
					for i := -halfParts; i <= halfParts; i++ {
						result = append(result, target.Add(geometry.Point{X: i, Y: 0}))
					}
				}
			}

			return result
		},
	}
}

func OneTargetTile(gridMap func() *gridmap.GridMap[*Actor, *Item, Object], args util.Arguments) *AoEResolver {
	return &AoEResolver{
		GetHitPositions: func(origin, target geometry.Point) []geometry.Point {
			return []geometry.Point{target}
		},
	}
}

func TilesAroundTarget(gridmap func() *gridmap.GridMap[*Actor, *Item, Object], args util.Arguments) *AoEResolver {
	return &AoEResolver{
		GetHitPositions: func(origin, target geometry.Point) []geometry.Point {
			currentMap := gridmap()
			neighbors := currentMap.NeighborsAll(target, func(p geometry.Point) bool {
				return currentMap.Contains(p) && currentMap.IsWalkable(p) && currentMap.IsActorAt(p) && p != origin
			})
			return neighbors
		},
	}
}

func toList(blocking map[geometry.Point]int) []geometry.Point {
	list := make([]geometry.Point, 0)
	for pos := range blocking {
		list = append(list, pos)
	}
	return list
}
