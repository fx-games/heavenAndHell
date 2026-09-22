package dungen

import (
	"math"
	"math/rand"
	"rpg/src/geometry"
	"slices"
)

type DungeonRoom struct {
	roomPositionOffset geometry.Point

	floorTiles map[geometry.Point]bool

	availableConnections    map[geometry.Point]RoomConnection
	connectedRooms          map[geometry.Point]*DungeonRoom
	rotationMatrix          [2][2]int
	inverseRotationMatrix   [2][2]int
	rotationOffset          geometry.CompassDirection
	mirrored                bool
	usedTemplate            string
	roomId                  int
	wallTiles               []geometry.Point
	poiPositions            []geometry.Point
	usedDecorationLevelName string
	slotId                  int
	plugsIntoSlotId         int
	canBeLit                bool
	bounds                  geometry.Rect
}

func (r *DungeonRoom) GetFreeConnectionsWithRotatedDirection() map[geometry.Point]RoomConnection {
	result := make(map[geometry.Point]RoomConnection)
	for _, point := range getSortedKeys(r.availableConnections) {
		roomConnection := r.availableConnections[point]
		if _, ok := r.connectedRooms[point]; !ok {
			result[point] = RoomConnection{
				OutwardDirection: r.transformDirection(roomConnection.OutwardDirection),
				ConnectionID:     roomConnection.ConnectionID,
			}
		}
	}
	return result
}

// transformDirection turns a template's connector direction into the placed room's, always within [0, 360):
// an unwrapped sum (North 270 + a 270 rotation = 540) has an Opposite that no wrapped direction ever equals.
func (r *DungeonRoom) transformDirection(d geometry.CompassDirection) geometry.CompassDirection {
	if r.mirrored { // East and West swap, North and South stay
		d = 180 - d + 360
	}
	d += r.rotationOffset
	for d >= 360 {
		d -= 360
	}
	return d
}

func (r *DungeonRoom) HasMatchingConnection(otherConnection RoomConnection) (geometry.Point, bool) {
	neededDirection := otherConnection.OutwardDirection.Opposite()
	neededId := otherConnection.ConnectionID
	for _, pos := range getSortedKeys(r.availableConnections) {
		if _, alreadyConnected := r.connectedRooms[pos]; alreadyConnected {
			continue
		}
		roomConnection := r.availableConnections[pos]

		if r.transformDirection(roomConnection.OutwardDirection) == neededDirection && roomConnection.ConnectionID == neededId {
			return pos, true
		}
	}
	return geometry.Point{}, false
}

func (r *DungeonRoom) SetPositionOffset(offset geometry.Point) {
	r.roomPositionOffset = offset
}

func (r *DungeonRoom) GetAbsoluteDoorPosition(doorPos geometry.Point) geometry.Point {
	return r.ToAbsolutePosition(doorPos)
}

func (r *DungeonRoom) ToAbsolutePosition(relativePos geometry.Point) geometry.Point {
	rotMat := r.rotationMatrix
	rotatedPos := matrixMul(relativePos, rotMat)
	return r.roomPositionOffset.Add(rotatedPos)
}

func (r *DungeonRoom) ToRelativePosition(absolutePos geometry.Point) geometry.Point {
	translatedPos := absolutePos.Sub(r.roomPositionOffset)
	rotMat := r.inverseRotationMatrix
	unrotatedPos := matrixMul(translatedPos, rotMat)
	return unrotatedPos
}

func (r *DungeonRoom) GetAbsoluteFloorTiles() []geometry.Point {
	result := make([]geometry.Point, 0)
	for point := range r.floorTiles {
		result = append(result, r.ToAbsolutePosition(point))
	}
	return result
}
func (r *DungeonRoom) GetRelativeFloorTiles() []geometry.Point {
	result := make([]geometry.Point, 0)
	for point := range r.floorTiles {
		result = append(result, point)
	}
	slices.SortFunc(result, comparePoints)
	return result
}
func (r *DungeonRoom) GetAbsoluteRoomTiles() []geometry.Point {
	result := make([]geometry.Point, 0)
	for point := range r.floorTiles {
		result = append(result, r.ToAbsolutePosition(point))
	}
	for _, point := range r.wallTiles {
		result = append(result, r.ToAbsolutePosition(point))
	}
	return result
}

func (r *DungeonRoom) GetPOIPositions() []geometry.Point {
	result := make([]geometry.Point, 0)
	for _, point := range r.poiPositions {
		result = append(result, r.ToAbsolutePosition(point))
	}
	return result
}
func (r *DungeonRoom) AddConnectedRoom(doorPos geometry.Point, room *DungeonRoom) {
	r.connectedRooms[doorPos] = room
}

func (r *DungeonRoom) GetUsedAbsoluteDoorTiles() []geometry.Point {
	result := make([]geometry.Point, 0)
	for point := range r.availableConnections {
		if _, ok := r.connectedRooms[point]; !ok {
			continue
		}
		result = append(result, r.ToAbsolutePosition(point))
	}
	return result
}

func (r *DungeonRoom) Contains(absolutePosition geometry.Point) bool {
	relative := r.ToRelativePosition(absolutePosition)
	if _, ok := r.floorTiles[relative]; ok {
		return true
	}
	return false
}

func (r *DungeonRoom) SetRotationCount(count int) {
	r.SetTransform(count, false)
}

// SetTransform turns the room by count quarter turns clockwise, after flipping it left to right if mirrored.
func (r *DungeonRoom) SetTransform(count int, mirrored bool) {
	rotations := [4][2][2]int{
		{{1, 0}, {0, 1}},
		{{0, -1}, {1, 0}},
		{{-1, 0}, {0, -1}},
		{{0, 1}, {-1, 0}},
	}
	rot, inv := rotations[count&3], rotations[(4-count)&3]
	if mirrored {
		// flip first: rot*F negates rot's first column, F*inv negates inv's first row
		rot = [2][2]int{{-rot[0][0], rot[0][1]}, {-rot[1][0], rot[1][1]}}
		inv = [2][2]int{{-inv[0][0], -inv[0][1]}, {inv[1][0], inv[1][1]}}
	}
	r.rotationMatrix, r.inverseRotationMatrix = rot, inv
	r.rotationOffset = geometry.CompassDirection(90 * (count & 3))
	r.mirrored = mirrored
}

func NewDungeonRoomFromRect(random *rand.Rand, bounds geometry.Rect) *DungeonRoom {
	rectRoom := &DungeonRoom{
		floorTiles:     roomTilesFromRect(bounds),
		bounds:         bounds,
		connectedRooms: make(map[geometry.Point]*DungeonRoom),
		availableConnections: map[geometry.Point]RoomConnection{
			bounds.GetRandomPointOnEdge(random, geometry.North).Add(geometry.North.ToPoint()): {
				OutwardDirection: geometry.North,
				ConnectionID:     0,
			},
			bounds.GetRandomPointOnEdge(random, geometry.South).Add(geometry.South.ToPoint()): {
				OutwardDirection: geometry.South,
				ConnectionID:     0,
			},
			bounds.GetRandomPointOnEdge(random, geometry.East).Add(geometry.East.ToPoint()): {
				OutwardDirection: geometry.East,
				ConnectionID:     0,
			},
			bounds.GetRandomPointOnEdge(random, geometry.West).Add(geometry.West.ToPoint()): {
				OutwardDirection: geometry.West,
				ConnectionID:     0,
			},
		},
	}
	rectRoom.SetRotationCount(0)
	return rectRoom
}
func (r *DungeonRoom) SetFloorTiles(tiles []geometry.Point) {
	r.floorTiles = make(map[geometry.Point]bool)
	for _, tile := range tiles {
		r.floorTiles[tile] = true
	}
}

func (r *DungeonRoom) SetConnections(tiles map[geometry.Point]RoomConnection) {
	r.availableConnections = tiles
}

func (r *DungeonRoom) Clone() *DungeonRoom {
	newRoom := &DungeonRoom{
		roomPositionOffset:      r.roomPositionOffset,
		connectedRooms:          make(map[geometry.Point]*DungeonRoom),
		rotationMatrix:          r.rotationMatrix,
		inverseRotationMatrix:   r.inverseRotationMatrix,
		rotationOffset:          r.rotationOffset,
		mirrored:                r.mirrored,
		usedTemplate:            r.usedTemplate,
		usedDecorationLevelName: r.usedDecorationLevelName,
		wallTiles:               make([]geometry.Point, 0),
		floorTiles:              make(map[geometry.Point]bool),
		availableConnections:    make(map[geometry.Point]RoomConnection),
		poiPositions:            make([]geometry.Point, 0),
		slotId:                  r.slotId,
		plugsIntoSlotId:         r.plugsIntoSlotId,
		canBeLit:                r.canBeLit,
	}
	for point := range r.floorTiles {
		newRoom.floorTiles[point] = true
	}
	newRoom.wallTiles = append(newRoom.wallTiles, r.wallTiles...)
	for point, direction := range r.availableConnections {
		newRoom.availableConnections[point] = direction
	}
	newRoom.poiPositions = append(newRoom.poiPositions, r.poiPositions...)
	return newRoom
}

func (r *DungeonRoom) PrintUntransformed() {
	bounds := r.GetRelativeBoundingRectIncludingConnectors()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			relativeRoomPos := geometry.Point{X: x, Y: y}
			if _, ok := r.floorTiles[relativeRoomPos]; ok {
				print(".")
			} else if _, ok = r.availableConnections[relativeRoomPos]; ok {
				print("+")
			} else {
				print(" ")
			}
		}
		println()
	}
}

func (r *DungeonRoom) GetRelativeBoundingRectIncludingConnectors() geometry.Rect {
	minX := math.MaxInt
	minY := math.MaxInt
	maxX := -math.MaxInt
	maxY := -math.MaxInt
	for point := range r.floorTiles {
		if point.X < minX {
			minX = point.X
		}
		if point.Y < minY {
			minY = point.Y
		}
		if point.X > maxX {
			maxX = point.X
		}
		if point.Y > maxY {
			maxY = point.Y
		}
	}
	for point := range r.availableConnections {
		if point.X < minX {
			minX = point.X
		}
		if point.Y < minY {
			minY = point.Y
		}
		if point.X > maxX {
			maxX = point.X
		}
		if point.Y > maxY {
			maxY = point.Y
		}
	}
	return geometry.NewRect(minX, minY, maxX+1, maxY+1)
}

func (r *DungeonRoom) PrintRotated() {
	bounds := r.GetRelativeBoundingRectIncludingConnectors()
	transformedBoundsMin := matrixMul(bounds.Min, r.rotationMatrix)
	transformedBoundsMax := matrixMul(bounds.Max, r.rotationMatrix)
	transformedBounds := geometry.NewRect(transformedBoundsMin.X, transformedBoundsMin.Y, transformedBoundsMax.X, transformedBoundsMax.Y)
	transformedBounds = transformedBounds.Shift(0, 0, 1, 1)
	zero := geometry.Point{X: 0, Y: 0}
	for y := transformedBounds.Min.Y; y < transformedBounds.Max.Y; y++ {
		for x := transformedBounds.Min.X; x < transformedBounds.Max.X; x++ {
			absTransformedPos := geometry.Point{X: x, Y: y}
			relativePos := matrixMul(absTransformedPos, r.inverseRotationMatrix)
			if relativePos == zero {
				print("0")
			} else if _, ok := r.floorTiles[relativePos]; ok {
				print(".")
			} else if _, ok = r.availableConnections[relativePos]; ok {
				print("+")
			} else if _, ok = r.connectedRooms[relativePos]; ok {
				print("=")
			} else {
				print(" ")
			}
		}
		println()
	}
}
func (r *DungeonRoom) PrintTransformed() {
	bounds := r.GetRelativeBoundingRectIncludingConnectors()
	transformedBoundsMin := r.ToAbsolutePosition(bounds.Min)
	transformedBoundsMax := r.ToAbsolutePosition(bounds.Max)
	transformedBounds := geometry.NewRect(transformedBoundsMin.X, transformedBoundsMin.Y, transformedBoundsMax.X, transformedBoundsMax.Y)
	transformedBounds = transformedBounds.Shift(0, 0, 1, 1)
	zero := geometry.Point{X: 0, Y: 0}
	for y := transformedBounds.Min.Y; y < transformedBounds.Max.Y; y++ {
		xStart := min(0, transformedBounds.Min.X)
		for x := xStart; x < transformedBounds.Max.X; x++ {
			absolutePos := geometry.Point{X: x, Y: y}
			relativePos := r.ToRelativePosition(absolutePos)
			if absolutePos == zero {
				print("0")
			} else if relativePos == zero {
				print("Q")
			} else if _, ok := r.floorTiles[relativePos]; ok {
				print(".")
			} else if _, ok = r.availableConnections[relativePos]; ok {
				print("+")
			} else if _, ok = r.connectedRooms[relativePos]; ok {
				print("=")
			} else {
				print(" ")
			}
		}
		println()
	}
}

func (r *DungeonRoom) PrintTransformedWithHighlight(relativePositionForHighlight geometry.Point) {
	bounds := r.GetRelativeBoundingRectIncludingConnectors()
	transformedBoundsMin := r.ToAbsolutePosition(bounds.Min)
	transformedBoundsMax := r.ToAbsolutePosition(bounds.Max)
	transformedBounds := geometry.NewRect(transformedBoundsMin.X, transformedBoundsMin.Y, transformedBoundsMax.X, transformedBoundsMax.Y)
	transformedBounds = transformedBounds.Shift(0, 0, 1, 1)
	zero := geometry.Point{X: 0, Y: 0}
	for y := transformedBounds.Min.Y; y < transformedBounds.Max.Y; y++ {
		xStart := min(0, transformedBounds.Min.X)
		for x := xStart; x < transformedBounds.Max.X; x++ {
			absolutePos := geometry.Point{X: x, Y: y}
			relativePos := r.ToRelativePosition(absolutePos)
			if relativePos == relativePositionForHighlight {
				print("X")
			} else if absolutePos == zero {
				print("0")
			} else if relativePos == zero {
				print("Q")
			} else if _, ok := r.floorTiles[relativePos]; ok {
				print(".")
			} else if _, ok = r.availableConnections[relativePos]; ok {
				print("+")
			} else if _, ok = r.connectedRooms[relativePos]; ok {
				print("=")
			} else {
				print(" ")
			}
		}
		println()
	}
}

func (r *DungeonRoom) SetId(id int) {
	r.roomId = id
}

func (r *DungeonRoom) GetRotatedRelativeDoorPosition(pos geometry.Point) geometry.Point {
	return matrixMul(pos, r.rotationMatrix)
}

func (r *DungeonRoom) GetPositionOffset() geometry.Point {
	return r.roomPositionOffset
}

func (r *DungeonRoom) SetIncludedWallTiles(tiles []geometry.Point) {
	r.wallTiles = tiles
}

func (r *DungeonRoom) SetPOIPositions(points []geometry.Point) {
	r.poiPositions = points
}

func (r *DungeonRoom) HasDecoratedTemplate() bool {
	return r.usedTemplate != "" && r.usedDecorationLevelName != ""
}

func (r *DungeonRoom) GetTemplateName() string {
	return r.usedTemplate
}

func (r *DungeonRoom) SetDecorationLevelName(levelName string) {
	r.usedDecorationLevelName = levelName
}

func (r *DungeonRoom) GetDecorationLevelName() string {
	return r.usedDecorationLevelName
}

func (r *DungeonRoom) HasOnlyZeroIDConnections() bool {
	for _, connection := range r.availableConnections {
		if connection.ConnectionID != 0 {
			return false
		}
	}
	return true
}

func (r *DungeonRoom) SetSlotId(id int) {
	r.slotId = id
}
func (r *DungeonRoom) GetSlotId() int {
	return r.slotId
}
func (r *DungeonRoom) SetPlugsIntoSlotId(plugsIn int) {
	r.plugsIntoSlotId = plugsIn
}
func (r *DungeonRoom) GetPlugsIntoSlotId() int {
	return r.plugsIntoSlotId
}

func (r *DungeonRoom) GetFloorTileCount() int {
	return len(r.floorTiles)
}

func (r *DungeonRoom) SetCanBeLit(lit bool) {
	r.canBeLit = lit
}

func (r *DungeonRoom) CanBeLit() bool {
	return r.canBeLit
}

func (r *DungeonRoom) IsCornerPosition(posOne geometry.Point) bool {
	shifted := posOne.Sub(r.roomPositionOffset)
	return r.bounds.IsOnCorner(shifted)
}
func NewDungeonRoomFromTemplate(templateName string) *DungeonRoom {
	rectRoom := &DungeonRoom{
		usedTemplate:   templateName,
		connectedRooms: make(map[geometry.Point]*DungeonRoom),
	}
	rectRoom.SetRotationCount(0)
	return rectRoom
}

func roomTilesFromRect(bounds geometry.Rect) map[geometry.Point]bool {
	result := make(map[geometry.Point]bool)
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			result[geometry.Point{X: x, Y: y}] = true
		}
	}
	return result
}
func matrixMul(point geometry.Point, b [2][2]int) geometry.Point {
	return geometry.Point{
		X: point.X*b[0][0] + point.Y*b[0][1],
		Y: point.X*b[1][0] + point.Y*b[1][1],
	}
}
