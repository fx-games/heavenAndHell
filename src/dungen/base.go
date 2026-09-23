package dungen

import (
	"cmp"
	"fmt"
	"math"
	"math/rand"
	"rpg/src/geometry"
)

type DungeonTile int

const (
	Wall DungeonTile = iota
	Door
	Room
	Corridor
	Lake     // deep water: seen across, never walked (addLakes)
	Shallows // the water's edge, waded through
	Lava     // seen across, never walked, burns (the passes' lava)
	Pillar   // a lone column: blocks like a wall, stands on floor
)

type DungeonMap struct {
	width      int
	height     int
	tiles      []DungeonTile
	rooms      []*DungeonRoom
	pathfinder *geometry.PathRange
	loopDoors  map[geometry.Point]bool // doorways addMoreDoors opened: there is always another way round
	themeRooms [][4]int                // the theme rooms a pass walled in: x, y, w, h in grid units before scaling
	scale      int                     // cells per grid unit the passes drew with (0: not drawn by them)
	pieces     []Stamped               // the set pieces laid into it
}

// Pieces are the set pieces laid into the level, for the painter and the population.
func (m *DungeonMap) Pieces() []Stamped { return m.pieces }

// IsLoopDoor reports a door addMoreDoors placed: shutting it cuts no part of the level off.
func (m *DungeonMap) IsLoopDoor(p geometry.Point) bool {
	return m.loopDoors[p] && m.GetTileAt(p) == Door
}

func (m *DungeonMap) GetJPSPath(start geometry.Point, end geometry.Point) []geometry.Point {
	if !m.IsWalkable(end) || !m.IsWalkable(start) {
		return []geometry.Point{}
	}
	return m.pathfinder.JPSPath([]geometry.Point{}, start, end, m.IsWalkable, false)
}
func (m *DungeonMap) AddRoomAndSetTiles(room *DungeonRoom) {
	m.rooms = append(m.rooms, room)
	for _, tile := range room.GetAbsoluteFloorTiles() {
		m.SetRoom(tile.X, tile.Y)
	}
}

func (m *DungeonMap) ConnectRoomsOnMap(placeDoor bool) {
	for _, room := range m.rooms {
		for _, door := range room.GetUsedAbsoluteDoorTiles() {
			if m.GetTileAt(door) != Wall {
				continue
			}
			// Both rooms plugged into this connector, so it has to open whatever the walls around it look like:
			// irregular (cave) rooms rarely flank it with a straight wall pair, and skipping it cut them off.
			if _, couldBeDoor := m.CouldBeADoor(door); placeDoor && couldBeDoor {
				m.SetDoor(door.X, door.Y)
			} else {
				m.SetCorridor(door.X, door.Y)
			}
		}
	}
}

// openDoorways leaves each door a door with chance keep and makes the rest open doorways, row by row.
func (m *DungeonMap) openDoorways(rnd *rand.Rand, keep float64) {
	for y := 0; y < m.height; y++ {
		for x := 0; x < m.width; x++ {
			if m.GetTile(x, y) == Door && rnd.Float64() >= keep {
				m.SetCorridor(x, y)
			}
		}
	}
}

func (m *DungeonMap) SetWall(x, y int) {
	m.tiles[x+y*m.width] = Wall
}

func (m *DungeonMap) SetCorridor(x, y int) {
	m.tiles[x+y*m.width] = Corridor
}

func (m *DungeonMap) SetRoom(x, y int) {
	m.tiles[x+y*m.width] = Room
}

func (m *DungeonMap) GetTile(x int, y int) DungeonTile {
	return m.tiles[x+y*m.width]
}

func (m *DungeonMap) GetTileAt(pos geometry.Point) DungeonTile {
	return m.tiles[pos.X+pos.Y*m.width]
}

func (m *DungeonMap) GetPotentialStairPositions() []geometry.Point {
	// scan the map for the following patterns:
	// 1.
	// ###
	// #.#
	// ...
	// 2.
	// ###
	// ###
	// ...
	stairPatternOne := [3][3]DungeonTile{
		{Wall, Wall, Wall},
		{Wall, Room, Wall},
		{Room, Room, Room},
	}

	stairPatternTwo := [3][3]DungeonTile{
		{Wall, Wall, Wall},
		{Wall, Wall, Wall},
		{Room, Room, Room},
	}

	result := make([]geometry.Point, 0)

	for x := 0; x < m.width-2; x++ {
		for y := 0; y < m.height-2; y++ {
			matchPos := geometry.Point{X: x + 1, Y: y + 1}

			if !m.patternMatchRotated(x, y, stairPatternOne) && !m.patternMatchRotated(x, y, stairPatternTwo) {
				continue
			}

			roomsAround := m.GetAdjacentRooms(matchPos)

			isDecorated := func(room *DungeonRoom) bool {
				return room.HasDecoratedTemplate()
			}

			if exists(roomsAround, isDecorated) {
				continue
			}

			minDist := minDistToOther(matchPos, result)
			if minDist < 5 {
				continue
			}
			result = append(result, matchPos)
		}
	}

	return result
}

func exists(around []*DungeonRoom, f func(room *DungeonRoom) bool) bool {
	for _, room := range around {
		if f(room) {
			return true
		}
	}
	return false
}

func minDistToOther(pos geometry.Point, result []geometry.Point) int {
	minDist := math.MaxInt64
	for _, otherPos := range result {
		dist := pos.ManhattanDistanceTo(otherPos)
		if dist < minDist {
			minDist = dist
		}
	}
	return minDist
}

func rotate(matrix [3][3]DungeonTile, rotationCount int) [3][3]DungeonTile {
	// rotate matrix by 90 degrees

	if rotationCount == 0 {
		return matrix
	}

	for i, j := 0, len(matrix)-1; i < j; i, j = i+1, j-1 {
		matrix[i], matrix[j] = matrix[j], matrix[i]
	}

	// transpose it
	for i := 0; i < len(matrix); i++ {
		for j := 0; j < i; j++ {
			matrix[i][j], matrix[j][i] = matrix[j][i], matrix[i][j]
		}
	}

	return rotate(matrix, rotationCount-1)
}
func (m *DungeonMap) AllRooms() []*DungeonRoom {
	return m.rooms
}

func (m *DungeonMap) SetDoor(x int, y int) {
	m.tiles[x+y*m.width] = Door
}

// CanPlaceRoom says whether a room fits: its floor lands on solid wall, inside the map, and touches no floor
// already there, not even diagonally, so rooms stay walled off from each other except where they connect.
func (m *DungeonMap) CanPlaceRoom(room *DungeonRoom) bool {
	return m.CanPlaceRoomNextTo(room, nil)
}

// CanPlaceRoomNextTo is CanPlaceRoom for a room attaching to host, whose floor it may touch: some templates
// are drawn to sit flush in another (a quest room in the hole of a ring room).
func (m *DungeonMap) CanPlaceRoomNextTo(room, host *DungeonRoom) bool {
	nb := geometry.Neighbors{}
	for _, tile := range room.GetAbsoluteFloorTiles() {
		if !m.Contains(tile) || m.GetTileAt(tile) != Wall {
			return false
		}
		blocked := nb.All(tile, func(pos geometry.Point) bool {
			return !m.Contains(pos) || m.GetTileAt(pos) != Wall && (host == nil || !host.Contains(pos))
		})
		if len(blocked) > 0 {
			return false
		}
	}
	return true
}

func (m *DungeonMap) Contains(pos geometry.Point) bool {
	return pos.X >= 0 && pos.X < m.width && pos.Y >= 0 && pos.Y < m.height
}

func (m *DungeonMap) CouldBeADoor(pos geometry.Point) (geometry.CompassDirection, bool) {
	// needs to be a walltile, that has empty tiles on both sides in cardinal directions

	if m.GetTileAt(pos) != Wall {
		return geometry.East, false
	}

	// north/south
	if m.IsEmptySpace(pos.Add(geometry.North.ToPoint())) &&
		m.IsEmptySpace(pos.Add(geometry.South.ToPoint())) &&
		m.IsWallAt(pos.Add(geometry.East.ToPoint())) &&
		m.IsWallAt(pos.Add(geometry.West.ToPoint())) {
		return geometry.North, true

	}

	// east/west
	if m.IsEmptySpace(pos.Add(geometry.East.ToPoint())) &&
		m.IsEmptySpace(pos.Add(geometry.West.ToPoint())) &&
		m.IsWallAt(pos.Add(geometry.North.ToPoint())) &&
		m.IsWallAt(pos.Add(geometry.South.ToPoint())) {
		return geometry.East, true
	}

	return geometry.East, false
}

// addMoreDoors opens walls between two rooms that are far apart on foot, as doors or as bare openings.
func (m *DungeonMap) addMoreDoors(rnd *rand.Rand, minPathDistance int, spawnChance float64, asDoor bool) {
	m.TraverseTilesRandomly(rnd, func(mapPos geometry.Point) {
		if direction, ok := m.CouldBeADoor(mapPos); ok {
			if rnd.Float64() > spawnChance {
				return
			}
			posOne := mapPos.Add(direction.ToPoint())
			posTwo := mapPos.Add(direction.Opposite().ToPoint())
			path := m.GetJPSPath(posOne, posTwo)
			if len(path) == 0 || len(path) > minPathDistance {
				m.AddDoorAndConnect(mapPos, direction, asDoor)
				if len(path) > 0 { // a loop, not the only way through
					if m.loopDoors == nil {
						m.loopDoors = map[geometry.Point]bool{}
					}
					m.loopDoors[mapPos] = true
				}
			}
		}
	})
}
func (m *DungeonMap) IsEmptySpace(pos geometry.Point) bool {
	if !m.Contains(pos) {
		return false
	}
	tileAt := m.GetTileAt(pos)
	return tileAt == Room || tileAt == Corridor
}

func (m *DungeonMap) IsWalkable(pos geometry.Point) bool {
	if !m.Contains(pos) {
		return false
	}
	tileAt := m.GetTileAt(pos)
	return tileAt != Wall && tileAt != Lake && tileAt != Lava && tileAt != Pillar
}

func (m *DungeonMap) AddDoorAndConnect(absoluteDoorPos geometry.Point, direction geometry.CompassDirection, asDoor bool) {
	posOne := absoluteDoorPos.Add(direction.ToPoint())
	posTwo := absoluteDoorPos.Add(direction.Opposite().ToPoint())

	roomOne := m.GetRoomAt(posOne)
	roomTwo := m.GetRoomAt(posTwo)

	if roomOne == nil || roomTwo == nil {
		return
	}

	roomOne.AddConnectedRoom(absoluteDoorPos, roomTwo)
	roomTwo.AddConnectedRoom(absoluteDoorPos, roomOne)

	if asDoor {
		m.SetDoor(absoluteDoorPos.X, absoluteDoorPos.Y)
	} else {
		m.SetCorridor(absoluteDoorPos.X, absoluteDoorPos.Y)
	}
}

func (m *DungeonMap) GetRoomAt(posOne geometry.Point) *DungeonRoom {
	for _, room := range m.rooms {
		if room.Contains(posOne) {
			return room
		}
	}
	return nil
}

func (m *DungeonMap) TraverseTilesRandomly(random *rand.Rand, traversalFunc func(pos geometry.Point)) {
	randomIndices := random.Perm(m.width * m.height)
	for _, index := range randomIndices {
		x := index % m.width
		y := index / m.width
		traversalFunc(geometry.Point{X: x, Y: y})
	}
}

func (m *DungeonMap) FillDeadEnds(random *rand.Rand) {
	deadEnds := make([]geometry.Point, 0)
	for y := 0; y < m.height; y++ {
		for x := 0; x < m.width; x++ {
			pos := geometry.Point{X: x, Y: y}
			_, isDeadEnd := m.IsDeadEnd(pos)
			if isDeadEnd {
				deadEnds = append(deadEnds, pos)
			}
		}
	}

	for _, pos := range deadEnds {
		for direction, isDeadEnd := m.IsDeadEnd(pos); isDeadEnd; direction, isDeadEnd = m.IsDeadEnd(pos) {
			m.SetWall(pos.X, pos.Y)
			pos = pos.Add(direction.ToPoint())
		}
	}
}

// IsDeadEnd reports floor with wall on three sides, and the side it opens to. A door counts as floor: when the
// corridor behind a door is filled in, the door goes too, rather than stand in the room's wall facing rock.
func (m *DungeonMap) IsDeadEnd(pos geometry.Point) (geometry.CompassDirection, bool) {
	passable := func(p geometry.Point) bool { return m.IsEmptySpace(p) || m.Contains(p) && m.GetTileAt(p) == Door }
	if !passable(pos) {
		return 0, false
	}

	nb := geometry.Neighbors{}
	neighoringWalls := nb.Cardinal(pos, func(pos geometry.Point) bool {
		return m.Contains(pos) && m.GetTileAt(pos) == Wall
	})

	var openDirection geometry.CompassDirection
	cardinalDirs := []geometry.CompassDirection{geometry.North, geometry.South, geometry.East, geometry.West}

	for _, dir := range cardinalDirs {
		if passable(pos.Add(dir.ToPoint())) {
			openDirection = dir
			break
		}
	}

	return openDirection, len(neighoringWalls) == 3
}

func (m *DungeonMap) patternMatch(x int, y int, patternOne [3][3]DungeonTile) bool {
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if m.GetTile(x+i, y+j) != patternOne[i][j] {
				return false
			}
		}
	}
	return true
}

func (m *DungeonMap) patternMatchRotated(x int, y int, patternOne [3][3]DungeonTile) bool {
	for i := 0; i < 4; i++ {
		rotatedPattern := rotate(patternOne, i)
		if m.patternMatch(x, y, rotatedPattern) {
			return true
		}
	}
	return false
}
func (m *DungeonMap) Trim() {
	// remove rows and columns that are only walls are adjacent to a row or column that is only walls
	firstNonWallY := 0

	rowIsAllWalls := func(yCoord int) bool {
		for x := 0; x < m.width; x++ {
			if m.GetTile(x, yCoord) != Wall {
				return false
			}
		}
		return true
	}
	// from top to bottom
	for y := 0; y < m.height; y++ {
		if !rowIsAllWalls(y) {
			firstNonWallY = y
			break
		}
	}

	lastNonWallY := m.height - 1
	for y := m.height - 1; y >= 0; y-- {
		if !rowIsAllWalls(y) {
			lastNonWallY = y
			break
		}
	}

	firstNonWallX := 0
	columnIsAllWalls := func(xCoord int) bool {
		for y := 0; y < m.height; y++ {
			if m.GetTile(xCoord, y) != Wall {
				return false
			}
		}
		return true
	}
	// from left to right
	for x := 0; x < m.width; x++ {
		if !columnIsAllWalls(x) {
			firstNonWallX = x
			break
		}
	}

	lastNonWallX := m.width - 1
	for x := m.width - 1; x >= 0; x-- {
		if !columnIsAllWalls(x) {
			lastNonWallX = x
			break
		}
	}
	lastXi := m.width - 1
	lastYi := m.height - 1
	trimLeft := max(0, firstNonWallX-1)
	trimRight := max(0, lastXi-lastNonWallX-1)
	trimTop := max(0, firstNonWallY-1)
	trimBottom := max(0, lastYi-lastNonWallY-1)

	if trimLeft == 0 && trimRight == 0 && trimTop == 0 && trimBottom == 0 {
		return
	}

	newWidth := m.width - trimLeft - trimRight
	newHeight := m.height - trimTop - trimBottom

	newTiles := make([]DungeonTile, newWidth*newHeight)
	for x := 0; x < newWidth; x++ {
		for y := 0; y < newHeight; y++ {
			newTiles[x+y*newWidth] = m.GetTile(x+trimLeft, y+trimTop)
		}
	}
	m.width = newWidth
	m.height = newHeight
	m.tiles = newTiles
	for _, room := range m.rooms {
		room.SetPositionOffset(room.GetPositionOffset().Sub(geometry.Point{X: trimLeft, Y: trimTop}))
	}

}

func (m *DungeonMap) GetSize() (int, int) {
	return m.width, m.height
}

func (m *DungeonMap) IsWallAt(pos geometry.Point) bool {
	if !m.Contains(pos) {
		return false
	}
	return m.GetTileAt(pos) == Wall
}

func (m *DungeonMap) GetMetaInfo() []string {
	walkableTiles := m.GetWalkableTileCount()
	wallTiles := m.GetWallTileCount()
	totalTileCount := m.width * m.height
	smallestRoom, biggestRoom := m.GetBiggestAndSmallestRoom()
	info := []string{
		fmt.Sprintf("Dimensions: %d x %d", m.width, m.height),
		fmt.Sprintf("Total tiles: %d", totalTileCount),
		fmt.Sprintf("Walkable tiles: %d (%.2f%%)", walkableTiles, float64(walkableTiles)/float64(totalTileCount)*100),
		fmt.Sprintf("Wall tiles: %d (%.2f%%)", wallTiles, float64(wallTiles)/float64(totalTileCount)*100),
		fmt.Sprintf("Rooms: %d", len(m.rooms)),
	}
	if len(m.rooms) > 0 { // the passes leave no rooms
		info = append(info, fmt.Sprintf("Smallest room: %d tiles", smallestRoom.GetFloorTileCount()),
			fmt.Sprintf("Biggest room: %d tiles", biggestRoom.GetFloorTileCount()))
	}

	return info
}

func (m *DungeonMap) GetBiggestAndSmallestRoom() (*DungeonRoom, *DungeonRoom) {
	var biggestRoom *DungeonRoom
	var smallestRoom *DungeonRoom
	for _, room := range m.rooms {
		if biggestRoom == nil || room.GetFloorTileCount() > biggestRoom.GetFloorTileCount() {
			biggestRoom = room
		}
		if smallestRoom == nil || room.GetFloorTileCount() < smallestRoom.GetFloorTileCount() {
			smallestRoom = room
		}
	}
	return smallestRoom, biggestRoom
}

func (m *DungeonMap) GetWalkableTileCount() int {
	count := 0
	for _, tile := range m.tiles {
		if tile == Room || tile == Corridor {
			count++
		}
	}
	return count
}

func (m *DungeonMap) GetWallTileCount() int {
	count := 0
	for _, tile := range m.tiles {
		if tile == Wall {
			count++
		}
	}
	return count
}

func (m *DungeonMap) GetAdjacentRooms(pos geometry.Point) []*DungeonRoom {
	neighbors := geometry.Neighbors{}
	emptyNeighbors := neighbors.All(pos, func(pos geometry.Point) bool {
		return m.Contains(pos) && m.IsEmptySpace(pos)
	})
	var adjacentRooms []*DungeonRoom
	for _, neighbor := range emptyNeighbors {
		roomAt := m.GetRoomAt(neighbor)
		if roomAt == nil {
			continue
		}
		adjacentRooms = append(adjacentRooms, roomAt)
	}
	return adjacentRooms
}

func (m *DungeonMap) GetCardinalNeighbours(pos geometry.Point) []geometry.Point {
	neighbors := geometry.Neighbors{}
	return neighbors.Cardinal(pos, func(pos geometry.Point) bool {
		return m.Contains(pos)
	})
}

func (m *DungeonMap) GetFilteredCardinalNeighbours(pos geometry.Point, filter func(pos geometry.Point) bool) []geometry.Point {
	neighbors := geometry.Neighbors{}
	return neighbors.Cardinal(pos, func(pos geometry.Point) bool {
		return m.Contains(pos) && filter(pos)
	})
}
func (m *DungeonMap) GetAllNeighbours(pos geometry.Point) []geometry.Point {
	neighbors := geometry.Neighbors{}
	return neighbors.All(pos, func(pos geometry.Point) bool {
		return m.Contains(pos)
	})
}

func (m *DungeonMap) GetAllFilteredNeighbours(pos geometry.Point, filter func(pos geometry.Point) bool) []geometry.Point {
	neighbors := geometry.Neighbors{}
	return neighbors.All(pos, func(pos geometry.Point) bool {
		return m.Contains(pos) && filter(pos)
	})
}

func (m *DungeonMap) GetRandomRoom(rnd *rand.Rand) *DungeonRoom {
	return m.rooms[rnd.Intn(len(m.rooms))]
}

func (m *DungeonMap) AddBorder() {
	currentTiles := m.tiles
	newTiles := make([]DungeonTile, (m.width+2)*(m.height+2))
	for x := 0; x < m.width+2; x++ {
		for y := 0; y < m.height+2; y++ {
			if x == 0 || y == 0 || x == m.width+1 || y == m.height+1 {
				newTiles[x+y*(m.width+2)] = Wall
				continue
			}
			newTiles[x+y*(m.width+2)] = currentTiles[x-1+(y-1)*m.width]
		}
	}
	m.width += 2
	m.height += 2
	m.tiles = newTiles

	for _, room := range m.rooms {
		room.SetPositionOffset(geometry.Point{X: 1, Y: 1})
	}
}

func NewDungeonMap(width, height int) *DungeonMap {
	return &DungeonMap{
		width:      width,
		height:     height,
		tiles:      make([]DungeonTile, width*height),
		rooms:      make([]*DungeonRoom, 0),
		pathfinder: geometry.NewPathRange(geometry.NewRect(0, 0, width, height)),
	}
}

// addRectRoom lays a plain rectangular room on floor, as rogue.go does; its doors are the corridors dug later.
func addRectRoom(m *DungeonMap, rnd *rand.Rand, floor geometry.Rect) *DungeonRoom {
	room := NewDungeonRoomFromRect(rnd, floor)
	room.SetConnections(map[geometry.Point]RoomConnection{})
	room.SetId(len(m.rooms))
	m.AddRoomAndSetTiles(room)
	return room
}

func center(r *DungeonRoom) geometry.Point {
	return r.bounds.Center()
}

// carveL digs an L-shaped corridor from a to b through rock, turning at a random end.
func carveL(m *DungeonMap, rnd *rand.Rand, a, b geometry.Point) {
	corner := geometry.Point{X: b.X, Y: a.Y}
	if rnd.Intn(2) == 0 {
		corner = geometry.Point{X: a.X, Y: b.Y}
	}
	for _, leg := range [][2]geometry.Point{{a, corner}, {corner, b}} {
		p, to := leg[0], leg[1]
		d := geometry.Point{X: sign(to.X - p.X), Y: sign(to.Y - p.Y)}
		for ; ; p = p.Add(d) {
			if m.GetTileAt(p) == Wall {
				m.SetCorridor(p.X, p.Y)
			}
			if p == to {
				break
			}
		}
	}
}

func sign(v int) int { return cmp.Compare(v, 0) }
