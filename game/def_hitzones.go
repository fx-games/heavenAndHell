package game

import (
	"math"
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/util"
)


func (g *GameInstance) GetAllHitZones() map[string]func(args util.Arguments)common.AoEResolver {
	return map[string]func(args util.Arguments)common.AoEResolver{
		"target_only": func(args util.Arguments) common.AoEResolver {
			return OneTargetTile
		},
		"ray_aoe": func(args util.Arguments) common.AoEResolver {
			return RayAoE(g.state.GetMap(), args.GetInt(0))
		},
		"bresenham_aoe": func(args util.Arguments) common.AoEResolver {
			return BresenhamAoE(g.state.GetMap(), args.GetInt(0))
		},
		"dijkstra_aoe": func(args util.Arguments) common.AoEResolver {
			return DijkstraAoE(g.state.GetMap(), args.GetInt(0))
		},
		"wall_aoe": func(args util.Arguments) common.AoEResolver {
			return WallAoE(g.state.GetMap(), args.GetInt(0))
		},
	}
}
// These determine aoe zones for spells and throwable items
func (g *GameInstance) NewHitZoneFromString(zone string) common.AoEResolver {
	allHitZones := g.GetAllHitZones()
	if util.LooksLikeAFunction(zone) {
		parametrizedHitZone, args := util.GetNameAndArgs(zone)
		if resolver, ok := allHitZones[parametrizedHitZone]; ok {
			return resolver(args)
		}
	}
	return OneTargetTile
}

func DijkstraAoE(currentMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], maxDistance int) func(origin, target geometry.Point) []geometry.Point {
	return func(origin, target geometry.Point) []geometry.Point {
		return toList(currentMap.GetDijkstraMapWithActorsNotBlocking(target, maxDistance))
	}
}

func RayAoE(currentMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], maxDistance int) func(originGrid, targetGrid geometry.Point) []geometry.Point {
	return func(originGrid, targetGrid geometry.Point) []geometry.Point {
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
	}
}

func BresenhamAoE(currentMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], maxDistance int) func(originGrid, targetGrid geometry.Point) []geometry.Point {
	return func(originGrid, targetGrid geometry.Point) []geometry.Point {
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
	}
}

func WallAoE(currentMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], wallParts int) func(origin, target geometry.Point) []geometry.Point {
	return func(origin, target geometry.Point) []geometry.Point {
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
	}
}
