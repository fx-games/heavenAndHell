package common

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/ldtk"
	"rpg/renderer"
	"rpg/stats"
	"rpg/util"
)

type PlayerActions interface {
	EndTurn(autTaken int)

	ManualMovePlayer(direction geometry.Point)

	DropItem(item *Item)
	EquipItem(item *Item)
	UnequipItem(item *Item)

	ActivateUntargetedAction(action *UntargetedAction, source EffectSource)
	BeginActionTargeting(element *TargetedAction)

	StartAutoExplore()
	StartPlayerAutoMovement(toTarget geometry.Point)
	CancelAutoMove()

	GetContextActionsForItem(item *Item) []util.MenuItem
	PickUpItem()
	AimedRangedAttack()
	AimedThrow()
	QuickRangedAttack()
	CastLastSpellAtNearest()
	RestUntilHealed()
	AutoAttack()
	ShowActorInfoAt(mapPos geometry.Point)
	ShowObjectInfoAt(mapPos geometry.Point)
	RangedAttackAt(mapPos geometry.Point) bool
}
type State struct {
	currentMap     *gridmap.GridMap[*Actor, *Item, Object]
	townMaps       map[string]*gridmap.GridMap[*Actor, *Item, Object]
	dungeonMaps    map[string]*gridmap.GridMap[*Actor, *Item, Object]
	d1Portal       *D1Portal
	awayTowners    map[VendorType]awayTowner // Hellfire's towners InitTowners wouldn't create now
	player         *Actor
	camera         Camera
	msg            func(string)
	playerTookTurn func(int)

	onMapChanged func(oldName, newName string)

	visibility          *Visibility
	townMapSpawn        geometry.Point
	deepestDungeonLevel int
	flags               *Flags
	playerStashInTown   *Inventory
	playerDijkstraMap   map[geometry.Point]int
	timekeeper          *TimeKeeper
	actions             *Broker
	animator            Animator

	config Configuration

	factory *Factory
	mapGen  *MapGenerator
	// maps
	ldtkProject *ldtk.Project

	SelectedClass defs.LoadoutDefinition
	worldTexture  renderer.TextureAtlas

	glowCache map[*gridmap.GridMap[*Actor, *Item, Object]]map[geometry.Point]d1Glow // decorative light per map (d1GlowAt)
}

func (s *State) GetWorldTexture() renderer.TextureAtlas {
	return s.worldTexture
}
func (s *State) GetPlayerStashInTown() *Inventory {
	return s.playerStashInTown
}
func (s *State) GetCurrentDungeonLevel() int {
	currentMapName := s.GetMap().GetName()
	if defs.IsTownLevel(currentMapName) {
		return 0
	}
	return LevelFromName(currentMapName)
}
func NewState(ldtkProject *ldtk.Project) State {

	return State{
		dungeonMaps:         make(map[string]*gridmap.GridMap[*Actor, *Item, Object]),
		deepestDungeonLevel: 1,
		townMaps:            make(map[string]*gridmap.GridMap[*Actor, *Item, Object]),
		flags:               NewFlags(),
		playerStashInTown:   NewInventory(32),
		ldtkProject:         ldtkProject,
		visibility:          NewVisibility(),
	}
}
func (s *State) SetAnimator(animator Animator) {
	s.animator = animator
}
func (s *State) SetOnMapChanged(onMapChanged func(oldName, newName string)) {
	s.onMapChanged = onMapChanged
}
func (s *State) SetMapGenerator(mapGen *MapGenerator) {
	s.mapGen = mapGen
}

// D1Quests reports which of D1's quests this game's seed picked (InitQuests): QuestNotAvail for the ones it
// disabled, so callers can tell an absent quest boss from a missing one.
func (s *State) D1Quests() *d1.Quests {
	return s.mapGen.d1Quests
}
func (s *State) GetVisibility() *Visibility {
	return s.visibility
}

func (s *State) PlayerTookTurn(autUsed int) {
	s.playerTookTurn(autUsed)
}

func (s *State) TransitionTo(dest gridmap.Transition) {
	actionBroker := s.actions
	actionBroker.Execute(NewTransitionAction(AbstractCause("Quick Travel"), dest))
	actionBroker.Execute(NewUpdateFoVAction(AbstractCause("Quick Travel"), s.GetPlayer()))
}
func (s *State) Log(message string) { // Maybe remove this?
	s.msg(message)
}

func (s *State) GetPlayer() *Actor {
	return s.player
}

// IsStairwayReachable returns true, if the player can reach all the stairs in the current level.
// Assumes that the player's dijkstra map is up-to-date.
// NOTE: This might still be misleading when applied to levels where
// POIs must be reached which are not the stairs.
// eg.: Vendors in Town, Quest Locations & End Bosses in Dungeons
func (s *State) IsStairwayReachable() bool {
	allStairs := s.GetMap().GetAllStairs()
	return s.IsReachableByPlayer(allStairs)
}

func (s *State) IsReachableByPlayer(positionsOfInterest []geometry.Point) bool {
	for _, stairs := range positionsOfInterest {
		if _, exists := s.playerDijkstraMap[stairs]; !exists {
			return false
		}
	}
	return true
}

func (s *State) GetMap() *gridmap.GridMap[*Actor, *Item, Object] {
	return s.currentMap
}

func (s *State) SetPlayer(actor *Actor) {
	s.player = actor
}

func (s *State) SetCurrentMap(loadedMap *gridmap.GridMap[*Actor, *Item, Object]) {
	s.currentMap = loadedMap
}

func (s *State) SetCamera(renderer Camera) {
	s.camera = renderer
}

func (s *State) SetPlayerMessage(msg func(string)) {
	s.msg = msg
}

func (s *State) SetPlayerTurnHandler(playerTookTurn func(int)) {
	s.playerTookTurn = playerTookTurn
}
func (s *State) SetTownMap(currentMap *gridmap.GridMap[*Actor, *Item, Object]) {
	s.townMaps[currentMap.GetName()] = currentMap
}

func (s *State) SetDungeonMap(name string, dungeonMap *gridmap.GridMap[*Actor, *Item, Object]) {
	s.dungeonMaps[name] = dungeonMap
}

func (s *State) GetTownMap(name string) *gridmap.GridMap[*Actor, *Item, Object] {
	if townMap, exists := s.townMaps[name]; exists {
		return townMap
	}
	return nil
}

func (s *State) GetDungeonMap(name string) *gridmap.GridMap[*Actor, *Item, Object] {
	return s.dungeonMaps[name]
}

func (s *State) DungeonMaps() map[string]*gridmap.GridMap[*Actor, *Item, Object] {
	return s.dungeonMaps
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
	s.timekeeper.AddTimed(actor)
}

func (s *State) HasTownMap(mapName string) bool {
	_, exists := s.townMaps[mapName]
	return exists
}

func (s *State) GetFlags() *Flags {
	return s.flags
}

func (s *State) IsPlayerInTown() bool {
	mapName := s.GetMap().GetName()
	return defs.IsTownLevel(mapName)
}

func (s *State) PlayerStandsInLitArea() bool {
	currentMap := s.GetMap()
	isPlayerLit := currentMap.IsTileLit(s.GetPlayer().MapPosition())
	return isPlayerLit || s.IsDaylight()
}

func (s *State) PlayerCanSeeNeighbors() bool {
	return s.GetPlayer().hero != nil || s.GetPlayer().GetEquipment().HasLightSourceEquipped() || s.PlayerStandsInLitArea()
}
func (s *State) IsDaylight() bool {
	return s.IsPlayerInTown() && !s.GetFlags().IsSet("town_changed_to_night")
}

// FlagBeenUnderground is set the first time the player enters a dungeon level. Until then the town draws gloomy,
// so the church's red light carries the opening; coming back up, the town is bright daylight.
const FlagBeenUnderground = "been_underground"

func (s *State) GetPlayerVisionRange() int {
	actor := s.GetPlayer()
	visionRange := actor.GetStats().GetAttribute(stats.VisionRange) // used in town (=day light)

	playerInDungeon := !s.IsPlayerInTown()
	if playerInDungeon {
		if s.GetMap().IsTileLit(actor.MapPosition()) {
			return visionRange
		}
		if actor.hero != nil { // D1: light radius 10 plus item bonuses, no torch needed
			return actor.HeroSheet().LightRad
		}
		equipment := actor.GetEquipment()
		if equipment.HasLightSourceEquipped() {
			lightRadius := actor.GetStats().GetAttribute(stats.LightRadius) // used in dungeons (=darkness)
			visionRange = lightRadius
		} else {
			visionRange = 0
		}
	} else if s.GetFlags().IsSet("town_changed_to_night") {
		visionRange = visionRange / 2
	}
	return visionRange
}

// LitByVision reports a cell the Vision spell exposes but the player cannot actually see. What stands there is
// drawn unlit, which makes it nearly invisible, so the map draws it at full brightness instead.
func (s *State) LitByVision(mapPos geometry.Point) bool {
	p := s.GetPlayer()
	return p != nil && p.HasStatusEffect(StatusVision) && !p.CanSee(mapPos)
}

// ApplyLighting shades what is drawn at mapPos: distance from the player, lit and unlit dungeon rooms, torches,
// and on top of all of it the church's red light, which has to show in the town's daylight too.
func (s *State) ApplyLighting(mapPos geometry.Point, drawColor color.RGBA, worldTick uint64) color.RGBA {
	return s.applyRedLight(mapPos, s.applyLighting(mapPos, drawColor, worldTick), worldTick)
}

// applyRedLight tints a cell inside the demon-possessed church (and the ground the light falls on outside its
// door) red, pulsing slowly, and lifts it out of the dark so the glow reads as a light source and not as shadow.
func (s *State) applyRedLight(mapPos geometry.Point, drawColor color.RGBA, worldTick uint64) color.RGBA {
	glow := s.d1GlowAt(mapPos)
	if !glow.red || glow.v <= 0 || !s.GetMap().IsExplored(mapPos) {
		return drawColor
	}
	pulse := 0.85 + 0.15*math.Sin(float64(worldTick)/45.0)
	// red light: the red channel is lifted above what the cell would otherwise show, the other two pushed down
	lit := color.RGBA{
		R: uint8(min(float64(drawColor.R)+190*glow.v, 255)),
		G: uint8(float64(drawColor.G) * 0.25),
		B: uint8(float64(drawColor.B) * 0.18),
		A: drawColor.A,
	}
	return lerpRGBA(drawColor, lit, min(glow.v*1.8, 1.0)*pulse)
}

// lerpRGBA mixes two colors; util.LerpColor underflows on a channel that goes down.
func lerpRGBA(from, to color.RGBA, t float64) color.RGBA {
	c := func(a, b uint8) uint8 { return uint8(max(min(float64(a)+(float64(b)-float64(a))*t, 255), 0)) }
	return color.RGBA{R: c(from.R, to.R), G: c(from.G, to.G), B: c(from.B, to.B), A: from.A}
}

func (s *State) applyLighting(mapPos geometry.Point, drawColor color.RGBA, worldTick uint64) color.RGBA {
	state := s
	distanceToPlayer := geometry.Distance(state.GetPlayer().MapPosition(), mapPos)
	maxDistance := float64(state.GetPlayerVisionRange() + 1)
	darkPercentage := 0.16
	lightPercentage := 1.0
	if state.IsDaylight() {
		darkPercentage = 0.75
		lightPercentage = 0.95
		if !state.GetFlags().IsSet(FlagBeenUnderground) { // gloomy first visit, before the first descent
			darkPercentage = 0.10
			lightPercentage = 0.30
		}
	}

	currentMap := state.GetMap()
	playerStandsInLitArea := state.PlayerStandsInLitArea()
	isInFieldOfView := state.GetPlayer().CanSee(mapPos)

	if !state.IsDaylight() {
		if currentMap.IsTileLit(mapPos) && currentMap.IsExplored(mapPos) {
			dungeonLitRoomLightPercentage := 0.75
			if !isInFieldOfView {
				dungeonLitRoomLightPercentage = 0.5
			}
			return multiplyColor(drawColor, dungeonLitRoomLightPercentage)
		}

		if playerStandsInLitArea && !currentMap.IsTileLit(mapPos) && currentMap.IsExplored(mapPos) {
			dungeonUnLitRoomLightPercentage := 0.25
			if !isInFieldOfView {
				dungeonUnLitRoomLightPercentage = 0.1
			}
			return multiplyColor(drawColor, dungeonUnLitRoomLightPercentage)
		}
	}
	if distanceToPlayer > maxDistance || !isInFieldOfView {
		drawColor = multiplyColor(drawColor, darkPercentage)
	} else {
		percentageLightness := float64(distanceToPlayer) / float64(maxDistance)
		percentageLightness = 1.0 - util.EaseInExpo(percentageLightness)
		if glow := s.d1GlowAt(mapPos); glow.v > 0 && !glow.red { // wall torches and braziers, each flickering out of step
			frame := util.GetLoopingFrameFromTick(worldTick+uint64(mapPos.X*7+mapPos.Y*13), d1HeroTorch.FlickerFrameDelay, len(d1HeroTorch.FlickerPattern))
			if g := glow.v * d1HeroTorch.FlickerPattern[frame]; g > percentageLightness {
				percentageLightness = g
				drawColor = multiplyColors(drawColor, d1HeroTorch.LightColor)
			}
		}
		//light := 255 // 1.0 -> 100% of original value
		//dark := 20   // 0.0 -> 12,75% of original value
		percentagOfValue := darkPercentage + ((lightPercentage - darkPercentage) * percentageLightness)
		if !playerStandsInLitArea {
			drawColor = s.mixLightSourceColor(drawColor, color.RGBA{127, 127, 127, 255}, worldTick)
		}
		drawColor = multiplyColor(drawColor, percentagOfValue)
	}
	return drawColor
}

// d1HeroTorch is the light every D1 hero carries without an item: H&H's torch color and fire flicker.
var d1HeroTorch = LightStats{LightColor: color.RGBA{R: 255, G: 223, B: 117, A: 255}, FlickerPattern: defs.GetFireFlickerPattern(), FlickerFrameDelay: 0.125}

func (s *State) mixLightSourceColor(drawColor, noLightSource color.RGBA, worldTick uint64) color.RGBA {
	state := s
	if s.GetPlayer().hero != nil {
		flicker := d1HeroTorch.FlickerPattern[util.GetLoopingFrameFromTick(worldTick, d1HeroTorch.FlickerFrameDelay, len(d1HeroTorch.FlickerPattern))]
		return multiplyColors(drawColor, multiplyColor(d1HeroTorch.LightColor, flicker))
	}
	equipment := state.GetPlayer().GetEquipment()
	if equipment.HasLightSourceEquipped() {
		lightSource := equipment.GetLightSource()
		if lightSource.HasChargesLeft() {
			lightStats := lightSource.GetLightStats()
			lightColor := lightStats.LightColor
			flickerPattern := lightStats.FlickerPattern
			delay := lightStats.FlickerFrameDelay
			flickerIndex := util.GetLoopingFrameFromTick(worldTick, delay, len(flickerPattern))
			lightColor = multiplyColor(lightColor, flickerPattern[flickerIndex])
			drawColor = multiplyColors(drawColor, lightColor)
		}
	} else {
		drawColor = multiplyColors(drawColor, noLightSource)
	}
	return drawColor
}

// GetWaterColor tints water tiles: light green in town and in its level while the Poisoned Water Supply was rolled
// for this game and isn't done yet, blue otherwise.
func (s *State) GetWaterColor() color.RGBA {
	name, q := s.GetMap().GetName(), s.QuestActive(d1.QPWater)
	sl := d1SetLevelOf(name)
	if (defs.IsTownLevel(name) || sl != nil && sl.SL == d1.SLPoisonWater) && q != d1.QuestNotAvail && q != d1.QuestDone {
		return color.RGBA{R: 199, G: 251, B: 161, A: 255}
	}
	return color.RGBA{R: 86, G: 112, B: 194, A: 255}
}

func (s *State) IsDoorAt(pos geometry.Point) (*Door, bool) {
	currentMap := s.GetMap()
	if !currentMap.IsObjectAt(pos) {
		return nil, false
	}
	objectAt := currentMap.ObjectAt(pos)
	door, isDoorHere := objectAt.(*Door)
	return door, isDoorHere
}

func (s *State) IsClosedDoorAt(pos geometry.Point) bool {
	door, isDoorHere := s.IsDoorAt(pos)
	if !isDoorHere {
		return false
	}
	return door.IsClosed()
}

func (s *State) AddBloodDecalAt(mapPos geometry.Point) {
	currentMap := s.GetMap()
	if !currentMap.IsWalkable(mapPos) || currentMap.IsObjectAt(mapPos) || currentMap.GetCell(mapPos).TileType.Special != gridmap.SpecialTileNone {
		return
	}
	icon := int32(-1)
	if existingDecal, exists := currentMap.GetDecal(mapPos); exists {
		if existingDecal == defs.IconDecalBlood5 {
			return
		}
		if existingDecal < defs.IconDecalBlood1 || existingDecal > defs.IconDecalBlood5 {
			return
		}
		icon = existingDecal + 1
	} else {
		bloodDecals := []int32{defs.IconDecalBlood1, defs.IconDecalBlood2, defs.IconDecalBlood3, defs.IconDecalBlood4, defs.IconDecalBlood5}
		icon = bloodDecals[rand.Intn(len(bloodDecals))]
	}
	currentMap.AddDecal(mapPos, icon)
}
func (s *State) updatePlayerDijkstra() {
	actor := s.GetPlayer()
	currentMap := s.GetMap()
	dijkstraMap := currentMap.GetDijkstraMapWithActorsNotBlocking(actor.MapPosition(), 10000)
	s.playerDijkstraMap = dijkstraMap
}

func (s *State) MapTransition(to gridmap.Transition) {
	if to.IsEmpty() {
		return
	}
	if s.animator != nil {
		s.animator.CancelAllAnimations()
	}
	if s.actions != nil {
		s.actions.ClearQueuedActions(func(action Action) bool {
			return true
		})
	}

	s.timekeeper.ClearTrackedBeings(func(being TimedBeing) bool {
		switch being.(type) {
		case *Actor:
			return true
		default:
			return false
		}
	})
	//oldPosOnOldMap := g.state.GetPlayer().MapPosition()
	// remove player from current map
	oldMapName := ""
	if s.GetMap() != nil && s.GetPlayer() != nil {
		s.GetMap().RemoveActor(s.GetPlayer())
		oldMapName = s.GetMap().GetName()
	}

	player := s.GetPlayer()
	if player != nil {
		player.GetStatusEffects().SetFalse(string(StatusVision))
	}

	// change map
	targetMap := to.TargetMap
	if defs.IsTownLevel(targetMap) {
		if s.HasTownMap(targetMap) {
			s.SetCurrentMap(s.GetTownMap(targetMap))
		} else {
			// load map
			townMap, atlas := s.NewGridMapFromLDTK(targetMap)
			s.SetCurrentMap(townMap)
			s.worldTexture = atlas
		}
	} else if s.HasDungeonMap(targetMap) {
		s.SetCurrentMap(s.GetDungeonMap(targetMap))
	} else {
		// generate the map and set it
		generatedMap := s.mapGen.GenerateMap(targetMap, LevelFromName(targetMap))
		s.SetDungeonMap(targetMap, generatedMap)
		s.SetCurrentMap(generatedMap)
	}
	newlyLoadedMap := s.GetMap()
	newlyLoadedMap.SetCardinalMovementOnly(s.config.CardinalMovementOnly)

	targetLocation := newlyLoadedMap.GetNamedLocation(to.TargetLocation)
	freeTargetLocations := newlyLoadedMap.GetFreeCardinalNeighbors(targetLocation)
	if len(freeTargetLocations) == 0 { // stairs in a cave corner, or barrels and monsters all around
		freeTargetLocations = freeCellsNear(newlyLoadedMap, targetLocation, 1, newlyLoadedMap.CanPlaceActorHere)
	}

	if len(freeTargetLocations) == 0 {
		newlyLoadedMap.PrintStairsAndActors()
		newlyLoadedMap.PrintWithHighlight(targetLocation)
		panic(fmt.Sprintf("no space for player at %v", targetLocation))
	}

	freeTargetLocation := freeTargetLocations[rand.Intn(len(freeTargetLocations))]
	// add player to new map
	player.SetDrawPosition(freeTargetLocation.ToPointF())
	newlyLoadedMap.AddActor(player, freeTargetLocation)
	newlyLoadedMap.SetExplored(targetLocation)
	for _, neighborOfStairs := range newlyLoadedMap.GetFilteredNeighbors(targetLocation, newlyLoadedMap.Contains) {
		newlyLoadedMap.SetExplored(neighborOfStairs)
	}
	for _, actor := range newlyLoadedMap.Actors() {
		if actor == player {
			continue
		}
		actor.RemoveEnergy(actor.GetEnergy())
		s.timekeeper.AddTimed(actor)
	}

	if s.onMapChanged != nil {
		s.onMapChanged(oldMapName, to.TargetMap)
	}
	//g.ShowLogMessage(fmt.Sprintf("Travel from (%d, %d) to (%d, %d)", oldPosOnOldMap.X, oldPosOnOldMap.Y, freeTargetLocation.X, freeTargetLocation.Y))
}

func (s *State) AddItemToMapWithDistribution(item *Item, pos geometry.Point) {
	currentMap := s.GetMap()
	dropPos := getDropPos(currentMap, pos)
	currentMap.AddItem(item, dropPos)
}

func (s *State) SetTimeKeeper(keeper *TimeKeeper) {
	s.timekeeper = keeper
}

func (s *State) RemoveObject(gameObject Object) {
	s.GetMap().RemoveObject(gameObject)
}

func (s *State) SpawnObject(location geometry.Point, gameObject Object) {
	s.GetMap().AddObject(gameObject, location)
}

func (s *State) GetTimeKeeper() *TimeKeeper {
	return s.timekeeper
}

func (s *State) SetActionBroker(broker *Broker) {
	s.actions = broker
}

func (s *State) Factory() *Factory { return s.factory }

func (s *State) SetFactory(factory *Factory) {
	s.factory = factory
}

func (s *State) GetPlayerDijkstraMap() map[geometry.Point]int {
	return s.playerDijkstraMap
}

// GetAllPOIs returns all points of interest on the current map.
// This is used to make sure that all the required locations on a map can be reached.
// This includes
// - stairs
// - vendors
// - enemies with win condition
// - quest items
func (s *State) GetAllPOIs() []geometry.Point {
	return s.GetMap().GetPositionsByFilter(func(mapCellRef *gridmap.MapCell[*Actor, *Item, Object]) bool {
		cell := *mapCellRef
		isStairs := cell.TileType.Special == gridmap.SpecialTileStairsUp || cell.TileType.Special == gridmap.SpecialTileStairsDown
		isVendor := cell.Actor != nil && (*cell.Actor).IsVendor()
		isWinCondition := cell.Actor != nil && (*cell.Actor).IsWinCondition()
		isQuestItem := cell.Item != nil && (*cell.Item).IsQuestItem()
		return isStairs || isVendor || isWinCondition || isQuestItem
	})
}
func getDropPos(gameMap *gridmap.GridMap[*Actor, *Item, Object], actorPos geometry.Point) geometry.Point {
	isFree := func(pos geometry.Point) bool {
		if gameMap.IsActorAt(pos) {
			actorAt := gameMap.ActorAt(pos)
			if actorAt.IsVendor() {
				return false
			}
		}
		if !gameMap.Contains(pos) || gameMap.GetCell(pos).TileType.Special != gridmap.SpecialTileNone {
			return false
		}
		return gameMap.IsWalkable(pos) && !gameMap.IsItemAt(pos) && !gameMap.IsObjectAt(pos)
	}
	if isFree(actorPos) {
		return actorPos
	}
	freeCells := gameMap.GetFreeCellsForDistribution(actorPos, 1, isFree)
	if len(freeCells) == 0 { // e.g. a container walled in by others: search the whole reachable area
		freeCells = freeCellsNear(gameMap, actorPos, gameMap.MapSize().X*gameMap.MapSize().Y, isFree)
	}
	if len(freeCells) == 0 {
		println("ERR: No free cells for item drop")
		return actorPos
	}
	dropPos := freeCells[0]
	return dropPos
}

func multiplyColors(drawColor color.RGBA, lightColor color.RGBA) color.RGBA {
	rVal, gVal, bVal, _ := drawColor.R, drawColor.G, drawColor.B, drawColor.A
	rValN := uint8(float64(rVal) * (float64(lightColor.R) / 255.0))
	gValN := uint8(float64(gVal) * (float64(lightColor.G) / 255.0))
	bValN := uint8(float64(bVal) * (float64(lightColor.B) / 255.0))
	//aValN := uint8(float64(aVal) * percentagOfValue)
	drawColor = color.RGBA{R: rValN, G: gValN, B: bValN, A: 255}
	return drawColor
}

func multiplyColor(drawColor color.RGBA, percentagOfValue float64) color.RGBA {
	rVal, gVal, bVal, _ := drawColor.R, drawColor.G, drawColor.B, drawColor.A
	rValN := uint8(float64(rVal) * percentagOfValue)
	gValN := uint8(float64(gVal) * percentagOfValue)
	bValN := uint8(float64(bVal) * percentagOfValue)
	//aValN := uint8(float64(aVal) * percentagOfValue)
	drawColor = color.RGBA{R: rValN, G: gValN, B: bValN, A: 255}
	return drawColor
}

type FlagName string

const (
	FlagHasTrueSight     FlagName = "has_true_sight"
	FlagStoryModeEnabled FlagName = "story_mode_enabled"
)

type QuestFlags struct {
	Giver        string
	Description  string
	GivenFlag    string
	GoalFlag     string
	RewardedFlag string
}

// d1Glow is a light source's reach on one cell: how much of it lands there (0..1) and whether it is the church's
// red light rather than ordinary torch light.
type d1Glow struct {
	v   float64
	red bool
}

// d1ChurchLightRadius is how far the red light inside the demon-possessed church reaches. Its source sits inside
// the nave, so the light bathes the church and spills back out of the doorway; the line-of-sight check below
// stops it at the church walls, lighting the windows from the inside.
const d1ChurchLightRadius = 19

// d1ChurchLightPos is where the red light burns: inside the cathedral, a few tiles behind its entrance at (41,6).
var d1ChurchLightPos = geometry.Point{X: 37, Y: 6}

// d1GlowAt is how much decorative light (wall torches, braziers, the church's red light) falls on a cell, 0..1:
// brightest at the light, fading to nothing at its radius, only where the light has a clear line. Built once per map.
func (s *State) d1GlowAt(p geometry.Point) d1Glow {
	m := s.GetMap()
	if s.glowCache == nil {
		s.glowCache = map[*gridmap.GridMap[*Actor, *Item, Object]]map[geometry.Point]d1Glow{}
	}
	glow, ok := s.glowCache[m]
	if !ok {
		glow = map[geometry.Point]d1Glow{}
		church := geometry.Point{X: -1, Y: -1}
		if defs.IsTownLevel(m.GetName()) {
			church = d1ChurchLightPos
		}
		for y := 0; y < m.MapSize().Y; y++ {
			for x := 0; x < m.MapSize().X; x++ {
				src := geometry.Point{X: x, Y: y}
				r, red := 0, false
				if icon, has := m.GetDecal(src); has && icon == defs.IconWallTorch {
					r = d1LightRadius(icon)
				}
				if m.IsObjectAt(src) { // ObjectAt dereferences the cell's object: only call it where there is one
					if wl, ok := m.ObjectAt(src).(*d1WallLight); ok {
						r = d1LightRadius(wl.icon)
					} else if o, ok := m.ObjectAt(src).(*D1Object); ok {
						r = d1ObjectLights[o.O.Kind]
					}
				}
				if src == church {
					r, red = d1ChurchLightRadius, true
				}
				for dy := -r; dy <= r && r > 0; dy++ {
					for dx := -r; dx <= r; dx++ {
						q := src.Add(geometry.Point{X: dx, Y: dy})
						d := geometry.Distance(src, q)
						if !m.Contains(q) || d >= float64(r) {
							continue
						}
						los := geometry.LineOfSight(src, q, func(c geometry.Point) bool { return c == src || m.IsTransparent(c) })
						if len(los) > 0 && los[len(los)-1] == q {
							if v := 1 - d/float64(r); v > glow[q].v {
								glow[q] = d1Glow{v: v, red: red}
							}
						}
					}
				}
			}
		}
		s.glowCache[m] = glow
	}
	return glow[p]
}
