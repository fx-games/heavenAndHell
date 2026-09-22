// Pathfinding, flood fills, nearest-X searches and rays over the map.

package gridmap

import (
	"math"
	"rpg/src/geometry"
	"rpg/src/util"
	"sort"
)

// NearestByPathCost floods the explored, walkable part of the map once from
// currentPosition and returns the candidate with the lowest path cost, or
// currentPosition if there is none. Straight-line distance is the wrong metric
// in a dungeon: the closest candidate as the crow flies is regularly the one
// behind a wall, three rooms away by foot.
func (m *GridMap[ActorType, ItemType, ObjectType]) NearestByPathCost(currentPosition geometry.Point, isCandidate func(p geometry.Point) bool) geometry.Point {
	// ponytail: full flood per call; cache it per turn if profiling ever cares.
	nearest, lowestCost := currentPosition, math.MaxInt32
	for loc, cost := range m.GetDijkstraMapForExploredWithActorsNotBlocking(currentPosition, 1000) {
		// Map iteration order is random, so ties need a deterministic winner:
		// without one the target flips between equidistant candidates.
		better := cost < lowestCost || (cost == lowestCost && lessByCoordinate(loc, nearest))
		if better && loc != currentPosition && isCandidate(loc) {
			nearest, lowestCost = loc, cost
		}
	}
	return nearest
}

// GetNearestUnexploredWalkablePosition returns the nearest frontier tile: an
// explored tile that borders the unknown. The unexplored tile itself is not a
// reachable target through known terrain, its explored neighbor is.
func (m *GridMap[ActorType, ItemType, ObjectType]) GetNearestUnexploredWalkablePosition(currentPosition geometry.Point) geometry.Point {
	return m.NearestByPathCost(currentPosition, func(p geometry.Point) bool {
		return len(m.GetFilteredNeighbors(p, func(n geometry.Point) bool {
			return m.IsTileWalkable(n) && !m.IsExplored(n)
		})) > 0
	})
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetNearestExploredItem(currentPosition geometry.Point, filter func(item ItemType) bool) geometry.Point {
	return m.NearestByPathCost(currentPosition, func(p geometry.Point) bool {
		return m.IsItemAt(p) && filter(m.itemAt(p))
	})
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetNearestExploredObject(currentPosition geometry.Point, filter func(object ObjectType) bool) geometry.Point {
	return m.NearestByPathCost(currentPosition, func(p geometry.Point) bool {
		return m.IsObjectAt(p) && filter(m.objectAt(p))
	})
}

func (m *GridMap[ActorType, ItemType, ObjectType]) WavePropagationFrom(pos geometry.Point, size int, pressure int) map[int][]geometry.Point {
	soundAnimationMap := make(map[int][]geometry.Point)
	m.pathfinder.DijkstraMap(m.getDijkstraMapperWithActorsNotBlocking(), []geometry.Point{pos}, size)
	for _, v := range m.pathfinder.DijkstraIterNodes {
		cost := v.Cost
		point := v.P
		if soundAnimationMap[cost] == nil {
			soundAnimationMap[cost] = make([]geometry.Point, 0)
		}
		soundAnimationMap[cost] = append(soundAnimationMap[cost], point)
	}
	return soundAnimationMap
}

func (m *GridMap[ActorType, ItemType, ObjectType]) getDijkstraMapperWithActorsNotBlocking() DijkstraMapper {
	return DijkstraMapper{
		neighbors: func(point geometry.Point) []geometry.Point {
			return m.GetFilteredNeighbors(point, func(p geometry.Point) bool {
				return m.Contains(p) && m.IsWalkable(p)
			})
		},
		cost: func(point geometry.Point, point2 geometry.Point) int {
			return int(geometry.Distance(point, point2) * 10)
		},
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) getDijkstraMapperForExploredWithActorsNotBlocking() DijkstraMapper {
	return DijkstraMapper{
		neighbors: func(point geometry.Point) []geometry.Point {
			return m.GetFilteredNeighbors(point, func(p geometry.Point) bool {
				return m.Contains(p) && m.IsTileWalkable(p) && m.IsExplored(p)
			})
		},
		cost: func(point geometry.Point, point2 geometry.Point) int {
			return int(geometry.Distance(point, point2) * 10)
		},
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) getDijkstraMapper(passable func(pos geometry.Point) bool) DijkstraMapper {
	return DijkstraMapper{
		neighbors: func(point geometry.Point) []geometry.Point {
			return m.GetFilteredNeighbors(point, func(p geometry.Point) bool {
				return m.Contains(p) && passable(p)
			})
		},
		cost: func(point geometry.Point, point2 geometry.Point) int {
			return int(geometry.Distance(point, point2) * 10)
		},
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetDijkstraMapWithActorsNotBlocking(start geometry.Point, maxCost int) map[geometry.Point]int {
	nodes := m.pathfinder.DijkstraMap(m.getDijkstraMapperWithActorsNotBlocking(), []geometry.Point{start}, maxCost*10)

	dijkstraMap := make(map[geometry.Point]int)
	for _, v := range nodes {
		dijkstraMap[v.P] = v.Cost
	}
	return dijkstraMap
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetDijkstraMapForExploredWithActorsNotBlocking(start geometry.Point, maxCost int) map[geometry.Point]int {
	nodes := m.pathfinder.DijkstraMap(m.getDijkstraMapperForExploredWithActorsNotBlocking(), []geometry.Point{start}, maxCost*10)

	dijkstraMap := make(map[geometry.Point]int)
	for _, v := range nodes {
		dijkstraMap[v.P] = v.Cost
	}
	return dijkstraMap
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetDijkstraMap(start geometry.Point, maxCost int, passable func(point geometry.Point) bool) map[geometry.Point]int {
	nodes := m.pathfinder.DijkstraMap(m.getDijkstraMapper(passable), []geometry.Point{start}, maxCost*10)

	dijkstraMap := make(map[geometry.Point]int)
	for _, v := range nodes {
		dijkstraMap[v.P] = v.Cost
	}
	return dijkstraMap
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetConnected(startLocation geometry.Point, traverse func(p geometry.Point) bool) []geometry.Point {
	results := make([]geometry.Point, 0)
	for _, node := range m.pathfinder.BreadthFirstMap(MapPather{neighborPredicate: traverse, allNeighbors: m.GetAllCardinalNeighbors}, []geometry.Point{startLocation}, 100) {
		results = append(results, node.P)
	}
	return results
}

// Reachable flood-fills the 8-connected cells from `from` that are on the map and pass `passable`.
func (m *GridMap[ActorType, ItemType, ObjectType]) Reachable(from geometry.Point, passable func(geometry.Point) bool) map[geometry.Point]bool {
	seen := map[geometry.Point]bool{from: true}
	for queue := []geometry.Point{from}; len(queue) > 0; queue = queue[1:] {
		for _, n := range m.GetFilteredNeighbors(queue[0], m.Contains) {
			if !seen[n] && passable(n) {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	return seen
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetJPSPath(start geometry.Point, end geometry.Point, isWalkable func(geometry.Point) bool) []geometry.Point {
	if !isWalkable(end) {
		end = m.getNearestFreeNeighbor(start, end, isWalkable)
	}
	//println(fmt.Sprintf("JPS from %v to %v", start, end))
	return m.pathfinder.JPSPath([]geometry.Point{}, start, end, isWalkable, !m.cardinalMovementOnly)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) getNearestFreeNeighbor(origin, pos geometry.Point, isFree func(geometry.Point) bool) geometry.Point {
	dist := math.MaxInt32
	nearest := pos
	for _, neighbor := range m.NeighborsCardinal(pos, isFree) {
		d := geometry.DistanceManhattan(origin, neighbor)
		if d < dist {
			dist = d
			nearest = neighbor
		}
	}
	return nearest
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetNearestSpecialTile(pos geometry.Point, tile SpecialTileType) geometry.Point {
	result := pos
	allPositions := m.GetAllSpecialTilePositions(tile)

	m.pathfinder.DijkstraMap(m.currentlyPassablePather(), []geometry.Point{pos}, 100)

	sort.Slice(allPositions, func(i, j int) bool {
		return m.pathfinder.DijkstraMapAt(allPositions[i]) < m.pathfinder.DijkstraMapAt(allPositions[j])
	})
	if len(allPositions) > 0 {
		result = allPositions[0]
	}
	return result
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetNearestTile(pos geometry.Point, filter func(tile Tile) bool) geometry.Point {
	result := pos

	allReachableExplored := m.GetDijkstraMapForExploredWithActorsNotBlocking(pos, 1000)
	minDist := math.MaxInt32

	for loc, dist := range allReachableExplored {
		if filter(m.GetCell(loc).TileType) {
			if dist < minDist {
				result = loc
				minDist = dist
			}
		}
	}
	return result
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetNearestActor(pos geometry.Point, filter func(actor ActorType) bool) ActorType {
	var nearestActor ActorType
	nearestDistance := math.MaxInt32
	for _, actor := range m.allActors {
		if filter(actor) {
			dist := geometry.DistanceSquared(pos, actor.MapPosition())
			if dist < nearestDistance {
				nearestDistance = dist
				nearestActor = actor
			}
		}
	}
	return nearestActor
}

func (m *GridMap[ActorType, ItemType, ObjectType]) currentlyPassablePather() MapPather {
	return MapPather{
		allNeighbors:      m.getCurrentlyPassableNeighbors,
		neighborPredicate: func(pos geometry.Point) bool { return true },
		pathCostFunc:      func(from geometry.Point, to geometry.Point) int { return 1 },
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetNearestDropOffPosition(pos geometry.Point) geometry.Point {
	nearestLocation := geometry.Point{X: 1, Y: 1}
	shortestDistance := math.MaxInt
	for index, c := range m.zoneMap {
		xPos := index % m.mapWidth
		yPos := index / m.mapWidth
		curPos := geometry.Point{X: xPos, Y: yPos}
		curDist := geometry.DistanceManhattan(curPos, pos)
		isItemHere := m.IsItemAt(curPos)
		isValidZone := c.IsDropOff()
		if !isItemHere && isValidZone && curDist < shortestDistance {
			nearestLocation = curPos
			shortestDistance = curDist
		}
	}
	return nearestLocation
}

func (m *GridMap[ActorType, ItemType, ObjectType]) FindNearestItem(pos geometry.Point, predicate func(item ItemType) bool) ItemType {
	var nearestItem ItemType
	nearestDistance := math.MaxInt
	for _, item := range m.allItems {
		if predicate(item) {
			curDist := geometry.DistanceManhattan(item.MapPosition(), pos)
			if curDist < nearestDistance {
				nearestItem = item
				nearestDistance = curDist
			}
		}
	}
	return nearestItem
}

func (m *GridMap[ActorType, ItemType, ObjectType]) FindAllNearbyActors(source geometry.Point, maxDist int, keep func(actor ActorType) bool) []ActorType {
	results := make([]ActorType, 0)
	for _, actor := range m.allActors {
		if keep(actor) && geometry.DistanceManhattan(actor.MapPosition(), source) <= maxDist {
			results = append(results, actor)
		}
	}
	return results
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetNearestWalkableNeighbor(start geometry.Point, dest geometry.Point) geometry.Point {
	minDist := math.MaxInt
	var minPos geometry.Point
	for _, neighbor := range m.NeighborsCardinal(dest, m.IsWalkable) {
		dist := geometry.DistanceManhattan(neighbor, start)
		if dist < minDist {
			minDist = dist
			minPos = neighbor
		}
	}
	return minPos
}

// BresenhamLine returns a list of points that are on the line between source and destination.
// NOTE: Will remove the source point from the list
func (m *GridMap[ActorType, ItemType, ObjectType]) BresenhamLine(source geometry.Point, destination geometry.Point, isBlocking func(mapPos geometry.Point) bool) []geometry.Point {
	los := geometry.LineOfSight(source, destination, func(p geometry.Point) bool {
		if !m.Contains(p) {
			return false
		}
		if p == source {
			return true
		}
		if isBlocking != nil && isBlocking(p) {
			return false
		}
		return m.IsWalkable(p)
	})
	withoutSource := los[1:]
	return withoutSource
}

func (m *GridMap[ActorType, ItemType, ObjectType]) RayCast(origin, direction geometry.PointF, isBlockingRay func(geometry.Point) bool) util.HitInfo2D {
	direction = direction.Normalize()
	hitInfo := util.Raycast2D(origin.X, origin.Y, direction.X, direction.Y, func(x, y int64) bool {
		currentMapCell := geometry.Point{X: int(x), Y: int(y)}
		return isBlockingRay(currentMapCell)
	})
	return hitInfo
}

func (m *GridMap[ActorType, ItemType, ObjectType]) ReflectingRayCast(origin, direction geometry.PointF, maxReflections int, isBlockingRay func(geometry.Point) bool) []util.HitInfo2D {
	direction = direction.Normalize()
	hitInfos := util.ReflectingRaycast2D(origin.X, origin.Y, direction.X, direction.Y, maxReflections, func(x, y int64) bool {
		currentMapCell := geometry.Point{X: int(x), Y: int(y)}
		return isBlockingRay(currentMapCell)
	})
	return hitInfos
}

func (m *GridMap[ActorType, ItemType, ObjectType]) JumpOverPositions(origin geometry.Point, target geometry.Point, maxSprint, maxJump int) JumpOverInfo {
	direction := target.Sub(origin)
	beyondTargetPos := target.Add(direction)
	lineOfSprint := m.BresenhamLine(origin, beyondTargetPos, func(pos geometry.Point) bool {
		return !m.Contains(pos)
	})

	var sprint []geometry.Point
	var jump []geometry.Point
	var jumpedPos geometry.Point
	actorFound := false

	for _, pos := range lineOfSprint {
		if m.IsCurrentlyPassable(pos) {
			if actorFound {
				jump = append(jump, pos)
				if len(jump) >= maxJump {
					break
				}
			} else {
				sprint = append(sprint, pos)
				if len(sprint) >= maxSprint {
					break
				}
			}
		} else if m.IsActorAt(pos) {
			jumpedPos = pos
			actorFound = true
		}
	}
	return JumpOverInfo{
		Sprint:     sprint,
		Jump:       jump,
		JumpedPos:  jumpedPos,
		ActorFound: actorFound,
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SprintToPosition(origin geometry.Point, target geometry.Point, maxSprint int) SprintTooInfo {
	lineOfSprint := m.BresenhamLine(origin, target, func(pos geometry.Point) bool { return !m.Contains(pos) })

	var sprint []geometry.Point
	var sprintEndPos geometry.Point
	var emptyTileFound bool

	for _, pos := range lineOfSprint {
		if !m.IsActorAt(pos) && m.IsCurrentlyPassable(pos) {
			sprint = append(sprint, pos)
			if len(sprint) >= maxSprint {
				break
			}
		} else {
			break
		}
	}

	if len(sprint) <= maxSprint && len(sprint) > 0 {
		emptyTileFound = true
		sprintEndPos = sprint[len(sprint)-1]
	}

	return SprintTooInfo{
		Sprint:         sprint,
		SprintEndPos:   sprintEndPos,
		EmptyTileFound: emptyTileFound,
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsValidSprintTarget(attacker geometry.Point, target geometry.Point, maxSprint int) bool {
	sprintInfo := m.SprintToPosition(attacker, target, maxSprint)
	isValid := false
	for _, pos := range sprintInfo.Sprint {
		if pos == target {
			isValid = true
			break
		}
	}
	return isValid
}
