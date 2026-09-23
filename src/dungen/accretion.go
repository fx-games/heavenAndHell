package dungen

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"math/rand"
	"rpg/src/geometry"
	"rpg/src/hhm"
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
	currentRoomId   int
	levelRules      TemplateRules
	placedTemplates map[string]bool
	mirror          bool
	loopDistance    int
	loopChance      float64
	lakes           int
}

func NewAccretionGenerator(randomSource *rand.Rand) *AccretionGenerator {
	return &AccretionGenerator{
		random:        randomSource,
		currentRoomId: 0,
		loopDistance:  20,
		loopChance:    1,
	}
}

func (g *AccretionGenerator) getTemplatesForLevel() []*DungeonRoom {
	return slices.DeleteFunc(slices.Clone(g.roomTemplates), func(r *DungeonRoom) bool {
		return g.levelRules.ForbidTemplates[r.GetTemplateName()]
	})
}

// pick is a weighted random pick (a template's Weight; 0 counts as 1), nil from none.
func (g *AccretionGenerator) pick(rooms []*DungeonRoom) *DungeonRoom {
	total := 0
	for _, r := range rooms {
		total += max(1, r.weight)
	}
	if total == 0 {
		return nil
	}
	n := g.random.Intn(total)
	for _, r := range rooms {
		if n -= max(1, r.weight); n < 0 {
			return r.Clone()
		}
	}
	return nil
}

// nextRoom picks a room to grow the level with: any allowed template that plugs into slotID, except the required
// rooms, which attachRequired adds once the level has grown. Nil when none plugs into that slot.
func (g *AccretionGenerator) nextRoom(slotID int) *DungeonRoom {
	var rooms []*DungeonRoom
	for _, r := range g.getTemplatesForLevel() {
		if r.GetPlugsIntoSlotId() == slotID && !g.levelRules.RequireTemplates[r.GetTemplateName()] {
			rooms = append(rooms, r)
		}
	}
	room := g.pick(rooms)
	if room != nil {
		room.SetId(g.currentRoomId)
		g.currentRoomId++
	}
	return room
}

// getFirstRoom picks the room a level grows from: one whose connectors take any room (connection ID 0), not a
// required room unless nothing else can start the level.
func (g *AccretionGenerator) getFirstRoom() (*DungeonRoom, error) {
	var candidates, required []*DungeonRoom
	for _, room := range g.getTemplatesForLevel() {
		switch {
		case !room.HasOnlyZeroIDConnections():
		case g.levelRules.RequireTemplates[room.GetTemplateName()]:
			required = append(required, room)
		default:
			candidates = append(candidates, room)
		}
	}
	if len(candidates) == 0 {
		candidates = required
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("dungen: no %q template has only unnumbered connectors to start a level from", g.levelRules.TemplatePrefix)
	}
	return g.pick(candidates), nil
}

// attachRequired adds each required room not placed yet, newest rooms first, so a quest room sits far out
// rather than by the first room. Growth places them from halfway on; this catches what it could not.
func (g *AccretionGenerator) attachRequired(dMap *DungeonMap) {
	for _, name := range slices.Sorted(maps.Keys(g.levelRules.RequireTemplates)) {
		if g.placedTemplates[name] {
			continue
		}
		i := slices.IndexFunc(g.roomTemplates, func(r *DungeonRoom) bool { return r.GetTemplateName() == name })
		if i < 0 {
			continue // MissingRequired reports it
		}
		room := g.roomTemplates[i].Clone()
		room.SetId(g.currentRoomId)
		g.currentRoomId++
		if !g.attach(dMap, room, dMap.rooms) && room.GetPlugsIntoSlotId() != 0 {
			// growth spends every slot as it opens: grow a room providing the slot, and hang the room on it
			var partners []*DungeonRoom
			for _, r := range g.getTemplatesForLevel() {
				if r.GetSlotId() == room.GetPlugsIntoSlotId() && r.GetPlugsIntoSlotId() == 0 {
					partners = append(partners, r)
				}
			}
			if partner := g.pick(partners); partner != nil {
				partner.SetId(g.currentRoomId)
				g.currentRoomId++
				if g.attach(dMap, partner, dMap.rooms) {
					g.attach(dMap, room, []*DungeonRoom{partner})
				}
			}
		}
	}
}

// missingFor is a required room not placed yet that plugs into slotID, or nil; into slot 0 only once halfway.
func (g *AccretionGenerator) missingFor(slotID int, halfway bool) *DungeonRoom {
	if slotID == 0 && !halfway {
		return nil
	}
	for _, name := range slices.Sorted(maps.Keys(g.levelRules.RequireTemplates)) {
		i := slices.IndexFunc(g.roomTemplates, func(r *DungeonRoom) bool { return r.GetTemplateName() == name })
		if !g.placedTemplates[name] && i >= 0 && g.roomTemplates[i].GetPlugsIntoSlotId() == slotID {
			room := g.roomTemplates[i].Clone()
			room.SetId(g.currentRoomId)
			g.currentRoomId++
			return room
		}
	}
	return nil
}

// partnerFor is a room providing the slot a missing required room plugs into, or nil.
func (g *AccretionGenerator) partnerFor() *DungeonRoom {
	var partners []*DungeonRoom
	for name := range g.levelRules.RequireTemplates {
		i := slices.IndexFunc(g.roomTemplates, func(r *DungeonRoom) bool { return r.GetTemplateName() == name })
		if g.placedTemplates[name] || i < 0 || g.roomTemplates[i].GetPlugsIntoSlotId() == 0 {
			continue
		}
		for _, r := range g.getTemplatesForLevel() {
			if r.GetSlotId() == g.roomTemplates[i].GetPlugsIntoSlotId() && r.GetPlugsIntoSlotId() == 0 && !slices.Contains(partners, r) {
				partners = append(partners, r)
			}
		}
	}
	slices.SortFunc(partners, func(a, b *DungeonRoom) int { return strings.Compare(a.GetTemplateName(), b.GetTemplateName()) })
	room := g.pick(partners)
	if room != nil {
		room.SetId(g.currentRoomId)
		g.currentRoomId++
	}
	return room
}

// attach connects room to one of rooms, the farthest from the level's first room first, and adds it to the
// level; false when none fits.
func (g *AccretionGenerator) attach(dMap *DungeonMap, room *DungeonRoom, rooms []*DungeonRoom) bool {
	for _, existing := range g.farthestFirst(dMap, rooms) {
		if g.tryConnectTo(dMap, room, existing) {
			dMap.AddRoomAndSetTiles(room)
			g.placedTemplates[room.GetTemplateName()] = true
			return true
		}
	}
	return false
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
	firstRoom.SetPositionOffset(geometry.Point{X: width - roomSize.X, Y: height - roomSize.Y}.Div(2))
	if !dMap.CanPlaceRoom(firstRoom) {
		return nil, fmt.Errorf("dungen: template %s does not fit a %dx%d level", firstRoom.GetTemplateName(), width, height)
	}
	dMap.AddRoomAndSetTiles(firstRoom)
	g.placedTemplates[firstRoom.GetTemplateName()] = true

	repCounter := 0
	currentSlotID := 0
	for len(dMap.rooms) < g.maxRoomCount && repCounter < 100 {
		repCounter++
		// A slot room whose partner will not fit is given up on rather than retried until the loop gives up on
		// the whole level.
		if currentSlotID != 0 && repCounter >= slotPatience {
			currentSlotID = 0
		}
		nextRoom := g.nextRoom(currentSlotID)
		// Required rooms: one plugging into any room waits for the level to be half grown, so it sits far out; one
		// needing a slot takes the first that opens (hub rooms only fit early), growing a hub for it if need be.
		if r := g.missingFor(currentSlotID, len(dMap.rooms) >= g.maxRoomCount/2); r != nil {
			nextRoom = r
		} else if currentSlotID == 0 && repCounter <= 3 {
			if r := g.partnerFor(); r != nil {
				nextRoom = r // a few tries, then growth goes on without it
			}
		}
		order := g.attachOrder(dMap.rooms)
		if g.levelRules.RequireTemplates[nextRoom.GetTemplateName()] {
			order = g.farthestFirst(dMap, dMap.rooms) // a quest room goes as far out as it fits
		}
		for _, existingRoom := range order {
			if g.tryConnectTo(dMap, nextRoom, existingRoom) {
				repCounter = 0
				dMap.AddRoomAndSetTiles(nextRoom)
				g.placedTemplates[nextRoom.GetTemplateName()] = true
				currentSlotID = nextRoom.GetSlotId()
				break
			}
		}
	}
	g.attachRequired(dMap)
	dMap.Trim()

	// Growing makes a tree; the extra openings make loops, so the way back is not always the way in.
	dMap.ConnectRoomsOnMap(!g.levelRules.NoDoors)
	dMap.addMoreDoors(g.random, g.loopDistance, g.loopChance, !g.levelRules.NoDoors)
	dMap.openDoorways(g.random, doorChance)
	dMap.addLakes(g.random, g.lakes)

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

// doorChance is how many doorways get a door; the rest stay open (Brogue's digDungeon: 60%).
const doorChance = 0.6

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

// farthestFirst orders rooms by how many rooms one walks through from the level's first room to reach them,
// farthest first, the way Brogue puts its vaults far from the stairs; ties keep attachOrder's weighted shuffle.
func (g *AccretionGenerator) farthestFirst(dMap *DungeonMap, rooms []*DungeonRoom) []*DungeonRoom {
	first := dMap.rooms[0]
	hops := map[*DungeonRoom]int{first: 0}
	for queue := []*DungeonRoom{first}; len(queue) > 0; queue = queue[1:] {
		r := queue[0]
		for _, door := range slices.SortedFunc(maps.Keys(r.connectedRooms), comparePoints) {
			if n := r.connectedRooms[door]; n != nil {
				if _, seen := hops[n]; !seen {
					hops[n] = hops[r] + 1
					queue = append(queue, n)
				}
			}
		}
	}
	order := g.attachOrder(rooms)
	slices.SortStableFunc(order, func(a, b *DungeonRoom) int { return hops[b] - hops[a] })
	return order
}

func (g *AccretionGenerator) tryConnectTo(dMap *DungeonMap, newRoom *DungeonRoom, existingRoom *DungeonRoom) bool {
	freeConnections := existingRoom.GetFreeConnectionsWithRotatedDirection()
	relPositions := getSortedKeys(freeConnections)
	orientations := 4
	if g.mirror {
		orientations = 8
	}
	for _, orientation := range g.random.Perm(orientations) {
		newRoom.SetTransform(orientation%4, orientation >= 4)
		for _, relativeConnectionPos := range relPositions {
			relativeNewDoorPos, ok := newRoom.HasMatchingConnection(freeConnections[relativeConnectionPos])
			if !ok {
				continue
			}
			// place the new room so its door lands on the existing room's free door
			absoluteDoorPos := existingRoom.GetAbsoluteDoorPosition(relativeConnectionPos)
			newRoom.SetPositionOffset(absoluteDoorPos.Sub(newRoom.GetRotatedRelativeDoorPosition(relativeNewDoorPos)))
			if dMap.CanPlaceRoomNextTo(newRoom, existingRoom) {
				newRoom.AddConnectedRoom(relativeNewDoorPos, existingRoom)
				existingRoom.AddConnectedRoom(relativeConnectionPos, newRoom)
				return true
			}
		}
	}
	return false
}

func getSortedKeys(doors map[geometry.Point]RoomConnection) []geometry.Point {
	return slices.SortedFunc(maps.Keys(doors), comparePoints)
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
func (g *AccretionGenerator) LoadTemplatesFrom(templates *hhm.Project, prefix string, ids TileIDs) error {
	for _, level := range templates.Levels {
		if !strings.HasPrefix(level.Name, prefix) || level.Props.IsDecoration {
			continue
		}
		room := templateFromLevel(level, ids)
		for _, pos := range getSortedKeys(room.availableConnections) {
			c := room.availableConnections[pos]
			if !room.floorTiles[pos.Sub(c.OutwardDirection.ToPoint())] {
				return fmt.Errorf("dungen: template %s: connector at %v faces %v, away from no floor tile", level.Name, pos, c.OutwardDirection)
			}
		}
		g.roomTemplates = append(g.roomTemplates, room)
	}
	return nil
}

func templateFromLevel(level *hhm.Level, ids TileIDs) *DungeonRoom {
	var floorTiles []geometry.Point
	var wallTiles []geometry.Point
	for y := 0; y < level.Height; y++ {
		for x := 0; x < level.Width; x++ {
			pos := geometry.Point{X: x, Y: y}
			switch level.TileAt(x, y) {
			case ids.Floor:
				floorTiles = append(floorTiles, pos)
			case ids.Wall:
				wallTiles = append(wallTiles, pos)
			}
		}
	}

	var connectors = make(map[geometry.Point]RoomConnection)
	var exposedPoints []geometry.Point
	for _, entity := range level.Entities {
		pos := geometry.Point{X: entity.X, Y: entity.Y}
		if strings.HasPrefix(entity.Kind, "POI") {
			exposedPoints = append(exposedPoints, pos)
		}
		if strings.HasSuffix(entity.Kind, "Connector") {
			connectors[pos] = RoomConnection{
				OutwardDirection: compassDirectionFromId(entity.Kind),
				ConnectionID:     entity.Int("ConnectionID"),
			}
		}
	}

	dRoom := NewDungeonRoomFromTemplate(level.Name)
	dRoom.SetSlotId(level.Props.SlotID)
	dRoom.SetPlugsIntoSlotId(level.Props.PlugsIntoSlotID)
	dRoom.SetFloorTiles(floorTiles)
	dRoom.SetIncludedWallTiles(wallTiles)
	dRoom.SetPOIPositions(exposedPoints)
	dRoom.SetConnections(connectors)
	dRoom.SetCanBeLit(level.Props.CanBeLit)
	dRoom.SetDecorationLevelName(level.Props.UsesDecoration)
	dRoom.weight = level.Props.Weight
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
