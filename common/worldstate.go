package common

import (
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/renderer"
)

type State struct {
	currentMap          *gridmap.GridMap[*Actor, Item, Object]
	townMaps            map[string]*gridmap.GridMap[*Actor, Item, Object]
	dungeonMaps         map[string]*gridmap.GridMap[*Actor, Item, Object]
	player              *Actor
	camera              Camera
	logger              func(string)
	playerTookTurn      func(int)
	onActorSpawned      func(*Actor)
	visibility          *Visibility
	townMapSpawn        geometry.Point
	deepestDungeonLevel int
	flags               *Flags
}

func NewState() State {
	return State{
		dungeonMaps:         make(map[string]*gridmap.GridMap[*Actor, Item, Object]),
		deepestDungeonLevel: 1,
		townMaps:            make(map[string]*gridmap.GridMap[*Actor, Item, Object]),
		flags:               NewFlags(),
	}
}

func (s *State) GetVisibility() *Visibility {
	return s.visibility
}

func (s *State) PlayerTookTurn(autUsed int) {
	s.playerTookTurn(autUsed)
}

func (s *State) Log(message string) {
	s.logger(message)
}

func (s *State) GetCamera() Camera {
	return s.camera
}

func (s *State) GetPlayer() *Actor {
	return s.player
}

func (s *State) GetMap() *gridmap.GridMap[*Actor, Item, Object] {
	return s.currentMap
}

func (s *State) SetPlayer(actor *Actor) {
	s.player = actor
}

func (s *State) SetMap(loadedMap *gridmap.GridMap[*Actor, Item, Object]) {
	s.currentMap = loadedMap
}

func (s *State) SetCamera(renderer *renderer.MapRenderer) {
	s.camera = renderer
}

func (s *State) SetLogger(logger func(string)) {
	s.logger = logger
}

func (s *State) SetPlayerTurnHandler(playerTookTurn func(int)) {
	s.playerTookTurn = playerTookTurn
}
func (s *State) SetActorSpawnHandler(onActorSpawned func(*Actor)) {
	s.onActorSpawned = onActorSpawned
}
func (s *State) InitVisibility() {
	s.visibility = NewVisibility()
}

func (s *State) SetTownMap(currentMap *gridmap.GridMap[*Actor, Item, Object]) {
	s.townMaps[currentMap.GetName()] = currentMap
}

func (s *State) SetDungeonMap(name string, dungeonMap *gridmap.GridMap[*Actor, Item, Object]) {
	s.dungeonMaps[name] = dungeonMap
}

func (s *State) GetTownMap(name string) *gridmap.GridMap[*Actor, Item, Object] {
	if townMap, exists := s.townMaps[name]; exists {
		return townMap
	}
	return nil
}

func (s *State) GetDungeonMap(name string) *gridmap.GridMap[*Actor, Item, Object] {
	return s.dungeonMaps[name]
}

func (s *State) HasDungeonMap(name string) bool {
	_, exists := s.dungeonMaps[name]
	return exists
}

func (s *State) SetTownSpawn(spawn geometry.Point) {
	s.townMapSpawn = spawn
}
func (s *State) GetTownSpawn() geometry.Point {
	return s.townMapSpawn

}
func (s *State) SetLastDungeonLevel(level int) {
	s.deepestDungeonLevel = level
}

func (s *State) GetLastDungeonLevel() int {
	return s.deepestDungeonLevel
}

func (s *State) OnActorSpawned(actor *Actor) {
	s.onActorSpawned(actor)
}

func (s *State) HasTownMap(mapName string) bool {
	_, exists := s.townMaps[mapName]
	return exists
}

func (s *State) GetFlags() *Flags {
	return s.flags
}
