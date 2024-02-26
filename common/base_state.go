package common

import (
	"fmt"
	"image/color"
	"math/rand"
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
	RestUntilHealed()
	AutoAttack()
	ShowActorInfoAt(mapPos geometry.Point)
}
type State struct {
	currentMap     *gridmap.GridMap[*Actor, *Item, Object]
	townMaps       map[string]*gridmap.GridMap[*Actor, *Item, Object]
	dungeonMaps    map[string]*gridmap.GridMap[*Actor, *Item, Object]
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
	actions             ActionBroker
	animator            Animator

	config Configuration

	factory *Factory
	mapGen  *MapGenerator
	// maps
	ldtkProject *ldtk.Project

	SelectedClass defs.LoadoutDefinition
	worldTexture  renderer.TextureAtlas
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

func (s *State) isInHellTown() bool {
	return s.GetMap().GetName() == string(defs.LevelNameTownHell)
}

func (s *State) PlayerStandsInLitArea() bool {
	currentMap := s.GetMap()
	isPlayerLit := currentMap.IsTileLit(s.GetPlayer().MapPosition())
	return isPlayerLit || s.IsDaylight()
}

func (s *State) PlayerCanSeeNeighbors() bool {
	return s.GetPlayer().GetEquipment().HasLightSourceEquipped() || s.PlayerStandsInLitArea()
}
func (s *State) IsDaylight() bool {
	return s.IsPlayerInTown() && !s.GetFlags().IsSet("town_changed_to_night")
}
func (s *State) GetPlayerVisionRange() int {
	actor := s.GetPlayer()
	visionRange := actor.GetStats().GetAttribute(stats.VisionRange) // used in town (=day light)

	playerInDungeon := !s.IsPlayerInTown()
	if playerInDungeon {
		if s.GetMap().IsTileLit(actor.MapPosition()) {
			return visionRange
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

func (s *State) ApplyLighting(mapPos geometry.Point, drawColor color.RGBA, worldTick uint64) color.RGBA {
	state := s
	distanceToPlayer := geometry.Distance(state.GetPlayer().MapPosition(), mapPos)
	maxDistance := float64(state.GetPlayerVisionRange() + 1)
	darkPercentage := 0.16
	lightPercentage := 1.0
	if state.IsDaylight() {
		darkPercentage = 0.75
		lightPercentage = 0.95
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

func (s *State) mixLightSourceColor(drawColor, noLightSource color.RGBA, worldTick uint64) color.RGBA {
	state := s
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
func (s *State) GetWaterColor() color.RGBA {
	//all_clear:SpecialLevelPoisonedWaterSupply
	//
	poisonColor := color.RGBA{R: 199, G: 251, B: 161, A: 255}
	cleanColor := color.RGBA{R: 86, G: 112, B: 194, A: 255}

	if s.GetFlags().IsSet(fmt.Sprintf("all_clear:%s", defs.SpecialLevelPoisonedWaterSupply)) {
		return cleanColor
	} else {
		return poisonColor
	}
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
	// TODO: add these back somewhere?
	//s.CancelAutoMove()
	//g.actionBroker.CancelAllAnimations()

	//oldPosOnOldMap := g.state.GetPlayer().MapPosition()
	// remove player from current map
	oldMapName := ""
	if s.GetMap() != nil && s.GetPlayer() != nil {
		s.GetMap().RemoveActor(s.GetPlayer())
		oldMapName = s.GetMap().GetName()
	}

	player := s.GetPlayer()

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
		generatedMap := s.mapGen.GenerateMap(targetMap, s.ldtkProject, LevelFromName(targetMap))
		s.SetDungeonMap(targetMap, generatedMap)
		s.SetCurrentMap(generatedMap)
	}
	newlyLoadedMap := s.GetMap()
	newlyLoadedMap.SetCardinalMovementOnly(s.config.CardinalMovementOnly)

	targetLocation := newlyLoadedMap.GetNamedLocation(to.TargetLocation)
	freeTargetLocations := newlyLoadedMap.GetFreeCardinalNeighbors(targetLocation)

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

func (s *State) IsStoryModeEnabled() bool {
	return s.GetFlags().IsSet(string(FlagStoryModeEnabled))
}

func (s *State) GetQuestLog(textParser func(string) []util.ColoredTextPart) []util.LabelText {
	var entries []util.LabelText

	flags := s.GetFlags()

	flagPairs := []QuestFlags{
		{"Kirioth (Town)", "Kill Meat Jack (Level 2 - Cathedral)", "quest_received:meat_jack", "killed:meat_jack", "quest_rewarded:meat_jack"},
		{"Abel (Town)", "Clear the poisoned water (Catacombs)", "quest_received:poisoned_water", string("all_clear:" + defs.SpecialLevelPoisonedWaterSupply), "quest_rewarded:poisoned_water"},
		{"Shekel (Town)", "Kill King Zedek (Level 4 - Catacombs)", "quest_received:skeleton_king", "killed:skeleton_king", "quest_rewarded:skeleton_king"},
		{"Kirioth (Town)", "Kill Lucifer (Level 8 - Hell)", "quest_received:lucifer", "killed:lucifer", "quest_rewarded:lucifer"},

		{"Lucifer (Hell)", "Find evidence (Town)", "quest_received:evidence_one", "found:evidence_one", "quest_rewarded:evidence_one"},
		{"Lucifer (Hell)", "Find evidence (Catacombs)", "quest_received:evidence_two", "found:evidence_two", "quest_rewarded:evidence_two"},
		{"Lucifer (Hell)", "Find evidence (Water Supply)", "quest_received:evidence_three", "found:evidence_three", "quest_rewarded:evidence_three"},
		{"Lucifer (Hell)", "Find evidence (Caves)", "quest_received:evidence_four", "found:evidence_four", "quest_rewarded:evidence_four"},
	}
	//textParser := g.tileRenderer.ParseColorCodedText
	// meat jack flags
	for _, flagPair := range flagPairs {
		questName := flagPair.Description
		returnMessage := fmt.Sprintf("Return to %s", flagPair.Giver)
		questGivenFlag := flagPair.GivenFlag
		questDoneFlag := flagPair.RewardedFlag
		if flags.IsSet(questGivenFlag) {
			if flags.IsSet(questDoneFlag) {
				entries = append(entries, util.LabelText{
					UseColorCodes:      true,
					TextWithColorCodes: textParser(fmt.Sprintf("%s - [:green]DONE", questName)),
					Text:               fmt.Sprintf("%s - DONE", questName),
					TextColor:          defs.DarkTextColor,
				})
			} else if flags.IsSet(flagPair.GoalFlag) {
				entries = append(entries, util.LabelText{
					UseColorCodes:      true,
					TextWithColorCodes: textParser(fmt.Sprintf("%s - [:yellow]IN PROGRESS", returnMessage)),
					Text:               fmt.Sprintf("%s - IN PROGRESS", returnMessage),
					TextColor:          defs.DarkTextColor,
				})
			} else {
				entries = append(entries, util.LabelText{
					UseColorCodes:      true,
					TextWithColorCodes: textParser(fmt.Sprintf("%s - [:yellow]IN PROGRESS", questName)),
					Text:               fmt.Sprintf("%s - IN PROGRESS", questName),
					TextColor:          defs.DarkTextColor,
				})
			}
		}
	}
	return entries
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
		cell := gameMap.GetCell(pos)
		if cell.TileType.Special != gridmap.SpecialTileNone {
			return false
		}
		return gameMap.Contains(pos) && gameMap.IsWalkable(pos) && !gameMap.IsItemAt(pos) && !gameMap.IsObjectAt(pos)
	}
	if isFree(actorPos) {
		return actorPos
	}
	freeCells := gameMap.GetFreeCellsForDistribution(actorPos, 1, isFree)
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
