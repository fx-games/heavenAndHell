package common

import (
	"fmt"
	"image/color"
	"log/slog"
	"math"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/hhm"
	"rpg/src/renderer"
	"rpg/src/rules"
	"rpg/src/stats"
	"rpg/src/util"
	"slices"
	"strconv"
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
	CloseDoorAt(mapPos geometry.Point) bool
}
type State struct {
	lighting       LightRules // the mod's lighting rules (mod.rec)
	currentMap     *gridmap.GridMap[*Actor, *Item, Object]
	maps           map[string]*gridmap.GridMap[*Actor, *Item, Object] // every level built this game, by id
	portal         *Portal
	player         *Actor
	camera         Camera
	msg            func(string)
	playerTookTurn func(int)

	onMapChanged func(oldName, newName string)

	visibility        *Visibility
	townMapSpawn      geometry.Point
	townVisits        int                   // how often the hero entered a town (Respawn: town_visits)
	lastSeen          map[string]LevelClock // when the hero last left each dungeon level (respawn.go)
	dungeonRules      DungeonRules          // the mod's Respawn and PersistLevels (mod.rec)
	deepestLevel      string                // the deepest non-town level visited (Visit)
	deepestTown       string                // the deepest town visited: where the town portal leads
	flags             *Flags
	playerStashInTown *Inventory
	playerDijkstraMap map[geometry.Point]int
	timekeeper        *TimeKeeper
	actions           *Broker
	animator          Animator
	dropsFrom         *Actor   // the monster dying right now: its loot shows when it falls
	dying             []*Actor // dead, but still drawn standing until the killing projectile reaches them

	config Configuration

	factory *Factory
	mapGen  *MapGenerator
	// maps
	drawnMaps *hhm.Project // every level the mod draws by hand, templates included

	SelectedClass defs.LoadoutDefinition
	worldTexture  renderer.TextureAtlas

	gossipSeeds map[VendorType]int64 // each townsperson's _tSeed: rolled on first talk, dropped when the hero reaches the dungeon

	glowCache map[*gridmap.GridMap[*Actor, *Item, Object]]map[geometry.Point]lightGlow // decorative light per map (glowAt)
}

func (s *State) GetWorldTexture() renderer.TextureAtlas {
	return s.worldTexture
}
func (s *State) GetPlayerStashInTown() *Inventory {
	return s.playerStashInTown
}
func NewState(maps *hhm.Project) State {

	return State{
		maps:              make(map[string]*gridmap.GridMap[*Actor, *Item, Object]),
		flags:             NewFlags(),
		playerStashInTown: NewInventory(32),
		drawnMaps:         maps,
		visibility:        NewVisibility(),
		gossipSeeds:       map[VendorType]int64{},
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

// Quests reports which of D1's quests this game's seed picked (InitQuests): QuestNotAvail for the ones it
// disabled, so callers can tell an absent quest boss from a missing one.
func (s *State) Quests() *rules.Quests {
	return s.mapGen.questTable()
}

func (s *State) GetVisibility() *Visibility {
	return s.visibility
}

// Dying are the dead actors to draw as still standing: their killing blow is still in the air.
func (s *State) Dying() []*Actor {
	s.dying = slices.DeleteFunc(s.dying, func(a *Actor) bool { return !a.VisualsPending() })
	return s.dying
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
func (s *State) SetTownSpawn(spawn geometry.Point) {
	s.townMapSpawn = spawn
}
func (s *State) GetTownSpawn() geometry.Point {
	return s.townMapSpawn

}
func (s *State) OnActorSpawned(actor *Actor) {
	s.timekeeper.AddTimed(actor)
}

func (s *State) GetFlags() *Flags {
	return s.flags
}

func (s *State) PlayerStandsInLitArea() bool {
	currentMap := s.GetMap()
	isPlayerLit := currentMap.IsTileLit(s.GetPlayer().MapPosition())
	return isPlayerLit || s.IsDaylight()
}

func (s *State) PlayerCanSeeNeighbors() bool {
	return s.PlayerLightRadius() > 0 || s.PlayerStandsInLitArea()
}

// LightRules is how the mod lights the hero's way, from its mod.rec:
//
//	Lighting          aura: Diablo's, a light around the hero always, radius 10 plus the gear's light affixes
//	                  (Sheet.LightRad); carried: Angband's, only the equipped light source while it burns, radius
//	                  what the gear adds to light_radius. Default aura
//	LightInTown       carried only: whether the light source glows in town too. Default true
//	LightBurnsInTown  carried only: whether it uses up charges there. Default false
//	AuraLightColor    "R G B" of the hero's own light under aura, of wall torches and braziers, and of a light
//	                  source item that names no light_color. Default 255 255 255
//	AuraLightFlicker  its flicker (fire, smooth, steady or numbers, as items.rec's); AuraLightFlickerDelay seconds
//	                  per frame. Default steady
type LightRules struct {
	Model       string
	InTown      bool
	BurnsInTown bool
	Aura        LightStats
}

// Lighting reads the mod's LightRules.
func Lighting() LightRules {
	r := LightRules{Model: "aura", InTown: true}
	if v, _ := ModSetting("Lighting"); v == "carried" {
		r.Model = "carried"
	}
	if v, ok := ModSetting("LightInTown"); ok {
		r.InTown = v == "true"
	}
	if v, _ := ModSetting("LightBurnsInTown"); v == "true" {
		r.BurnsInTown = true
	}
	r.Aura = LightStats{LightColor: color.RGBA{R: 255, G: 255, B: 255, A: 255}, FlickerPattern: []float64{1}, FlickerFrameDelay: 1}
	if v, ok := ModSetting("AuraLightColor"); ok {
		r.Aura.LightColor = util.GetColorFromRGBString(v)
	}
	if v, ok := ModSetting("AuraLightFlicker"); ok {
		r.Aura.FlickerPattern = defs.FlickerPattern(v)
	}
	if v, ok := ModSetting("AuraLightFlickerDelay"); ok {
		if d, err := strconv.ParseFloat(v, 64); err == nil && d > 0 {
			r.Aura.FlickerFrameDelay = d
		}
	}
	return r
}

// aura is the mod's aura light (LightRules.Aura), read from mod.rec when nothing set the rules yet.
func (s *State) aura() LightStats {
	if s.lighting.Aura.FlickerPattern == nil {
		s.lighting.Aura = Lighting().Aura
	}
	return s.lighting.Aura
}

// carriedLight is the hero's equipped light source while it shines under the mod's rules: it has charges left,
// and in town only if the mod lets lights glow there. Nil under Diablo's aura.
func (s *State) carriedLight() *Item {
	if s.lighting.Model != "carried" || (s.IsPlayerInTown() && !s.lighting.InTown) {
		return nil
	}
	light := s.GetPlayer().GetEquipment().GetLightSource()
	if light == nil || !light.IsLightSource() || !light.HasChargesLeft() {
		return nil
	}
	return light
}

// LightBurns reports whether the hero's light source uses up a charge this turn: in the dark of the dungeon, and
// in town only where the mod says lights burn there.
func (s *State) LightBurns() bool {
	if s.IsPlayerInTown() && s.lighting.Model == "carried" {
		return s.lighting.BurnsInTown && s.carriedLight() != nil
	}
	return s.GetPlayer().GetEquipment().HasLightSourceEquipped() && !s.PlayerStandsInLitArea()
}

// PlayerLightRadius is how far the hero's own light reaches in the dark, under the mod's Lighting.
func (s *State) PlayerLightRadius() int {
	actor := s.GetPlayer()
	if s.lighting.Model != "carried" {
		if actor.hero != nil {
			return actor.HeroSheet().LightRad
		}
		return 0
	}
	if s.carriedLight() == nil {
		return 0
	}
	radius := actor.GetStats().GetAttribute(stats.LightRadius)
	for _, it := range actor.GetEquipment().AllItems() {
		if it.roll != nil {
			radius += it.roll.PLLight
		}
	}
	return max(radius, 0)
}

// SetLighting picks the mod's lighting rules (Lighting) for this game.
func (s *State) SetLighting(rules LightRules) { s.lighting = rules }
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
		visionRange = s.PlayerLightRadius() // the mod's Lighting: Diablo's aura or the light source carried
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
	return s.applyPortalLight(mapPos, s.applyRedLight(mapPos, s.applyTownLight(mapPos, drawColor, s.applyLighting(mapPos, drawColor, worldTick), worldTick), worldTick), worldTick)
}

// applyTownLight lets a carried light glow in town, where the daylight (or the gloom before the first descent)
// otherwise decides alone: the tile's own color, tinted by the light and flickering, over what the town shows,
// fading out at the light's radius.
func (s *State) applyTownLight(mapPos geometry.Point, base, shaded color.RGBA, worldTick uint64) color.RGBA {
	if !s.IsDaylight() || !s.GetPlayer().CanSee(mapPos) {
		return shaded
	}
	light := s.carriedLight()
	if light == nil {
		return shaded
	}
	reach := float64(s.PlayerLightRadius() + 1)
	d := geometry.Distance(s.GetPlayer().MapPosition(), mapPos)
	if d >= reach {
		return shaded
	}
	stats := light.GetLightStats()
	flicker := stats.FlickerPattern[util.GetLoopingFrameFromTick(worldTick, stats.FlickerFrameDelay, len(stats.FlickerPattern))]
	falloff := 1 - d/reach
	return lerpRGBA(shaded, multiplyColors(base, multiplyColor(stats.LightColor, flicker)), falloff*falloff)
}

// applyPortalLight bathes the cells right around an open town portal in a cold, shimmering light blue, lifted out
// of the dark like the church's red light.
func (s *State) applyPortalLight(mapPos geometry.Point, drawColor color.RGBA, worldTick uint64) color.RGBA {
	glow := s.glowAt(mapPos)
	if glow.kind != glowPortal || glow.v <= 0 || !s.GetMap().IsExplored(mapPos) {
		return drawColor
	}
	pulse := 0.8 + 0.2*math.Sin(float64(worldTick)/12.0)
	lit := color.RGBA{
		R: uint8(min(float64(drawColor.R)*0.45+80*glow.v, 255)),
		G: uint8(min(float64(drawColor.G)*0.8+170*glow.v, 255)),
		B: uint8(min(float64(drawColor.B)+235*glow.v, 255)),
		A: drawColor.A,
	}
	return lerpRGBA(drawColor, lit, min(glow.v*1.8, 1.0)*pulse)
}

// applyRedLight tints a cell inside the demon-possessed church (and the ground the light falls on outside its
// door) red, pulsing slowly, and lifts it out of the dark so the glow reads as a light source and not as shadow.
func (s *State) applyRedLight(mapPos geometry.Point, drawColor color.RGBA, worldTick uint64) color.RGBA {
	glow := s.glowAt(mapPos)
	if glow.kind != glowRed || glow.v <= 0 || !s.GetMap().IsExplored(mapPos) {
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
		if glow := s.glowAt(mapPos); glow.v > 0 && glow.kind == glowTorch { // wall torches and braziers, each flickering out of step
			frame := util.GetLoopingFrameFromTick(worldTick+uint64(mapPos.X*7+mapPos.Y*13), s.aura().FlickerFrameDelay, len(s.aura().FlickerPattern))
			if g := glow.v * s.aura().FlickerPattern[frame]; g > percentageLightness {
				percentageLightness = g
				drawColor = multiplyColors(drawColor, s.aura().LightColor)
			}
		}
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
	if s.lighting.Model != "carried" && s.GetPlayer().hero != nil {
		flicker := s.aura().FlickerPattern[util.GetLoopingFrameFromTick(worldTick, s.aura().FlickerFrameDelay, len(s.aura().FlickerPattern))]
		return multiplyColors(drawColor, multiplyColor(s.aura().LightColor, flicker))
	}
	equipment := state.GetPlayer().GetEquipment()
	if equipment.HasLightSourceEquipped() {
		lightSource := equipment.GetLightSource()
		if lightSource.HasChargesLeft() && (s.lighting.Model != "carried" || s.carriedLight() != nil) {
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
	name, q := s.GetMap().GetName(), s.QuestActive(rules.QPWater)
	if (s.Level(name).Town || s.setQuest(name) == rules.QPWater) && q != rules.QuestNotAvail && q != rules.QuestDone {
		return color.RGBA{R: 199, G: 251, B: 161, A: 255}
	}
	return color.RGBA{R: 86, G: 112, B: 194, A: 255}
}

// WaterColorOf tints one water tile: GetWaterColor, lightened halfway to white for the shallows one wades through.
func (s *State) WaterColorOf(t gridmap.Tile) color.RGBA {
	c := s.GetWaterColor()
	if t.IsWalkable {
		c = color.RGBA{R: c.R/2 + 128, G: c.G/2 + 128, B: c.B/2 + 128, A: 255}
	}
	return c
}

func (s *State) IsDoorAt(pos geometry.Point) (*Door, bool) {
	currentMap := s.GetMap()
	objectAt, ok := currentMap.TryGetObjectAt(pos)
	if !ok {
		return nil, false
	}
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
		icon = bloodDecals[rng.Intn(len(bloodDecals))]
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
	// remove player from current map
	oldMapName := ""
	if s.GetMap() != nil && s.GetPlayer() != nil {
		s.GetMap().RemoveActor(s.GetPlayer())
		oldMapName = s.GetMap().GetName()
		s.leaveLevel(oldMapName)
	}

	player := s.GetPlayer()
	if player != nil {
		player.GetStatusEffects().SetFalse(string(StatusVision))
	}

	// change map
	targetMap := to.TargetMap
	if !s.HasMap(targetMap) {
		s.SetMap(targetMap, s.mapGen.GenerateMap(targetMap))
	}
	s.SetCurrentMap(s.Map(targetMap))
	s.enterLevel(s.GetMap())
	s.Visit(targetMap)
	s.wireExits(s.GetMap()) // an exit may have opened since (open_when: deepest, quests)
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

	freeTargetLocation := freeTargetLocations[rng.Intn(len(freeTargetLocations))]
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
		actor.ResetEnergy()
		s.timekeeper.AddTimed(actor)
	}

	if s.onMapChanged != nil {
		s.onMapChanged(oldMapName, to.TargetMap)
	}
}

func (s *State) AddItemToMapWithDistribution(item *Item, pos geometry.Point) {
	currentMap := s.GetMap()
	dropPos := getDropPos(currentMap, pos)
	currentMap.AddItem(item, dropPos)
	if s.dropsFrom.VisualsPending() {
		item.shownWith = s.dropsFrom
	}
	s.actions.Emit(ItemDropped{Item: item, At: dropPos, From: s.dropsFrom})
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
		if actorAt, ok := gameMap.TryGetActorAt(pos); ok {
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
		slog.Error("no free cells for item drop")
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
	drawColor = color.RGBA{R: rValN, G: gValN, B: bValN, A: 255}
	return drawColor
}

func multiplyColor(drawColor color.RGBA, percentagOfValue float64) color.RGBA {
	rVal, gVal, bVal, _ := drawColor.R, drawColor.G, drawColor.B, drawColor.A
	rValN := uint8(float64(rVal) * percentagOfValue)
	gValN := uint8(float64(gVal) * percentagOfValue)
	bValN := uint8(float64(bVal) * percentagOfValue)
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

// lightGlow is a light source's reach on one cell: how much of it lands there (0..1) and which kind of light it is.
type lightGlow struct {
	v    float64
	kind glowKind
}

type glowKind uint8

const (
	glowTorch  glowKind = iota // wall torches and braziers
	glowRed                    // the church's red light
	glowPortal                 // an open town portal
)

// portalLightRadius keeps the portal's light to the tiles right around it: full strength on the portal, falling
// off over the next two.
const portalLightRadius = 3

// churchLightRadius is how far the red light inside the demon-possessed church reaches. Its source sits inside
// the nave, so the light bathes the church and spills back out of the doorway; the line-of-sight check below
// stops it at the church walls, lighting the windows from the inside.
const churchLightRadius = 19

// churchLightPos is where the red light burns: inside the cathedral, a few tiles behind its entrance at (41,6).
var churchLightPos = geometry.Point{X: 37, Y: 6}

// Lit: some light source's glow reaches p (dLight below lightmax, the hero's own light aside).
func (s *State) Lit(p geometry.Point) bool { return s.glowAt(p).v > 0 }

// glowAt is how much decorative light (wall torches, braziers, the church's red light) falls on a cell, 0..1:
// brightest at the light, fading to nothing at its radius, only where the light has a clear line. Built once per map.
func (s *State) glowAt(p geometry.Point) lightGlow {
	m := s.GetMap()
	if s.glowCache == nil {
		s.glowCache = map[*gridmap.GridMap[*Actor, *Item, Object]]map[geometry.Point]lightGlow{}
	}
	glow, ok := s.glowCache[m]
	if !ok {
		glow = map[geometry.Point]lightGlow{}
		church := geometry.Point{X: -1, Y: -1}
		if s.Level(m.GetName()).Town {
			church = churchLightPos
		}
		for y := 0; y < m.MapSize().Y; y++ {
			for x := 0; x < m.MapSize().X; x++ {
				src := geometry.Point{X: x, Y: y}
				r, kind := 0, glowTorch
				if icon, has := m.GetDecal(src); has && icon == defs.IconWallTorch {
					r = lightRadius(icon)
				}
				switch o, _ := m.TryGetObjectAt(src); o := o.(type) {
				case *wallLight:
					r = lightRadius(o.icon)
				case *Prop:
					r = s.factory.objectDisplay(o.O.Kind).Light
				case *portalObject:
					r, kind = portalLightRadius, glowPortal
				}
				if src == church {
					r, kind = churchLightRadius, glowRed
				}
				halo(m, src, r, func(q geometry.Point, v float64) {
					if v > glow[q].v {
						glow[q] = lightGlow{v: v, kind: kind}
					}
				})
			}
		}
		s.glowCache[m] = glow
	}
	return glow[p]
}

// halo calls lit for each cell within radius r of a light at src that the light reaches past walls, with its
// brightness there: 1 at the light, fading to 0 at r.
func halo(m *gridmap.GridMap[*Actor, *Item, Object], src geometry.Point, r int, lit func(q geometry.Point, v float64)) {
	for dy := -r; dy <= r && r > 0; dy++ {
		for dx := -r; dx <= r; dx++ {
			q := src.Add(geometry.Point{X: dx, Y: dy})
			d := geometry.Distance(src, q)
			if !m.Contains(q) || d >= float64(r) {
				continue
			}
			los := geometry.LineOfSight(src, q, func(c geometry.Point) bool { return c == src || m.IsTransparent(c) })
			if len(los) > 0 && los[len(los)-1] == q {
				lit(q, 1-d/float64(r))
			}
		}
	}
}
