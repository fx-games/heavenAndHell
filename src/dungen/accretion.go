package dungen

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"math/rand"
	"rpg/src/geometry"
	"rpg/src/ldtk"
	"slices"
	"strings"
)

type TemplateRules struct {
	RequireTemplates map[string]bool
	ForbidTemplates  map[string]bool
	TemplatePrefix   string
	NoDoors          bool
}

type RoomConnection struct {
	OutwardDirection geometry.CompassDirection
	ConnectionID     int
}
type AccretionGenerator struct {
	random          *rand.Rand
	roomTemplates   []*DungeonRoom
	maxRoomCount    int
	printDebugSteps bool
	currentRoomId   int
	levelRules      TemplateRules
	placedTemplates map[string]bool
	mirror          bool
	loopDistance    int
	loopChance      float64
}

func NewAccretionGenerator(randomSource *rand.Rand) *AccretionGenerator {
	return &AccretionGenerator{
		random:        randomSource,
		currentRoomId: 0,
		loopDistance:  10,
		loopChance:    0.8,
	}
}

func (g *AccretionGenerator) getTemplatesForLevel() []*DungeonRoom {
	var templates []*DungeonRoom
	for _, room := range g.roomTemplates {
		if !g.levelRules.ForbidTemplates[room.GetTemplateName()] {
			templates = append(templates, room)
		}
	}
	return templates
}

func (g *AccretionGenerator) getNewRoomFromRandomTemplateForSlot(slotID int) (*DungeonRoom, error) {
	var matchingRooms []*DungeonRoom

	missingTemplates := make(map[string]bool)
	for reqRoom := range g.levelRules.RequireTemplates {
		if _, ok := g.placedTemplates[reqRoom]; !ok {
			missingTemplates[reqRoom] = true
		}
	}

	if len(missingTemplates) > 0 { // force one of the missing templates
		missingSlotIds := make(map[int]bool)
		for _, room := range g.getTemplatesForLevel() {
			if _, ok := missingTemplates[room.GetTemplateName()]; !ok {
				continue
			}
			if room.GetPlugsIntoSlotId() == slotID {
				matchingRooms = append(matchingRooms, room)
			} else {
				missingSlotIds[room.GetPlugsIntoSlotId()] = true
			}
		}
		if len(matchingRooms) == 0 {
			// probably a missing template is not available for this slot
			// see what slot we need and try to find a room that provides it
			for _, room := range g.getTemplatesForLevel() {
				if _, ok := missingSlotIds[room.GetSlotId()]; !ok {
					continue
				}
				if room.GetPlugsIntoSlotId() == slotID {
					matchingRooms = append(matchingRooms, room)
				}
			}
		}
	} else { // otherwise, just pick any room
		for _, room := range g.getTemplatesForLevel() {
			if room.GetPlugsIntoSlotId() == slotID {
				if _, ok := g.placedTemplates[room.GetTemplateName()]; ok {
					if _, ok := g.levelRules.RequireTemplates[room.GetTemplateName()]; ok {
						continue
					}
				}
				matchingRooms = append(matchingRooms, room)
			}
		}
	}

	if len(matchingRooms) == 0 {
		return nil, fmt.Errorf("dungen: no %q template plugs into slot %d", g.levelRules.TemplatePrefix, slotID)
	}

	randomTemplate := matchingRooms[g.random.Intn(len(matchingRooms))]
	return randomTemplate.Clone(), nil
}

// getFirstRoom picks the room a level grows from: one whose connectors take any room (connection ID 0).
func (g *AccretionGenerator) getFirstRoom() (*DungeonRoom, error) {
	var candidates []*DungeonRoom
	for _, room := range g.getTemplatesForLevel() {
		if room.HasOnlyZeroIDConnections() {
			candidates = append(candidates, room)
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("dungen: no %q template has only unnumbered connectors to start a level from", g.levelRules.TemplatePrefix)
	}
	return candidates[g.random.Intn(len(candidates))].Clone(), nil
}

func (g *AccretionGenerator) nextRoomForGenerator(slotID int) (*DungeonRoom, error) {
	newRoom, err := g.getNewRoomFromRandomTemplateForSlot(slotID)
	if err != nil {
		return nil, err
	}
	newRoom.SetId(g.currentRoomId)
	g.currentRoomId++
	return newRoom, nil
}

// Generate grows one level. It fails only on template data that cannot build a level at all; a level that came
// out small or without its required rooms is the caller's to judge (see MissingRequired).
func (g *AccretionGenerator) Generate(width, height int) (*DungeonMap, error) {
	g.placedTemplates = make(map[string]bool)
	g.currentRoomId = 0
	dMap := NewDungeonMap(width, height)

	firstRoom, err := g.getFirstRoom()
	if err != nil {
		return nil, err
	}
	firstRoom.SetId(g.currentRoomId)
	g.currentRoomId++

	roomSize := firstRoom.GetRelativeBoundingRectIncludingConnectors().Size()
	centerMapPos := geometry.Point{X: width - roomSize.X, Y: height - roomSize.Y}.Div(2)
	firstRoom.SetPositionOffset(centerMapPos)

	if !dMap.CanPlaceRoom(firstRoom) {
		return nil, fmt.Errorf("dungen: template %s does not fit a %dx%d level", firstRoom.GetTemplateName(), width, height)
	}

	dMap.AddRoomAndSetTiles(firstRoom)
	g.placedTemplates[firstRoom.GetTemplateName()] = true

	if g.printDebugSteps {
		println("=== first room ===")
		dMap.Print()
	}
	maxRooms := g.maxRoomCount
	repCounter := 0
	currentSlotID := 0
	for len(dMap.rooms) < maxRooms && repCounter < 100 {
		// A slot room whose partner will not fit is given up on rather than retried until the loop gives up on
		// the whole level.
		if currentSlotID != 0 && repCounter >= slotPatience {
			currentSlotID = 0
		}
		nextRoom, err := g.nextRoomForGenerator(currentSlotID)
		if err != nil {
			return nil, err
		}
		if g.printDebugSteps {
			println(fmt.Sprintf("=== room %d ===", nextRoom.roomId))
			nextRoom.PrintUntransformed()
		}
		for _, existingRoom := range g.attachOrder(dMap.rooms) {
			if g.tryConnectTo(dMap, nextRoom, existingRoom) {
				repCounter = 0
				dMap.AddRoomAndSetTiles(nextRoom)
				g.placedTemplates[nextRoom.GetTemplateName()] = true
				currentSlotID = nextRoom.GetSlotId()
				if g.printDebugSteps {
					println(fmt.Sprintf("=== added room %d ===", nextRoom.roomId))
					dMap.PrintPlannedDoors()
				}
				break
			}
		}

		repCounter++
	}
	if len(dMap.rooms) < maxRooms && g.printDebugSteps {
		// Not an error: the caller regrows the level if it came out too small, and a library does not print.
		println("=== COULD NOT PLACE ALL ROOMS ===")
	}

	if g.printDebugSteps {
		println("=== BEFORE TRIM ===")
		dMap.PrintPlannedDoors()
		dMap.Trim()
		println("=== AFTER TRIM ===")
		dMap.PrintPlannedDoors()
	}

	// Growing makes a tree; the extra openings make loops, so the way back is not always the way in.
	dMap.ConnectRoomsOnMap(!g.levelRules.NoDoors)
	dMap.addMoreDoors(g.random, g.loopDistance, g.loopChance, !g.levelRules.NoDoors)

	return dMap, nil
}

// MissingRequired names a template the level must contain but the last Generate did not place, or "".
func (g *AccretionGenerator) MissingRequired() string {
	for _, name := range slices.Sorted(maps.Keys(g.levelRules.RequireTemplates)) {
		if !g.placedTemplates[name] {
			return name
		}
	}
	return ""
}

// slotPatience is how many failed placements a slot's partner room gets before growth moves on without it.
const slotPatience = 10

// attachOrder is the order a new room tries the placed rooms in: random, weighted towards the newest, so a level
// grows outwards instead of crowding around its first room (weighted shuffle, Efraimidis-Spirakis).
func (g *AccretionGenerator) attachOrder(rooms []*DungeonRoom) []*DungeonRoom {
	keys := make(map[*DungeonRoom]float64, len(rooms))
	for i, room := range rooms {
		keys[room] = math.Pow(g.random.Float64(), 1/float64(i+1))
	}
	order := slices.Clone(rooms)
	slices.SortStableFunc(order, func(a, b *DungeonRoom) int { return cmp.Compare(keys[b], keys[a]) })
	return order
}

func (g *AccretionGenerator) tryConnectTo(dMap *DungeonMap, newRoom *DungeonRoom, existingRoom *DungeonRoom) bool {
	if g.printDebugSteps {
		println(fmt.Sprintf("=== try connect room %d to room %d ===", newRoom.roomId, existingRoom.roomId))
	}
	freeConnections := existingRoom.GetFreeConnectionsWithRotatedDirection()
	relPositions := getSortedKeys(freeConnections)
	orientations := 4
	if g.mirror {
		orientations = 8
	}
	for _, orientation := range g.random.Perm(orientations) {
		newRoom.SetTransform(orientation%4, orientation >= 4)
		if g.printDebugSteps {
			println(fmt.Sprintf("=== orientation %d for room %d ===", orientation, newRoom.roomId))
			newRoom.PrintRotated()
		}
		for _, relativeConnectionPos := range relPositions {
			roomConnection := freeConnections[relativeConnectionPos]
			if relativeNewDoorPos, ok := newRoom.HasMatchingConnection(roomConnection); ok {
				rotatedRelativeDoorPos := newRoom.GetRotatedRelativeDoorPosition(relativeNewDoorPos)
				// both have free doors in opposite directions
				absoluteDoorPos := existingRoom.GetAbsoluteDoorPosition(relativeConnectionPos)

				// the new room should be placed so that the new door is at the same position as the existing door
				newRoom.SetPositionOffset(absoluteDoorPos.Sub(rotatedRelativeDoorPos))
				if g.printDebugSteps {
					println(fmt.Sprintf("=== CONNECTION POSSIBLE ==="))
					println(fmt.Sprintf("> NEW ROOM %d", newRoom.roomId))
					newRoom.PrintTransformedWithHighlight(relativeNewDoorPos)
					println(fmt.Sprintf("> EXISTING ROOM %d", existingRoom.roomId))
					existingRoom.PrintTransformedWithHighlight(relativeConnectionPos)
				}
				if dMap.CanPlaceRoomNextTo(newRoom, existingRoom) {
					if g.printDebugSteps {
						println(fmt.Sprintf("=== CONNECTED ==="))
					}
					newRoom.AddConnectedRoom(relativeNewDoorPos, existingRoom)
					existingRoom.AddConnectedRoom(relativeConnectionPos, newRoom)
					return true
				} else if g.printDebugSteps && newRoom.GetPlugsIntoSlotId() > 0 {
					println(fmt.Sprintf("=== COULD NOT FIT ==="))
					println(fmt.Sprintf("> NEW ROOM %d", newRoom.roomId))
					newRoom.PrintTransformedWithHighlight(relativeNewDoorPos)
					println(fmt.Sprintf("> EXISTING ROOM %d", existingRoom.roomId))
					existingRoom.PrintTransformedWithHighlight(relativeConnectionPos)
					println(fmt.Sprintf("> MAP"))
					dMap.PrintWithHighlight(absoluteDoorPos)
				}
			}
		}
	}

	return false
}

func getSortedKeys(doors map[geometry.Point]RoomConnection) []geometry.Point {
	keys := make([]geometry.Point, len(doors))
	i := 0
	for k := range doors {
		keys[i] = k
		i++
	}
	slices.SortFunc(keys, comparePoints)
	return keys
}

// comparePoints orders points row by row. Map iteration order is random, so anything that feeds the RNG from a
// map has to go through an order like this to keep a seed reproducible; it must not assume a template width.
func comparePoints(a, b geometry.Point) int {
	return cmp.Or(cmp.Compare(a.Y, b.Y), cmp.Compare(a.X, b.X))
}

// TileIDs are the tiles in a template's Tiles layer that mean floor and wall; everything else is ignored.
type TileIDs struct{ Wall, Floor int }

// LoadTemplatesFrom adds every non-decoration template whose name starts with prefix. A template whose
// connectors do not sit on the edge of its floor is refused: placed, it would open onto nothing, and that is
// what cut levels in two.
func (g *AccretionGenerator) LoadTemplatesFrom(templates *ldtk.Project, prefix string, ids TileIDs) error {
	for _, level := range templates.Levels {
		if !strings.HasPrefix(level.Identifier, prefix) || level.PropertyByIdentifier("IsDecoration").AsBool() {
			continue
		}
		room := templateFromLevel(level, ids)
		for _, pos := range getSortedKeys(room.availableConnections) {
			c := room.availableConnections[pos]
			if !room.floorTiles[pos.Sub(c.OutwardDirection.ToPoint())] {
				return fmt.Errorf("dungen: template %s: connector at %v faces %v, away from no floor tile", level.Identifier, pos, c.OutwardDirection)
			}
		}
		g.roomTemplates = append(g.roomTemplates, room)
	}
	return nil
}

func templateFromLevel(level *ldtk.Level, ids TileIDs) *DungeonRoom {
	tileLayer := level.LayerByIdentifier("Tiles")
	metaLayer := level.LayerByIdentifier("Meta")

	var floorTiles []geometry.Point
	var wallTiles []geometry.Point
	for _, tile := range tileLayer.Tiles {
		posX, posY := tileLayer.ToGridPosition(tile.Position[0], tile.Position[1])
		pos := geometry.Point{X: posX, Y: posY}
		if tile.ID == ids.Floor {
			floorTiles = append(floorTiles, pos)
		} else if tile.ID == ids.Wall {
			wallTiles = append(wallTiles, pos)
		}
	}

	var connectors = make(map[geometry.Point]RoomConnection)

	var exposedPoints []geometry.Point
	for _, entity := range metaLayer.Entities {
		posX, posY := metaLayer.ToGridPosition(entity.Position[0], entity.Position[1])
		pos := geometry.Point{X: posX, Y: posY}

		if strings.HasPrefix(entity.Identifier, "POI") {
			exposedPoints = append(exposedPoints, pos)
		}
		if strings.HasSuffix(entity.Identifier, "Connector") {
			direction := compassDirectionFromId(entity.Identifier)
			idProp := entity.PropertyByIdentifier("ConnectionID").AsInt()
			connectors[pos] = RoomConnection{
				OutwardDirection: direction,
				ConnectionID:     idProp,
			}
		}
	}

	slotId := level.PropertyByIdentifier("SlotID").AsInt()
	plugsIn := level.PropertyByIdentifier("PlugsIntoSlotID").AsInt()
	canBeLit := level.PropertyByIdentifier("CanBeLit").AsBool()
	dRoom := NewDungeonRoomFromTemplate(level.Identifier)
	dRoom.SetSlotId(slotId)
	dRoom.SetPlugsIntoSlotId(plugsIn)
	dRoom.SetFloorTiles(floorTiles)
	dRoom.SetIncludedWallTiles(wallTiles)
	dRoom.SetPOIPositions(exposedPoints)
	dRoom.SetConnections(connectors)
	dRoom.SetCanBeLit(canBeLit)

	if !level.PropertyByIdentifier("UsesDecoration").IsNull() {
		dRoom.SetDecorationLevelName(level.PropertyByIdentifier("UsesDecoration").AsString())
	}

	return dRoom
}

func (g *AccretionGenerator) SetRoomCount(maxRoomCount int) {
	g.maxRoomCount = maxRoomCount
}

// SetMirroring lets rooms be flipped as well as turned, which doubles the layouts each template gives.
func (g *AccretionGenerator) SetMirroring(mirror bool) {
	g.mirror = mirror
}

func (g *AccretionGenerator) SetLevelRules(rules TemplateRules) {
	g.levelRules = rules
}

func compassDirectionFromId(identifier string) geometry.CompassDirection {
	switch identifier {
	case "NorthConnector":
		return geometry.North
	case "SouthConnector":
		return geometry.South
	case "EastConnector":
		return geometry.East
	case "WestConnector":
		return geometry.West
	}
	panic("unknown connector identifier: " + identifier)
}
