package gridmap

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"math/rand"
	"os"
	"rpg/src/geometry"
	"sort"
	"strconv"
	"time"
)

type NamedLocation struct {
	LocationName string
	Pos          geometry.Point
}
type Transition struct {
	TargetMap      string
	TargetLocation string
}

func (t Transition) IsEmpty() bool {
	return t.TargetMap == "" && t.TargetLocation == ""
}

func (t Transition) Encode() string {
	return fmt.Sprintf("transition(%s, %s)", t.TargetMap, t.TargetLocation)
}

type MapObject interface {
	MapPosition() geometry.Point
	Icon(tick uint64) int32
	SetMapPosition(geometry.Point)
}

type MapActor interface {
	MapObject
}
type MapObjectWithProperties[ActorType interface {
	comparable
	MapActor
}] interface {
	MapObject
	IsWalkable(person ActorType) bool
	IsTransparent() bool
	IsPassableForProjectile() bool
}
type ZoneType int

func (t ZoneType) ToString() string {
	switch t {
	case ZoneTypePublic:
		return "Public"
	case ZoneTypePrivate:
		return "Private"
	case ZoneTypeHighSecurity:
		return "High Security"
	case ZoneTypeDropOff:
		return "Drop Off"
	}
	return "Unknown"
}

const (
	ZoneTypePublic ZoneType = iota
	ZoneTypePrivate
	ZoneTypeHighSecurity
	ZoneTypeDropOff
)

type ZoneInfo struct {
	Name        string
	Type        ZoneType
	AmbienceCue string
}

const PublicZoneName = "Public Space"

func (i ZoneInfo) IsDropOff() bool {
	return i.Type == ZoneTypeDropOff
}

func (i ZoneInfo) IsHighSecurity() bool {
	return i.Type == ZoneTypeHighSecurity || i.Type == ZoneTypeDropOff
}

func (i ZoneInfo) IsPublic() bool {
	return i.Type == ZoneTypePublic
}

func (i ZoneInfo) IsPrivate() bool {
	return i.Type == ZoneTypePrivate
}

func (i ZoneInfo) ToString() string {
	return fmt.Sprintf("%s (%s)", i.Name, i.Type.ToString())
}

func NewPublicZone(name string) *ZoneInfo {
	return &ZoneInfo{
		Name: name,
		Type: ZoneTypePublic,
	}
}

type GlobalMapDataOnDisk struct {
	MissionTitle      string
	Width             int
	Height            int
	PlayerSpawn       geometry.Point
	MaxLightIntensity float64
	MaxVisionRange    int
	TimeOfDay         time.Time
	AmbienceSoundCue  string
}

type GridMap[ActorType interface {
	comparable
	MapActor
}, ItemType interface {
	comparable
	MapObject
}, ObjectType interface {
	comparable
	MapObjectWithProperties[ActorType]
}] struct {
	name            string
	cells           []MapCell[ActorType, ItemType, ObjectType]
	allActors       []ActorType
	allDownedActors []ActorType
	removedActors   []ActorType
	allItems        []ItemType
	allObjects      []ObjectType

	decals map[geometry.Point]int32

	playerSpawn geometry.Point

	mapWidth  int
	mapHeight int

	pathfinder  *geometry.PathRange
	listOfZones []*ZoneInfo
	zoneMap     []*ZoneInfo

	timeOfDay time.Time

	namedLocations map[string]geometry.Point
	noClip         bool

	transitionMap map[geometry.Point]Transition
	secretDoors   map[geometry.Point]bool

	namedRects   map[string]geometry.Rect
	namedTrigger map[string]Trigger

	namedPaths     map[string][]geometry.Point
	displayName    string
	metaInfoString []string

	cardinalMovementOnly bool
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SetCardinalMovementOnly(cardinalMovementOnly bool) {
	m.cardinalMovementOnly = cardinalMovementOnly
}
func (m *GridMap[ActorType, ItemType, ObjectType]) AddZone(zone *ZoneInfo) {
	m.listOfZones = append(m.listOfZones, zone)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SetTile(position geometry.Point, mapTile Tile) {
	if !m.Contains(position) {
		return
	}
	index := position.Y*m.mapWidth + position.X
	m.cells[index].TileType = mapTile
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SetZone(position geometry.Point, zone *ZoneInfo) {
	if !m.Contains(position) {
		return
	}
	m.zoneMap[position.Y*m.mapWidth+position.X] = zone
}

func (m *GridMap[ActorType, ItemType, ObjectType]) RemoveItemAt(position geometry.Point) {
	if item, ok := m.TryGetItemAt(position); ok {
		m.RemoveItem(item)
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) RemoveObjectAt(position geometry.Point) {
	if object, ok := m.TryGetObjectAt(position); ok {
		m.RemoveObject(object)
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) RemoveObject(obj ObjectType) {
	m.cells[obj.MapPosition().Y*m.mapWidth+obj.MapPosition().X] = m.cells[obj.MapPosition().Y*m.mapWidth+obj.MapPosition().X].WithObjectHereRemoved(obj)
	for i := len(m.allObjects) - 1; i >= 0; i-- {
		if m.allObjects[i] == obj {
			m.allObjects = append(m.allObjects[:i], m.allObjects[i+1:]...)
			return
		}
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IterAll(f func(p geometry.Point, c MapCell[ActorType, ItemType, ObjectType])) {
	for y := 0; y < m.mapHeight; y++ {
		for x := 0; x < m.mapWidth; x++ {
			f(geometry.Point{X: x, Y: y}, m.cells[y*m.mapWidth+x])
		}
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IterWindow(window geometry.Rect, f func(p geometry.Point, c MapCell[ActorType, ItemType, ObjectType])) {
	for y := window.Min.Y; y < window.Max.Y; y++ {
		for x := window.Min.X; x < window.Max.X; x++ {
			mapPos := geometry.Point{X: x, Y: y}
			if !m.Contains(mapPos) {
				continue
			}
			f(mapPos, m.cells[y*m.mapWidth+x])
		}
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SetPlayerSpawn(position geometry.Point) {
	m.playerSpawn = position
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SaveToDisk(path string) error {
	file, _ := os.Open(path)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	encodeErr := enc.Encode(m)
	encodedMap := buf.Bytes()
	writeCount, writeErr := file.Write(encodedMap)
	if encodeErr == nil && writeErr == nil {
		println("Map saved to file: " + path + ", length: " + strconv.Itoa(writeCount))
		return nil
	} else {
		println("Error saving map to file: " + path)
		if encodeErr != nil {
			println(encodeErr.Error())
			return encodeErr
		} else if writeErr != nil {
			println(writeErr.Error())
			return writeErr
		}
	}
	return nil
}

func (m *GridMap[ActorType, ItemType, ObjectType]) CellAt(location geometry.Point) MapCell[ActorType, ItemType, ObjectType] {
	return m.cells[m.mapWidth*location.Y+location.X]
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsItemAt(location geometry.Point) bool {
	return m.cells[m.mapWidth*location.Y+location.X].Item != nil
}
func (m *GridMap[ActorType, ItemType, ObjectType]) SetActorToDowned(a ActorType) {
	if !m.RemoveActor(a) {
		println("Could not remove actor from map")
		return
	}
	m.allDownedActors = append(m.allDownedActors, a)
	if other, ok := m.TryGetDownedActorAt(a.MapPosition()); ok && other != a {
		m.displaceDownedActor(a)
		return
	}
	m.cells[a.MapPosition().Y*m.mapWidth+a.MapPosition().X] = m.cells[a.MapPosition().Y*m.mapWidth+a.MapPosition().X].WithDownedActor(a)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SetActorToRemoved(person ActorType) {
	m.RemoveActor(person)
	m.RemoveDownedActor(person)
	m.removedActors = append(m.removedActors, person)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SetActorToNormal(person ActorType) {
	m.RemoveDownedActor(person)
	m.allActors = append(m.allActors, person)
	if m.IsActorAt(person.MapPosition()) && m.actorAt(person.MapPosition()) != person {
		m.displaceActor(person)
		return
	}
	m.cells[person.MapPosition().Y*m.mapWidth+person.MapPosition().X] = m.cells[person.MapPosition().Y*m.mapWidth+person.MapPosition().X].WithActor(person)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) MoveItem(item ItemType, to geometry.Point) {
	m.cells[item.MapPosition().Y*m.mapWidth+item.MapPosition().X] = m.cells[item.MapPosition().Y*m.mapWidth+item.MapPosition().X].WithItemHereRemoved(item)
	item.SetMapPosition(to)
	//util.XYToIndex()
	m.cells[to.Y*m.mapWidth+to.X] = m.cells[to.Y*m.mapWidth+to.X].WithItem(item)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetRandomFreeAndSafeNeighbor(source *rand.Rand, location geometry.Point) geometry.Point {
	freeNeighbors := m.GetFilteredNeighbors(location, func(p geometry.Point) bool {
		return m.Contains(p) && m.IsCurrentlyPassable(p) && !m.IsObviousHazardAt(p)
	})
	if len(freeNeighbors) == 0 {
		return location
	}
	return freeNeighbors[source.Intn(len(freeNeighbors))]
}
func (m *GridMap[ActorType, ItemType, ObjectType]) GetRandomNeighbor(source *rand.Rand, location geometry.Point) geometry.Point {
	freeNeighbors := m.GetFilteredNeighbors(location, func(p geometry.Point) bool {
		return m.Contains(p)
	})
	if len(freeNeighbors) == 0 {
		return location
	}
	return freeNeighbors[source.Intn(len(freeNeighbors))]
}

type SetOfPoints map[geometry.Point]bool

func (s *SetOfPoints) Contains(p geometry.Point) bool {
	_, ok := (*s)[p]
	return ok
}

func lessByCoordinate(a, b geometry.Point) bool {
	return a.Y < b.Y || (a.Y == b.Y && a.X < b.X)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetFreeCellsForDistribution(position geometry.Point, neededCellCount int, freePredicate func(p geometry.Point) bool) []geometry.Point {
	// The flood is a FIFO queue and the result breaks distance ties by coordinate: this used to pop its open
	// list out of a Go map and sort the result unstably, so which cell an actor, a corpse or a dropped item
	// landed on changed from run to run and took the whole game's reproducibility with it.
	seen := SetOfPoints{position: true}
	queue := []geometry.Point{position}
	found := make(SetOfPoints)
	var freeCells []geometry.Point
	for len(queue) > 0 && len(freeCells) < neededCellCount {
		currentPosition := queue[0]
		queue = queue[1:]
		for _, neighbor := range m.GetFilteredNeighbors(currentPosition, freePredicate) {
			if !found.Contains(neighbor) {
				found[neighbor] = true
				freeCells = append(freeCells, neighbor)
			}
		}
		for _, neighbor := range m.GetFilteredNeighbors(currentPosition, m.IsTileWalkable) {
			if !seen.Contains(neighbor) {
				seen[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}
	sort.Slice(freeCells, func(i, j int) bool {
		di, dj := geometry.DistanceSquared(freeCells[i], position), geometry.DistanceSquared(freeCells[j], position)
		if di != dj {
			return di < dj
		}
		return lessByCoordinate(freeCells[i], freeCells[j])
	})
	return freeCells
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsNextToTileWithSpecial(pos geometry.Point, specialType SpecialTileType) bool {
	for _, neighbor := range m.GetAllCardinalNeighbors(pos) {
		if m.CellAt(neighbor).TileType.Special == specialType {
			return true
		}
	}
	return false
}

func (m *GridMap[ActorType, ItemType, ObjectType]) RemoveItem(item ItemType) {
	m.cells[item.MapPosition().Y*m.mapWidth+item.MapPosition().X] = m.cells[item.MapPosition().Y*m.mapWidth+item.MapPosition().X].WithItemHereRemoved(item)
	for i := len(m.allItems) - 1; i >= 0; i-- {
		if m.allItems[i] == item {
			m.allItems = append(m.allItems[:i], m.allItems[i+1:]...)
			return
		}
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetAllCardinalNeighbors(pos geometry.Point) []geometry.Point {
	neighbors := geometry.Neighbors{}
	allCardinalNeighbors := neighbors.Cardinal(pos, func(p geometry.Point) bool {
		return m.Contains(p)
	})
	return allCardinalNeighbors
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsSpecialAt(pos geometry.Point, specialValue SpecialTileType) bool {
	special := m.CellAt(pos).TileType.Special
	return special == specialValue
}

type DijkstraMapper struct {
	neighbors func(geometry.Point) []geometry.Point
	cost      func(geometry.Point, geometry.Point) int
}

func (d DijkstraMapper) Neighbors(point geometry.Point) []geometry.Point {
	return d.neighbors(point)
}

func (d DijkstraMapper) Cost(point geometry.Point, point2 geometry.Point) int {
	return d.cost(point, point2)
}

// update for entities:
// call update for every updatable entity (genMap, AllItems, AllObjects, tiles)
// default: just return
// entities have an internal schedule, waiting for ticks to happen

func NewEmptyMap[ActorType interface {
	comparable
	MapActor
}, ItemType interface {
	comparable
	MapObject
}, ObjectType interface {
	comparable
	MapObjectWithProperties[ActorType]
}](width, height int) *GridMap[ActorType, ItemType, ObjectType] {
	pathRange := geometry.NewPathRange(geometry.NewRect(0, 0, width, height))
	publicSpaceZone := NewPublicZone(PublicZoneName)
	m := &GridMap[ActorType, ItemType, ObjectType]{
		cells:           make([]MapCell[ActorType, ItemType, ObjectType], width*height),
		allActors:       make([]ActorType, 0),
		allDownedActors: make([]ActorType, 0),
		allItems:        make([]ItemType, 0),
		allObjects:      make([]ObjectType, 0),
		namedLocations:  map[string]geometry.Point{},
		listOfZones:     []*ZoneInfo{publicSpaceZone},
		zoneMap:         NewZoneMap(publicSpaceZone, width, height),
		mapWidth:        width,
		mapHeight:       height,
		timeOfDay:       time.Now(),
		pathfinder:      pathRange,
		secretDoors:     make(map[geometry.Point]bool),
		transitionMap:   make(map[geometry.Point]Transition),
		namedRects:      make(map[string]geometry.Rect),
		namedTrigger:    make(map[string]Trigger),
		namedPaths:      make(map[string][]geometry.Point),
		decals:          make(map[geometry.Point]int32),
	}
	m.Fill(MapCell[ActorType, ItemType, ObjectType]{
		TileType: Tile{
			DefinedIcon:        ' ',
			DefinedDescription: "empty space",
			IsWalkable:         true,
			IsTransparent:      true,
			Special:            SpecialTileNone,
		},
		IsExplored: false,
	})
	return m
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetCell(p geometry.Point) MapCell[ActorType, ItemType, ObjectType] {
	return m.cells[p.X+p.Y*m.mapWidth]
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SetCell(p geometry.Point, cell MapCell[ActorType, ItemType, ObjectType]) {
	m.cells[p.X+p.Y*m.mapWidth] = cell
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetActor(p geometry.Point) ActorType {
	return *m.cells[p.X+p.Y*m.mapWidth].Actor
}

func (m *GridMap[ActorType, ItemType, ObjectType]) RemoveActor(actor ActorType) bool {
	m.cells[actor.MapPosition().X+actor.MapPosition().Y*m.mapWidth] = m.cells[actor.MapPosition().X+actor.MapPosition().Y*m.mapWidth].WithActorHereRemoved(actor)
	for i := len(m.allActors) - 1; i >= 0; i-- {
		if m.allActors[i] == actor {
			m.allActors = append(m.allActors[:i], m.allActors[i+1:]...)
			return true
		}
	}
	return false
}

// MoveActor Should only be called my the model, so we can ensure that a HUD IsFinished will follow
func (m *GridMap[ActorType, ItemType, ObjectType]) MoveActor(actor ActorType, newPos geometry.Point) {
	m.MoveActorFrom(actor, actor.MapPosition(), newPos)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) MoveActorFrom(actor ActorType, from, to geometry.Point) {
	if !m.Contains(to) {
		return
	}
	if !m.IsWalkableFor(to, actor) {
		return
	}
	if m.Contains(from) {
		m.cells[from.X+from.Y*m.mapWidth] = m.cells[from.X+from.Y*m.mapWidth].WithActorHereRemoved(actor)
	}
	actor.SetMapPosition(to)
	m.cells[to.X+to.Y*m.mapWidth] = m.cells[to.X+to.Y*m.mapWidth].WithActor(actor)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) MoveObject(obj ObjectType, newPos geometry.Point) {
	m.cells[obj.MapPosition().X+obj.MapPosition().Y*m.mapWidth] = m.cells[obj.MapPosition().X+obj.MapPosition().Y*m.mapWidth].WithObjectHereRemoved(obj)
	obj.SetMapPosition(newPos)
	m.cells[newPos.X+newPos.Y*m.mapWidth] = m.cells[newPos.X+newPos.Y*m.mapWidth].WithObject(obj)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) Fill(mapCell MapCell[ActorType, ItemType, ObjectType]) {
	for i := range m.cells {
		m.cells[i] = mapCell
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsTransparent(p geometry.Point) bool {
	if !m.Contains(p) {
		return false
	}

	if objectAt, ok := m.TryGetObjectAt(p); ok && !objectAt.IsTransparent() {
		return false
	}

	return m.GetCell(p).TileType.IsTransparent
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsTileWalkable(point geometry.Point) bool {
	if !m.Contains(point) {
		return false
	}
	return m.GetCell(point).TileType.IsWalkable
}

func (m *GridMap[ActorType, ItemType, ObjectType]) Contains(dest geometry.Point) bool {
	return dest.X >= 0 && dest.X < m.mapWidth && dest.Y >= 0 && dest.Y < m.mapHeight
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsActorAt(location geometry.Point) bool {
	if !m.Contains(location) {
		return false
	}
	return m.cells[location.X+location.Y*m.mapWidth].Actor != nil
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsDownedActorAt(location geometry.Point) bool {
	if !m.Contains(location) {
		return false
	}
	return m.cells[location.X+location.Y*m.mapWidth].DownedActor != nil
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsObjectAt(location geometry.Point) bool {
	return m.cells[location.X+location.Y*m.mapWidth].Object != nil
}

func (m *GridMap[ActorType, ItemType, ObjectType]) ZoneAt(p geometry.Point) *ZoneInfo {
	return m.zoneMap[m.mapWidth*p.Y+p.X]
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetFilteredCardinalNeighbors(pos geometry.Point, filter func(geometry.Point) bool) []geometry.Point {
	neighbors := geometry.Neighbors{}
	filtered := neighbors.Cardinal(pos, filter)
	return filtered
}

func (m *GridMap[ActorType, ItemType, ObjectType]) Actors() []ActorType {
	return m.allActors
}

func (m *GridMap[ActorType, ItemType, ObjectType]) DownedActors() []ActorType {
	return m.allDownedActors
}

func (m *GridMap[ActorType, ItemType, ObjectType]) Items() []ItemType {
	return m.allItems
}

func (m *GridMap[ActorType, ItemType, ObjectType]) Objects() []ObjectType {
	return m.allObjects
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetFilteredNeighbors(pos geometry.Point, filter func(geometry.Point) bool) []geometry.Point {
	neighbors := geometry.Neighbors{}
	filtered := neighbors.All(pos, filter)
	return filtered
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetFilteredNeighborsForMovement(pos geometry.Point, filter func(geometry.Point) bool) []geometry.Point {
	neighbors := geometry.Neighbors{}
	var result []geometry.Point
	if m.cardinalMovementOnly {
		result = neighbors.Cardinal(pos, filter)
	} else {
		result = neighbors.All(pos, filter)
	}
	return result
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetFilteredDirectionsForMovement(filter func(geometry.Point) bool) []geometry.Point {
	neighbors := geometry.Neighbors{}
	var result []geometry.Point
	if m.cardinalMovementOnly {
		result = neighbors.Cardinal(geometry.Point{}, filter)
	} else {
		result = neighbors.All(geometry.Point{}, filter)
	}
	return result
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetFreeCardinalNeighbors(pos geometry.Point) []geometry.Point {
	neighbors := geometry.Neighbors{}
	freeNeighbors := neighbors.Cardinal(pos, func(p geometry.Point) bool {
		return m.Contains(p) && m.IsWalkable(p) && !m.IsActorAt(p)
	})
	return freeNeighbors
}

func (m *GridMap[ActorType, ItemType, ObjectType]) displaceDownedActor(a ActorType) {
	free := m.GetFreeCellsForDistribution(a.MapPosition(), 1, func(p geometry.Point) bool {
		return !m.IsDownedActorAt(p) && m.IsWalkable(p)
	})
	freePos := free[0]
	m.MoveDownedActor(a, freePos)
}
func (m *GridMap[ActorType, ItemType, ObjectType]) displaceActor(a ActorType) {
	free := m.GetFreeCellsForDistribution(a.MapPosition(), 1, func(p geometry.Point) bool {
		return !m.IsActorAt(p) && m.IsWalkable(p)
	})
	freePos := free[0]
	m.MoveActor(a, freePos)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) getCurrentlyPassableNeighbors(pos geometry.Point) []geometry.Point {
	neighbors := geometry.Neighbors{}
	freeNeighbors := neighbors.All(pos, func(p geometry.Point) bool {
		return m.Contains(p) && m.IsCurrentlyPassable(p)
	})
	return freeNeighbors
}
func (m *GridMap[ActorType, ItemType, ObjectType]) IsCurrentlyPassable(p geometry.Point) bool {
	if !m.Contains(p) {
		return false
	}
	return m.IsWalkable(p) && (!m.IsActorAt(p)) //&& !knownAsBlocked
}
func (m *GridMap[ActorType, ItemType, ObjectType]) CurrentlyPassableAndSafeForActor(person ActorType) func(p geometry.Point) bool {
	return func(p geometry.Point) bool {
		if !m.Contains(p) ||
			(m.IsActorAt(p) && m.actorAt(p) != person) {
			return false
		}
		if _, isTransition := m.GetTransitionAt(p); isTransition { // a path walked for the player never crosses a portal or stairs
			return false
		}
		return m.IsWalkableFor(p, person) && !m.IsObviousHazardAt(p)
	}
}
func (m *GridMap[ActorType, ItemType, ObjectType]) IsWalkable(p geometry.Point) bool {
	if !m.Contains(p) {
		return false
	}
	var noActor ActorType
	if m.IsObjectAt(p) && (!m.objectAt(p).IsWalkable(noActor)) {
		return false
	}
	cellAt := m.GetCell(p)
	return cellAt.TileType.IsWalkable
}
func (m *GridMap[ActorType, ItemType, ObjectType]) IsObviousHazardAt(p geometry.Point) bool {
	return m.IsDamagingTileAt(p)
}
func (m *GridMap[ActorType, ItemType, ObjectType]) IsWalkableFor(p geometry.Point, person ActorType) bool {
	if !m.Contains(p) {
		return false
	}

	if m.IsActorAt(p) && m.actorAt(p) != person {
		return false
	}

	if m.noClip {
		return true
	}

	if m.IsObjectAt(p) && (!m.objectAt(p).IsWalkable(person)) {
		return false
	}

	cellAt := m.GetCell(p)
	return cellAt.TileType.IsWalkable

}
func (m *GridMap[ActorType, ItemType, ObjectType]) IsInHostileZone(person ActorType) bool {
	ourPos := person.MapPosition()
	zoneAt := m.ZoneAt(ourPos)
	if zoneAt == nil || zoneAt.IsPublic() {
		return false
	}
	return zoneAt.IsHighSecurity()
}

func (m *GridMap[ActorType, ItemType, ObjectType]) CurrentlyPassableForActor(person ActorType) func(p geometry.Point) bool {
	return func(p geometry.Point) bool {
		if !m.Contains(p) ||
			(m.IsActorAt(p) && m.actorAt(p) != person) {
			return false
		}
		return m.IsWalkableFor(p, person)
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsExplored(pos geometry.Point) bool {
	return m.GetCell(pos).IsExplored
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SetExplored(pos geometry.Point) {
	if !m.Contains(pos) {
		return
	}
	m.cells[pos.X+pos.Y*m.mapWidth].IsExplored = true
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SetLit(pos geometry.Point, value bool) {
	if !m.Contains(pos) {
		return
	}
	m.cells[pos.X+pos.Y*m.mapWidth].IsLit = value
}
func (m *GridMap[ActorType, ItemType, ObjectType]) GetAllStairs() []geometry.Point {
	return m.GetPositionsByFilter(func(cell *MapCell[ActorType, ItemType, ObjectType]) bool {
		return cell.TileType.Special == SpecialTileStairsUp || cell.TileType.Special == SpecialTileStairsDown
	})
}
func (m *GridMap[ActorType, ItemType, ObjectType]) GetAllSpecialTilePositions(tile SpecialTileType) []geometry.Point {
	result := make([]geometry.Point, 0)
	for index, c := range m.cells {
		if c.TileType.Special == tile {
			x := index % m.mapWidth
			y := index / m.mapWidth
			result = append(result, geometry.Point{X: x, Y: y})
		}
	}
	return result
}
func (m *GridMap[ActorType, ItemType, ObjectType]) GetPositionsByFilter(filter func(tile *MapCell[ActorType, ItemType, ObjectType]) bool) []geometry.Point {
	result := make([]geometry.Point, 0)
	for index, c := range m.cells {
		if filter(&c) {
			x := index % m.mapWidth
			y := index / m.mapWidth
			result = append(result, geometry.Point{X: x, Y: y})
		}
	}
	return result
}
func (m *GridMap[ActorType, ItemType, ObjectType]) GetNeighborWithSpecial(pos geometry.Point, specialType SpecialTileType) geometry.Point {
	neighbors := m.GetAllCardinalNeighbors(pos)
	for _, n := range neighbors {
		if m.GetCell(n).TileType.Special == specialType {
			return n
		}
	}
	return pos
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SwapDownedPositions(downedActorOne ActorType, downedActorTwo ActorType) {
	posTwo := downedActorTwo.MapPosition()
	posOne := downedActorOne.MapPosition()
	downedActorOne.SetMapPosition(posTwo)
	downedActorTwo.SetMapPosition(posOne)
	m.cells[posOne.X+posOne.Y*m.mapWidth] = m.cells[posOne.X+posOne.Y*m.mapWidth].WithDownedActor(downedActorTwo)
	m.cells[posTwo.X+posTwo.Y*m.mapWidth] = m.cells[posTwo.X+posTwo.Y*m.mapWidth].WithDownedActor(downedActorOne)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SwapPositions(actorOne ActorType, actorTwo ActorType) {
	posTwo := actorTwo.MapPosition()
	posOne := actorOne.MapPosition()
	actorOne.SetMapPosition(posTwo)
	actorTwo.SetMapPosition(posOne)
	m.cells[posOne.X+posOne.Y*m.mapWidth] = m.cells[posOne.X+posOne.Y*m.mapWidth].WithActor(actorTwo)
	m.cells[posTwo.X+posTwo.Y*m.mapWidth] = m.cells[posTwo.X+posTwo.Y*m.mapWidth].WithActor(actorOne)
}

func NewZoneMap(zone *ZoneInfo, width int, height int) []*ZoneInfo {
	zoneMap := make([]*ZoneInfo, width*height)
	for i := 0; i < width*height; i++ {
		zoneMap[i] = zone
	}
	return zoneMap
}

type MapPather struct {
	neighborPredicate func(pos geometry.Point) bool
	allNeighbors      func(pos geometry.Point) []geometry.Point
	pathCostFunc      func(from geometry.Point, to geometry.Point) int
}

func (m MapPather) Neighbors(point geometry.Point) []geometry.Point {
	neighbors := make([]geometry.Point, 0)
	for _, p := range m.allNeighbors(point) {
		if m.neighborPredicate(p) {
			neighbors = append(neighbors, p)
		}
	}
	return neighbors
}
func (m MapPather) Cost(from geometry.Point, to geometry.Point) int {
	return m.pathCostFunc(from, to)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsTileWithSpecialAt(pos geometry.Point, special SpecialTileType) bool {
	return m.GetCell(pos).TileType.Special == special
}

func (m *GridMap[ActorType, ItemType, ObjectType]) MoveDownedActor(actor ActorType, newPos geometry.Point) {
	if m.cells[newPos.Y*m.mapWidth+newPos.X].DownedActor != nil {
		return
	}
	m.cells[actor.MapPosition().Y*m.mapWidth+actor.MapPosition().X] = m.cells[actor.MapPosition().Y*m.mapWidth+actor.MapPosition().X].WithDownedActorHereRemoved(actor)
	actor.SetMapPosition(newPos)
	m.cells[newPos.Y*m.mapWidth+newPos.X] = m.cells[newPos.Y*m.mapWidth+newPos.X].WithDownedActor(actor)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) RemoveDownedActor(actor ActorType) bool {
	m.cells[actor.MapPosition().Y*m.mapWidth+actor.MapPosition().X] = m.cells[actor.MapPosition().Y*m.mapWidth+actor.MapPosition().X].WithDownedActorHereRemoved(actor)
	for i := len(m.allDownedActors) - 1; i >= 0; i-- {
		if m.allDownedActors[i] == actor {
			m.allDownedActors = append(m.allDownedActors[:i], m.allDownedActors[i+1:]...)
			return true
		}
	}
	return false
}

func (m *GridMap[ActorType, ItemType, ObjectType]) Apply(f func(cell MapCell[ActorType, ItemType, ObjectType]) MapCell[ActorType, ItemType, ObjectType]) {
	for i, cell := range m.cells {
		m.cells[i] = f(cell)
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsNamedLocationAt(positionInWorld geometry.Point) bool {
	for _, loc := range m.namedLocations {
		if loc == positionInWorld {
			return true
		}
	}
	return false
}
func (m *GridMap[ActorType, ItemType, ObjectType]) ZoneNames() []string {
	result := make([]string, 0)
	for _, zone := range m.listOfZones {
		result = append(result, zone.Name)
	}
	return result
}

func (m *GridMap[ActorType, ItemType, ObjectType]) Resize(width int, height int, emptyTile Tile) {
	oldWidth := m.mapWidth
	oldHeight := m.mapHeight

	newCells := make([]MapCell[ActorType, ItemType, ObjectType], width*height)
	newZoneMap := make([]*ZoneInfo, width*height)
	for i := 0; i < width*height; i++ {
		newZoneMap[i] = m.listOfZones[0]
	}
	for i := 0; i < width*height; i++ {
		newCells[i] = MapCell[ActorType, ItemType, ObjectType]{
			TileType:   emptyTile,
			IsExplored: true,
		}
	}

	// copy over the old cells into the center of the new map
	for y := 0; y < oldHeight; y++ {
		if y >= height {
			break
		}
		for x := 0; x < oldWidth; x++ {
			if x >= width {
				break
			}
			destIndex := y*width + x
			srcIndex := y*oldWidth + x
			newCells[destIndex] = m.cells[srcIndex]
			newZoneMap[destIndex] = m.zoneMap[srcIndex]
		}
	}

	m.cells = newCells
	m.zoneMap = newZoneMap
	m.mapWidth = width
	m.mapHeight = height
}
func (m *GridMap[ActorType, ItemType, ObjectType]) MapSize() geometry.Point {
	return geometry.Point{X: m.mapWidth, Y: m.mapHeight}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) NeighborsAll(pos geometry.Point, filter func(p geometry.Point) bool) []geometry.Point {
	neighbors := geometry.Neighbors{}
	return neighbors.All(pos, filter)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) NeighborsCardinal(pos geometry.Point, filter func(p geometry.Point) bool) []geometry.Point {
	neighbors := geometry.Neighbors{}
	return neighbors.Cardinal(pos, filter)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsPassableForProjectile(p geometry.Point) bool {
	isTileWalkable := m.IsTileWalkable(p)
	isActorOnTile := m.IsActorAt(p)
	objectOnTile, isObjectOnTile := m.TryGetObjectAt(p)
	isObjectBlocking := isObjectOnTile && !objectOnTile.IsPassableForProjectile()
	return isTileWalkable && !isActorOnTile && !isObjectBlocking
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SetAllExplored() {
	for y := 0; y < m.mapHeight; y++ {
		for x := 0; x < m.mapWidth; x++ {
			m.cells[y*m.mapWidth+x].IsExplored = true
		}
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) RandomSpawnPosition() geometry.Point {
	for {
		x := rand.Intn(m.mapWidth)
		y := rand.Intn(m.mapHeight)
		pos := geometry.Point{X: x, Y: y}
		if m.IsCurrentlyPassable(pos) {
			return pos
		}
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) AddNamedLocation(name string, point geometry.Point) {
	m.namedLocations[name] = point
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetNamedLocation(name string) geometry.Point {
	return m.namedLocations[name]
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetNamedLocationByPos(pos geometry.Point) string {
	for name, location := range m.namedLocations {
		if location == pos {
			return name
		}
	}
	return ""
}

func (m *GridMap[ActorType, ItemType, ObjectType]) RenameLocation(oldName string, newName string) {
	if pos, ok := m.namedLocations[oldName]; ok {
		delete(m.namedLocations, oldName)
		m.namedLocations[newName] = pos
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) RemoveNamedLocation(namedLocation string) {
	delete(m.namedLocations, namedLocation)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsDamagingTileAt(p geometry.Point) bool {
	return m.CellAt(p).TileType.Special == SpecialTileDamaging
}

func (m *GridMap[ActorType, ItemType, ObjectType]) RandomPosAround(pos geometry.Point) geometry.Point {
	neighbors := m.NeighborsAll(pos, func(p geometry.Point) bool {
		return m.Contains(p)
	})
	if len(neighbors) == 0 {
		return pos
	}
	neighbors = append(neighbors, pos)
	return neighbors[rand.Intn(len(neighbors))]
}

// actorAt, downedActorAt, objectAt and itemAt dereference the cell: only call them after the matching IsXAt.
// Other packages use TryGetXAt.
func (m *GridMap[ActorType, ItemType, ObjectType]) actorAt(p geometry.Point) ActorType {
	return *m.cells[p.X+p.Y*m.mapWidth].Actor
}

func (m *GridMap[ActorType, ItemType, ObjectType]) downedActorAt(p geometry.Point) ActorType {
	return *m.cells[p.X+p.Y*m.mapWidth].DownedActor
}

func (m *GridMap[ActorType, ItemType, ObjectType]) objectAt(p geometry.Point) ObjectType {
	return *m.cells[p.X+p.Y*m.mapWidth].Object
}

func (m *GridMap[ActorType, ItemType, ObjectType]) itemAt(p geometry.Point) ItemType {
	return *m.cells[p.X+p.Y*m.mapWidth].Item
}

func (m *GridMap[ActorType, ItemType, ObjectType]) TryGetActorAt(pos geometry.Point) (ActorType, bool) {
	var none ActorType
	if !m.IsActorAt(pos) {
		return none, false
	}
	return m.actorAt(pos), true
}

func (m *GridMap[ActorType, ItemType, ObjectType]) TryGetDownedActorAt(pos geometry.Point) (ActorType, bool) {
	var none ActorType
	if !m.IsDownedActorAt(pos) {
		return none, false
	}
	return m.downedActorAt(pos), true
}

func (m *GridMap[ActorType, ItemType, ObjectType]) TryGetObjectAt(pos geometry.Point) (ObjectType, bool) {
	var none ObjectType
	if !m.IsObjectAt(pos) {
		return none, false
	}
	return m.objectAt(pos), true
}

func (m *GridMap[ActorType, ItemType, ObjectType]) TryGetItemAt(pos geometry.Point) (ItemType, bool) {
	var none ItemType
	if !m.IsItemAt(pos) {
		return none, false
	}
	return m.itemAt(pos), true
}

// AddActor puts an actor on the map at spawnPos and reports whether that worked. Spawning is not walking: an
// actor may start on a cell whose object would block a step into it (D1 places its monsters before its objects),
// but never on top of another actor and never off the map. Placing it here is what puts it in allActors, so an
// actor that is on the map always owns exactly one cell, and one cell never holds two actors. Going through
// MoveActor instead used to append the actor and then silently refuse the move, leaving a monster that acted and
// was drawn but that nothing could find by its position.
func (m *GridMap[ActorType, ItemType, ObjectType]) AddActor(actor ActorType, spawnPos geometry.Point) bool {
	if !m.Contains(spawnPos) || m.IsActorAt(spawnPos) {
		return false
	}
	m.allActors = append(m.allActors, actor)
	if from := actor.MapPosition(); m.Contains(from) {
		m.cells[from.X+from.Y*m.mapWidth] = m.cells[from.X+from.Y*m.mapWidth].WithActorHereRemoved(actor)
	}
	actor.SetMapPosition(spawnPos)
	m.cells[spawnPos.X+spawnPos.Y*m.mapWidth] = m.cells[spawnPos.X+spawnPos.Y*m.mapWidth].WithActor(actor)
	return true
}

func (m *GridMap[ActorType, ItemType, ObjectType]) AddDownedActor(actor ActorType, spawnPos geometry.Point) {
	if m.IsDownedActorAt(spawnPos) {
		return
	}
	m.allDownedActors = append(m.allDownedActors, actor)
	m.MoveDownedActor(actor, spawnPos)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) AddObject(object ObjectType, spawnPos geometry.Point) {
	if m.IsObjectAt(spawnPos) {
		return
	}
	m.allObjects = append(m.allObjects, object)
	m.MoveObject(object, spawnPos)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) AddItem(item ItemType, spawnPos geometry.Point) {
	if m.IsItemAt(spawnPos) {
		return
	}
	m.allItems = append(m.allItems, item)
	m.MoveItem(item, spawnPos)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) UpdateFieldOfView(fov *geometry.FOV, fovPosition geometry.Point, visionRange int) {
	visionRangeSquared := visionRange * visionRange
	sourceIsLit := m.IsTileLit(fovPosition)
	var fovRange = geometry.NewRect(-visionRange, -visionRange, visionRange+1, visionRange+1)
	fov.SetRange(fovRange.Add(fovPosition).Intersect(geometry.NewRect(0, 0, m.mapWidth, m.mapHeight)))

	fov.SSCVisionMap(fovPosition, visionRange, false, func(p geometry.Point) bool {
		if !m.Contains(p) || (sourceIsLit && !m.IsTileLit(p)) {
			return false
		}
		return m.IsTransparent(p) && geometry.DistanceSquared(p, fovPosition) <= visionRangeSquared
	})
}

func (m *GridMap[ActorType, ItemType, ObjectType]) ToggleNoClip() bool {
	m.noClip = !m.noClip
	return m.noClip
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SetSecretDoorAt(pos geometry.Point) {
	m.secretDoors[pos] = true
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsSecretDoorAt(neighbor geometry.Point) bool {
	if _, ok := m.secretDoors[neighbor]; ok {
		return true
	}
	return false
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SetName(name string) {
	m.name = name
}

func (m *GridMap[ActorType, ItemType, ObjectType]) AddTransitionAt(pos geometry.Point, transition Transition) {
	m.transitionMap[pos] = transition
}

func (m *GridMap[ActorType, ItemType, ObjectType]) RemoveTransitionAt(pos geometry.Point) {
	delete(m.transitionMap, pos)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetTransitionAt(pos geometry.Point) (Transition, bool) {
	transition, ok := m.transitionMap[pos]
	return transition, ok

}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetName() string {
	return m.name
}

func (m *GridMap[ActorType, ItemType, ObjectType]) WriteTiles(out io.Writer) {
	for _, cell := range m.cells {
		cell.TileType.ToBinary(out)
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) ReadTiles(in io.Reader) {
	for i := range m.cells {
		m.cells[i].TileType = NewTileFromBinary(in)
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) Transitions() map[geometry.Point]Transition {
	return m.transitionMap
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SecretDoors() map[geometry.Point]bool {
	return m.secretDoors
}

func (m *GridMap[ActorType, ItemType, ObjectType]) AddNamedRegion(name string, region geometry.Rect) {
	m.namedRects[name] = region
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetNamedRegion(name string) geometry.Rect {
	return m.namedRects[name]
}

type Trigger struct {
	Name    string
	Bounds  geometry.Rect
	OneShot bool
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetNamedTriggerAt(pos geometry.Point) (Trigger, bool) {
	for _, trigger := range m.namedTrigger {
		if trigger.Bounds.Contains(pos) {
			return trigger, true
		}
	}
	return Trigger{}, false
}
func (m *GridMap[ActorType, ItemType, ObjectType]) SetTileIcon(pos geometry.Point, index int32) {
	m.cells[pos.Y*m.mapWidth+pos.X].TileType.DefinedIcon = index
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetTileIconAt(pos geometry.Point) int32 {
	return m.cells[pos.Y*m.mapWidth+pos.X].TileType.DefinedIcon
}

func (m *GridMap[ActorType, ItemType, ObjectType]) RemoveNamedRegion(regionName string) {
	delete(m.namedRects, regionName)
}
func (m *GridMap[ActorType, ItemType, ObjectType]) RemoveNamedTrigger(triggerName string) {
	delete(m.namedTrigger, triggerName)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) AddNamedTrigger(name string, rect Trigger) {
	m.namedTrigger[name] = rect
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SetDisplayName(name string) {
	m.displayName = name
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetDisplayName() string {
	return m.displayName
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetFilteredActorsInRadius(location geometry.Point, radius int, filter func(actor ActorType) bool) []ActorType {
	if (radius * radius) < len(m.allActors) {
		return m.iterateMapForActors(location, radius, filter)
	} else {
		return m.iterateActors(location, radius, filter)
	}
}
func (m *GridMap[ActorType, ItemType, ObjectType]) iterateActors(location geometry.Point, radius int, filter func(actor ActorType) bool) []ActorType {
	result := make([]ActorType, 0)
	for _, actor := range m.allActors {
		if geometry.DistanceManhattan(location, actor.MapPosition()) <= radius && filter(actor) {
			result = append(result, actor)
		}
	}
	return result
}

func (m *GridMap[ActorType, ItemType, ObjectType]) iterateMapForActors(location geometry.Point, radius int, filter func(actor ActorType) bool) []ActorType {
	result := make([]ActorType, 0)
	for y := location.Y - radius; y <= location.Y+radius; y++ {
		for x := location.X - radius; x <= location.X+radius; x++ {
			pos := geometry.Point{X: x, Y: y}
			if actorAt, ok := m.TryGetActorAt(pos); ok {
				if filter(actorAt) {
					result = append(result, actorAt)
				}
			}
		}
	}
	return result
}

func (m *GridMap[ActorType, ItemType, ObjectType]) AddNamedPath(name string, pathFromRootEntity []geometry.Point) {
	m.namedPaths[name] = pathFromRootEntity
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetNamedPath(name string) []geometry.Point {
	return m.namedPaths[name]
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SetNamedLocations(locations []NamedLocation) {
	m.namedLocations = make(map[string]geometry.Point)
	for _, location := range locations {
		m.namedLocations[location.LocationName] = location.Pos
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetRandomLocation(filter func(location geometry.Point) bool) (geometry.Point, bool) {
	endCounter := 1000000
	for {
		x := rand.Intn(m.mapWidth)
		y := rand.Intn(m.mapHeight)
		pos := geometry.Point{X: x, Y: y}
		if filter(pos) {
			return pos, true
		}
		endCounter--
		if endCounter <= 0 {
			return geometry.Point{}, false
		}
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) CanPlaceActorHere(pos geometry.Point) bool {
	return m.IsWalkable(pos) && !m.IsActorAt(pos) && !m.IsObviousHazardAt(pos)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) CanPlaceItemHere(pos geometry.Point) bool {
	return m.IsWalkable(pos) && !m.IsItemAt(pos) && m.IsSpecialAt(pos, SpecialTileNone)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) SetMetaString(info []string) {
	m.metaInfoString = info
}
func (m *GridMap[ActorType, ItemType, ObjectType]) GetMetaString() []string {
	return m.metaInfoString
}

func (m *GridMap[ActorType, ItemType, ObjectType]) AddToMetaInfo(infoLine string) {
	m.metaInfoString = append(m.metaInfoString, infoLine)
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsStairsAt(point geometry.Point) bool {
	if !m.Contains(point) {
		return false
	}
	special := m.GetCell(point).TileType.Special
	return special == SpecialTileStairsUp || special == SpecialTileStairsDown
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetFilteredActors(f func(actor ActorType) bool) []ActorType {
	var result []ActorType
	for _, actor := range m.allActors {
		if f(actor) {
			result = append(result, actor)
		}
	}
	return result
}

func (m *GridMap[ActorType, ItemType, ObjectType]) MoveDistance(pOne geometry.Point, pTwo geometry.Point) int {
	if m.cardinalMovementOnly {
		return geometry.DistanceManhattan(pOne, pTwo)
	}
	return geometry.DistanceChebyshev(pOne, pTwo)
}
func (m *GridMap[ActorType, ItemType, ObjectType]) ApplyToActorsAt(positions []geometry.Point, applyFunc func(actor ActorType)) {
	for _, pos := range positions {
		if actor, ok := m.TryGetActorAt(pos); ok {
			applyFunc(actor)
		}
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) AddDecal(pos geometry.Point, decalIcon int32) {
	m.decals[pos] = decalIcon
}

// Decals are every decal on the map by cell; the map's own, not a copy.
func (m *GridMap[ActorType, ItemType, ObjectType]) Decals() map[geometry.Point]int32 { return m.decals }

func (m *GridMap[ActorType, ItemType, ObjectType]) GetDecal(pos geometry.Point) (int32, bool) {
	decal, ok := m.decals[pos]
	if !ok {
		return -1, false
	}
	return decal, true
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetWidth() int {
	return m.mapWidth
}

func (m *GridMap[ActorType, ItemType, ObjectType]) GetHeight() int {
	return m.mapHeight
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsTileLit(pos geometry.Point) bool {
	if !m.Contains(pos) {
		return false
	}
	return m.GetCell(pos).IsLit
}

func (m *GridMap[ActorType, ItemType, ObjectType]) IsBorderWall(point geometry.Point) bool {
	// pos is on the edge of the map
	if point.X == 0 || point.X == m.mapWidth-1 || point.Y == 0 || point.Y == m.mapHeight-1 {
		return true
	}
	return false
}

type JumpOverInfo struct {
	Sprint     []geometry.Point
	Jump       []geometry.Point
	JumpedPos  geometry.Point
	ActorFound bool
}

type SprintTooInfo struct {
	Sprint         []geometry.Point
	SprintEndPos   geometry.Point
	EmptyTileFound bool
}
