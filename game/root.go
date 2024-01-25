package game

import (
	"cmp"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"image/color"
	"io"
	"io/fs"
	"math"
	"math/rand"
	"os"
	"path"
	"rpg/actions"
	"rpg/audio"
	"rpg/common"
	"rpg/dungen"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/ldtk_go"
	"rpg/renderer"
	"rpg/stats"
	"rpg/ui"
	"rpg/util"
	"slices"
	"strings"
	"time"
)

type GameInstance struct {
	deviceDPIScale              float64
	tileScale                   float64
	title                       string
	deviceIndependentScreenSize geometry.Point

	tileRenderer *renderer.TileRenderer

	mapRenderer  *renderer.MapRenderer
	worldTexture renderer.TextureAtlas

	mousePosInPixels geometry.Point
	mousePosOnMap    geometry.Point

	message           PrintMessage
	tilemapAnimations map[int32]TilemapAnimInfo

	worldTicks uint64

	actionBroker *actions.Broker
	uiController *ui.UIController

	state             common.State
	hybridRenderer    *HybridRenderer
	targeter          *Targeter
	logBuffer         []string
	timekeeper        *TimeKeeper
	audioPlayer       *audio.Player
	playerDijkstraMap map[geometry.Point]int
	inventory         *ui.IconOverlay
	skills            *ui.IconOverlay
	timer             *ui.IconOverlay
	playerPath        []geometry.Point
	autoExplore       bool
	configuration     Configuration
	isFullScreen      bool
	statsLabel        *ui.BasicLabel
	levelUpButton     *ui.ButtonOverlay
	randomSource      *rand.Rand

	globalFlashColorTicksLeft  int
	globalFlashColorTicksStart int
	globalFlashColor           color.Color
	ldtkProject                *ldtk_go.Project
	miniMap                    *ui.MinimapOverlay
	disableFogOfWar            bool
	debugLines                 []util.Tuple[geometry.Point, geometry.Point]        // TODO: Move this into the renderer
	debugCells                  []geometry.Point
	shiftIsBeingPressedThisTick bool
	uiBarWidth                  int
	setToExactFit              bool
}

func (g *GameInstance) AddItemToMapWithDistribution(item common.Item, pos geometry.Point) {
	currentMap := g.state.GetMap()
	dropPos := getDropPos(currentMap, pos)
	currentMap.AddItem(item, dropPos)
}
func getDropPos(gameMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], actorPos geometry.Point) geometry.Point {
	isFree := func(pos geometry.Point) bool {
		if gameMap.IsActorAt(pos) {
			actorAt := gameMap.ActorAt(pos)
			if actorAt.IsVendor() {
				return false
			}
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

func (g *GameInstance) GetPlayerVisionRange() int {
	actor := g.state.GetPlayer()
	visionRange := actor.GetStats().GetAttribute(stats.VisionRange) // used in town (=day light)

	playerInDungeon := !g.IsPlayerInTown()
	if playerInDungeon {
		equipment := actor.GetEquipment()
		if equipment.HasLightSourceEquipped() {
			lightRadius := actor.GetStats().GetAttribute(stats.LightRadius) // used in dungeons (=darkness)
			visionRange = lightRadius
		} else {
			visionRange = 0
		}
	} else if g.state.GetFlags().IsSet("town_changed_to_night") {
		visionRange = visionRange / 2
	}
	return visionRange
}

func (g *GameInstance) IsPlayerInTown() bool {
	mapName := g.state.GetMap().GetName()
	return isTownLevel(mapName)
}

func (g *GameInstance) TriggerCellEffectsDuringMovementForActor(sourceAction actions.Action, actor *common.Actor, mapPos geometry.Point, startAnimating func() bool) {
	currentMap := g.state.GetMap()
	newPos := mapPos

	if currentMap.IsObjectAt(newPos) {
		objectAtLocation := currentMap.ObjectAt(newPos)
		objectAtLocation.ActorWalkedOn(actor, startAnimating)
	}

	if currentMap.GetCell(newPos).TileType.Special == gridmap.SpecialTileDamaging {
		action := actions.NewChangeRessourceAction(sourceAction.Cause(), actor, stats.Health, -3)
		action.SetAnimationStartPredicate(startAnimating)
		g.actionBroker.Execute(action)
	}
}

func (g *GameInstance) Say(actor *common.Actor, trigger common.ActorSpeechTrigger) {
	textLines := actor.GetSpeech(g.randomSource, trigger)
	if len(textLines) == 0 {
		return
	}

	if oldAnim, exists := g.actionBroker.GetAnimation(func (anim common.Animation) bool {
		if speechAnim, isSpeechAnim := anim.(*actions.SpeechAnimation); isSpeechAnim {
			return speechAnim.GetActor() == actor
		}
		return false
	}); exists {
		oldAnim.Cancel()
	}

	speechAnim := actions.NewSpeechAnimation(actor, textLines)
	g.actionBroker.AddAnimation(speechAnim)

	g.Log(fmt.Sprintf("%s says: %s", actor.Name(), strings.Join(textLines, "\n")))
}

func (g *GameInstance) UpdateMiniMap() {
	g.mapRenderer.UpdateMiniMap()
}

func (g *GameInstance) FlashScreen(color color.Color, seconds float64) {
	g.globalFlashColor = color
	g.globalFlashColorTicksLeft = int(util.SecondsToTicks(seconds))
	g.globalFlashColorTicksStart = int(util.SecondsToTicks(seconds))
}
func (g *GameInstance) MapTransition(to gridmap.Transition) {
	if to.IsEmpty() {
		return
	}
	g.timekeeper.ClearTrackedBeings(func(being TimedBeing) bool {
		switch being.(type) {
		case *common.Actor:
			return true
		default:
			return false
		}
	})
	g.cancelAutoMove()
	g.actionBroker.CancelAllAnimations()

	oldPosOnOldMap := g.state.GetPlayer().MapPosition()
	// remove player from current map
	player := g.state.GetPlayer()
	g.state.GetMap().RemoveActor(player)

	// change map
	targetMap := to.TargetMap
	if isTownLevel(targetMap) {
		if g.state.HasTownMap(targetMap) {
			g.SetMap(g.state.GetTownMap(targetMap))
		} else {
			// load map
			townMap, _ := g.NewGridMapFromLDTK(targetMap)
			g.SetMap(townMap)
		}
	} else if g.state.HasDungeonMap(targetMap) {
		g.SetMap(g.state.GetDungeonMap(targetMap))
	} else {
		// generate the map and set it
		generatedMap := g.generateMap(targetMap, g.ldtkProject, levelFromName(targetMap))
		g.state.SetDungeonMap(targetMap, generatedMap)
		g.SetMap(generatedMap)
	}
	newlyLoadedMap := g.state.GetMap()
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

	for _, actor := range newlyLoadedMap.Actors() {
		if actor == player {
			continue
		}
		actor.RemoveEnergy(actor.GetEnergy())
		g.timekeeper.AddTimed(actor)
	}

	g.Log(fmt.Sprintf("Travel from (%d, %d) to (%d, %d)", oldPosOnOldMap.X, oldPosOnOldMap.Y, freeTargetLocation.X, freeTargetLocation.Y))
}

func (g *GameInstance) OpenNPCMenu(npc *common.Actor) {
	vendorType := npc.GetVendorType()

	if vendorType == common.VendorTypeWands {
		g.state.GetPlayer().HealCompletely()
		g.Log(fmt.Sprintf("You feel refreshed"))
		g.updateStatsLabel()
	}

	options := g.getVendorOptions(npc)
	if len(options) == 0 {
		return
	}

	menu := g.uiController.OpenMenu(options)
	menu.OnMouseMoved(g.mousePosInPixels.X, g.mousePosInPixels.Y)
}
func (g *GameInstance) getVendorOptions(vendor *common.Actor) []util.MenuItem {
	vendorType := vendor.GetVendorType()
	var menuItems []util.MenuItem

	switch vendorType {
	case common.VendorTypeWands:
		menuItems = g.getWandVendorOptions(vendor)
	case common.VendorTypeWeapons:
		menuItems = g.getWeaponVendorOptions(vendor)
	case common.VendorTypeArmor:
		menuItems = g.getArmorVendorOptions(vendor)
	case common.VendorTypeGeneral:
		menuItems = g.getGeneralVendorOptions(vendor)
	}
	return menuItems
}
func (g *GameInstance) getWandVendorOptions(vendor *common.Actor) []util.MenuItem {
	return []util.MenuItem{
		{
			Text: "TALK",
			Action: func() {
				textForNPC, onShowFunc := g.getTextForNamedNPC(vendor)
				g.uiController.OpenTextWindow(textForNPC)
				if onShowFunc != nil {
					onShowFunc()
				}
			},
		},
		{
			Text: "BUY",
			Action: func() {
				g.openShopFor(vendor)
			},
		},
		{
			Text:   "TRANSFER",
			Action: g.openTransferEnchantmentMenu,
		},
	}
}

func (g *GameInstance) getGeneralVendorOptions(vendor *common.Actor) []util.MenuItem {
	return []util.MenuItem{
		{
			Text: "TALK",
			Action: func() {
				textForNPC, onShowFunc := g.getTextForNamedNPC(vendor)
				g.uiController.OpenTextWindow(textForNPC)
				if onShowFunc != nil {
					onShowFunc()
				}
			},
		},
		{
			Text: "BUY",
			Action: func() {
				g.openShopFor(vendor)
			},
		},
	}
}

func (g *GameInstance) isInHellTown() bool {
	return g.state.GetMap().GetName() == string(LevelNameTownHell)
}

func (g *GameInstance) getWeaponVendorOptions(vendor *common.Actor) []util.MenuItem {
	return []util.MenuItem{
		{
			Text: "TALK",
			Action: func() {
				textForNPC, onShowFunc := g.getTextForNamedNPC(vendor)
				g.uiController.OpenTextWindow(textForNPC)
				if onShowFunc != nil {
					onShowFunc()
				}
			},
		},
		{
			Text: "BUY",
			Action: func() {
				g.openShopFor(vendor)
			},
		},
	}
}

func (g *GameInstance) getArmorVendorOptions(vendor *common.Actor) []util.MenuItem {
	return []util.MenuItem{
		{
			Text: "TALK",
			Action: func() {
				textForNPC, onShowFunc := g.getTextForNamedNPC(vendor)
				g.uiController.OpenTextWindow(textForNPC)
				if onShowFunc != nil {
					onShowFunc()
				}
			},
		},
		{
			Text: "BUY",
			Action: func() {
				g.openShopFor(vendor)
			},
		},
		{
			Text:   "STASH",
			Action: g.openPlayerStashInTown,
		},
	}
}
func (g *GameInstance) openShopFor(npc *common.Actor) {
	g.uiController.PopAll()
	npcInventory := npc.GetInventory()
	if npcInventory.IsEmpty() {
		return
	}
	player := g.state.GetPlayer()
	goldAmount := player.GetStats().GetAttribute(stats.Gold)
	var menuItems []util.MenuItem
	for _, i := range npcInventory.Items() {
		item := i.(*GameItem)
		cost := item.BuyPrice()
		cannotPayForItem := cost > goldAmount
		colorCodeForPrice := "yellow"
		if cannotPayForItem {
			colorCodeForPrice = "red"
		}
		menuItems = append(menuItems, util.MenuItem{
			UseColorCodes:      true,
			TextWithColorCodes: fmt.Sprintf("%s - [:%s]%d", item.Name(), colorCodeForPrice, cost),
			Text:               fmt.Sprintf("%s - %d", item.Name(), cost),
			Action: func() {

				if cannotPayForItem {
					g.Log("You don't have enough gold")
					g.openShopFor(npc)
					return
				}
				player.GetStats().DecrementBy(stats.Gold, cost)
				npc.GetStats().IncrementBy(stats.Gold, cost)

				npc.GetInventory().Remove(item)
				player.GetInventory().Add(item)
				g.Log(fmt.Sprintf("You bought %s for %d gold", item.Name(), cost))

				g.openShopFor(npc)
			},
		})
	}
	shopMenu := g.uiController.OpenMenu(menuItems)
	shopMenu.OnMouseMoved(g.mousePosInPixels.X, g.mousePosInPixels.Y)
}

func (g *GameInstance) GetActionBroker() *actions.Broker {
	return g.actionBroker
}
func (g *GameInstance) GetMoveTowardsPlayer(from geometry.Point) geometry.Point {
	return g.getMoveOnPlayerDijkstraMap(from, true)
}

func (g *GameInstance) GetMoveAwayFromPlayer(from geometry.Point) geometry.Point {
	return g.getMoveOnPlayerDijkstraMap(from, false)
}
func (g *GameInstance) getMoveOnPlayerDijkstraMap(from geometry.Point, towardsPlayer bool) geometry.Point {
	if g.playerDijkstraMap == nil {
		return from
	}
	currentDistanceToPlayer, exist := g.playerDijkstraMap[from]
	if !exist {
		return from
	}
	var compareFunc func(a, b int) bool
	if towardsPlayer {
		compareFunc = func(a, b int) bool { return a < b }
	} else {
		compareFunc = func(a, b int) bool { return a > b }
	}
	currentMap := g.state.GetMap()
	neighbors := currentMap.GetFilteredNeighborsForMovement(from, func(pos geometry.Point) bool { // choose a possible next step
		if !currentMap.IsCurrentlyPassable(pos) { // only walk on passable tiles
			return false
		}
		if _, existsTransition := currentMap.GetTransitionAt(pos); existsTransition { // don't walk on transitions
			return false
		}
		neighborDist, neighborExists := g.playerDijkstraMap[pos]
		if !neighborExists {
			return false
		}
		neighborsWithTransition := currentMap.GetFilteredNeighbors(pos, func(pos geometry.Point) bool { // in fact, don't walk on tiles next to transitions either
			if !currentMap.IsCurrentlyPassable(pos) {
				return false
			}
			if _, existsTransition := currentMap.GetTransitionAt(pos); existsTransition {
				return true
			}
			return false
		})
		if len(neighborsWithTransition) > 0 {
			return false
		}
		return compareFunc(neighborDist, currentDistanceToPlayer) // depends on rolling direction on our dijkstra map
		//return neighborDist < currentDistanceToPlayer // depends on rolling direction on our dijkstra map
	})

	if len(neighbors) == 0 {
		return from
	}
	// we found some locations we can move to, that also bring us closer to the player
	// which of these is the closest to the player?

	nearestDist := math.MaxInt
	nearestPos := from

	for _, neighbor := range neighbors {
		neighborDist, _ := g.playerDijkstraMap[neighbor]
		//if neighborDist < nearestDist {  // depends on rolling direction on our dijkstra map
		if compareFunc(neighborDist, nearestDist) {
			nearestDist = neighborDist
			nearestPos = neighbor
		}
	}

	return nearestPos
}
func (g *GameInstance) SetPlayerDijkstraMap(dijkstraMap map[geometry.Point]int) {
	g.playerDijkstraMap = dijkstraMap
}

func (g *GameInstance) GetAudio() *audio.Player {
	return g.audioPlayer
}

func (g *GameInstance) GetCamera() common.Camera {
	return g.mapRenderer
}

func NewGameInstance(title string, deviceIndependentScreenSize geometry.Point, tileScale float64) *GameInstance {

	deviceScaleFactor := ebiten.DeviceScaleFactor()
	globalRandomSource := rand.NewSource(time.Now().UnixNano())
	g := &GameInstance{
		title:                       title,
		deviceIndependentScreenSize: deviceIndependentScreenSize,
		tileScale:                   tileScale,
		deviceDPIScale:              deviceScaleFactor,
		state:                       common.NewState(),
		randomSource:                rand.New(globalRandomSource),
		audioPlayer:                 audio.NewAudioPlayer(os.DirFS("assets").(fs.ReadDirFS), audio.SampleRate44100),
	}
	g.actionBroker = actions.NewBroker(g)

	tileRenderer := renderer.NewTileRenderer(g.GetDeviceScaleFactor, g.GetTileScaleFactor)
	tileRenderer.SetColorFromName(common.ColorFromName)
	tileRenderer.SetWhiteTile(common.IconWhite)
	tileRenderer.SetDefaultColors(common.DarkYellow, color.Black)
	g.SetTileRenderer(tileRenderer)

	timeKeeper := NewTimeKeeper(func(being TimedBeing) (int, func()) {
		switch typedBeing := being.(type) {
		case *common.Actor:
			return g.getAIAction(typedBeing)
		case Turner:
			return g.getTurnerAction(being.GetBaseSpeed(), being.GetEnergy(), typedBeing)
		}
		return 0, nil
	})
	g.SetTimeKeeper(timeKeeper)

	return g
}

func (g *GameInstance) InitGame(config Configuration) {
	g.configuration = config
	g.state.SetLogger(g.Log)
	g.state.SetPlayerTurnHandler(g.onPlayerTurnTaken)
	g.state.SetActorSpawnHandler(func(actor *common.Actor) { g.timekeeper.AddTimed(actor) })
	g.state.InitVisibility()
	g.timekeeper.SetOnRemoveTimed(g.afterTimedRemoved)


	g.actionBroker.RegisterHook(actions.NewOnHealthChangedHook())

	g.actionBroker.RegisterHook(actions.NewMainOnMoveHook(g.configuration.AutoPickup)) // update fov, etc.
	g.actionBroker.RegisterHook(actions.NewMainOnMeleeHook())
	g.actionBroker.RegisterHook(actions.NewMainOnRangedHook())
	g.actionBroker.RegisterHook(actions.NewMainOnKillHook())
	g.actionBroker.RegisterHook(actions.NewOnDieHook("lucifer talking", func(itemContext *actions.ItemContext, action actions.Action, engine actions.Engine) []actions.Action {
			dieAction := action.(*actions.DieAction)
			dyingActor := dieAction.GetActor()
			if dyingActor.GetInternalName() == "lucifer" {
				g.onLuciferDeath()
			}
			return nil
	}))

	g.actionBroker.RegisterHook(actions.NewMainOnDropHook())
	g.actionBroker.RegisterHook(actions.NewMainOnEquipHook())

	masterSword := NewMasterSword(g.GetWorldState())
	sword := NewSword(g.GetWorldState(), g.randomSource, 1)
	//g.TransferRandomEnchantment(masterSword, sword)
	g.spawnItem(geometry.Point{X: 15, Y: 10}, masterSword)
	g.spawnItem(geometry.Point{X: 17, Y: 10}, sword)

	g.spawnItem(geometry.Point{X: 15, Y: 14}, NewArmor(g.GetWorldState(), g.randomSource, 1))

	g.spawnItem(geometry.Point{X: 15, Y: 16}, NewBow(g.GetWorldState(), g.randomSource, 1))

	g.spawnItem(geometry.Point{X: 15, Y: 20}, g.NewEtherealTargetedWand(g.getFireballAction(ActionNameFireball)))

	g.spawnItem(geometry.Point{X: 15, Y: 21}, g.NewInfiniteUntargetedWand(g.getMultiShot(5)))

	g.spawnItem(geometry.Point{X: 16, Y: 20}, g.NewFiniteUntargetedWand(g.getHealHealthAndMana()))

	g.spawnItem(geometry.Point{X: 14, Y: 18}, NewHelmet(g.GetWorldState(), g.randomSource, 1))

	g.spawnItem(geometry.Point{X: 14, Y: 17}, g.NewHiddenNote("a letter from hell", NewDialogueTextFromFile("note_lucifer.txt"), nil))
	g.spawnItem(geometry.Point{X: 14, Y: 16}, NewAmulet(g.GetWorldState(), g.randomSource, 1))

	//g.spawnFireAt(geometry.Point{X: 15, Y: 22})

	townSpawn := g.state.GetMap().GetNamedLocation("spawn")
	g.state.SetTownSpawn(townSpawn)
	g.spawnPlayer(townSpawn)

	// add actors to timekeeper turn system
	for _, actor := range g.state.GetMap().Actors() {
		if actor == g.state.GetPlayer() {
			continue
		}
		g.timekeeper.AddTimed(actor)
	}

	// load audio
	if config.AudioEnabled {
		sfxPath := path.Join("sfx")
		cues := []string{
			"heavy_door",
			"missile_hit_human",
			"heavy_hit_human",
			"levelup",
			"bat_attack",
			"skeleton_hit",
			"skeleton_death",
		}
		sfxFiles := make([]string, len(cues))
		for i, sfxFile := range cues {
			sfxFiles[i] = path.Join(sfxPath, sfxFile+".ogg")
		}
		g.audioPlayer.RegisterSoundCues(sfxFiles)
		g.audioPlayer.RegisterRandomizedSoundCues([]string{
			path.Join(sfxPath, "bow_fire"),
			//path.Join(sfxPath, "close_door"),
			path.Join(sfxPath, "open_door"),
			path.Join(sfxPath, "melee_attack"),
			path.Join(sfxPath, "human_hit"),
		})

		g.audioPlayer.PreLoadCuesIntoMemory(cues)
	}


	g.state.GetFlags().SetTrue("game_started")
	g.updateStatsLabel()
	g.UpdateMiniMap()
}

func (g *GameInstance) setupUI() {
	g.uiController = ui.NewUIController(g.hybridRenderer, g.GetDeviceIndependentScreenSize())

	paddingLeftAndRight := 6
	g.inventory = g.addInventoryOverlay(geometry.Point{X: paddingLeftAndRight, Y: 10})
	inventoryBoxSize := g.inventory.GetBounds().Size()

	g.uiBarWidth = inventoryBoxSize.X + (2*paddingLeftAndRight)
	g.uiController.AddBlackBackgroundBorder(g.uiBarWidth)
	xOffset := g.uiBarWidth-paddingLeftAndRight

	g.levelUpButton = g.addLevelUpButton(geometry.Point{X: xOffset, Y: 100})
	buttonBounds := g.levelUpButton.GetBounds()

	g.skills = g.addSkillsOverlay(geometry.Point{X: paddingLeftAndRight, Y: 180})
	skillBounds := g.skills.GetBounds()

	topYForMap := buttonBounds.Max.Y + paddingLeftAndRight
	bottomYForMap := skillBounds.Min.Y - paddingLeftAndRight
	maxHeightForMap := bottomYForMap - topYForMap

	g.statsLabel = g.addStatsLabel(geometry.Point{X: xOffset, Y: paddingLeftAndRight})

	g.miniMap = g.addMiniMap(geometry.Point{X: g.uiBarWidth, Y: topYForMap}, maxHeightForMap, paddingLeftAndRight)

	g.timer = g.addTimerOverlay(geometry.Point{X: paddingLeftAndRight, Y: 360})

	g.OnScreenSizeChanged()
}
func (g *GameInstance) handleLightSource() {
	player := g.state.GetPlayer()
	if player.GetEquipment().HasLightSourceEquipped() && !g.isDaylight() {
		lightSource := player.GetEquipment().GetLightSource()
		asGameItem := lightSource.(*GameItem)
		asGameItem.UseCharge()
	}
}
func (g *GameInstance) onPlayerTurnTaken(autUsed int) {
	player := g.state.GetPlayer()

	g.timekeeper.PlayerTakesTurn(autUsed)

	g.updateStatsLabel()
	g.checkForLevelUp()

	player.BeforeTurn()
	g.handleLightSource()

	g.UpdateMiniMap()

	if !g.setToExactFit && !g.configuration.KeepPlayerCentered{
		distFromBorder := 4
		playerPos := player.MapPosition()
		visibleMap := g.mapRenderer.GetVisibleMap()
		visibleMap = visibleMap.Shift(distFromBorder, distFromBorder, -distFromBorder, -distFromBorder)
		if !visibleMap.Contains(playerPos) {
			g.mapRenderer.CenterOnFloat(player.DrawPosition())
		}
	}


	// automatic player movement..
	if g.configuration.AutoPickup && g.state.GetMap().IsItemAt(player.MapPosition()) {
		itemAtLocation := g.state.GetMap().ItemAt(player.MapPosition())
		if itemAtLocation != nil && player.CanPickup() && !itemAtLocation.IsDroppedByPlayer() {
			pickUpAction := actions.NewPickUpAction(actions.AbstractCause("AutoPickup"), player)
			endTurnAction := actions.NewEndPlayerTurnAction(pickUpAction.Cause(), pickUpAction.GetDelayInAut())
			g.actionBroker.Execute(pickUpAction)
			g.actionBroker.Execute(endTurnAction)
		}
	} else {
		g.nextStepOnPlayerPath()
	}
}

func (g *GameInstance) spawnFireAt(location geometry.Point) {
	fire := common.NewFire(3)
	onTurn := func() {
		if !fire.IsAlive() {
			g.removeObject(fire)
			println("fire died")
			return
		}
		fire.DealDamage(nil)
	}
	fire.SetOnTurn(onTurn)
	fire.SetDealDamage(g.procFireAtLocation)
	g.timekeeper.AddTimed(fire)

	g.spawnObject(location, fire)
	g.procFireAtLocation(location, actions.AnimateNow())
}

func (g *GameInstance) procFireAtLocation(location geometry.Point, startAnimating func() bool) {
	if g.state.GetMap().IsActorAt(location) {
		actor := g.state.GetMap().ActorAt(location)
		action := actions.NewChangeRessourceAction(actions.AbstractCause("Fire"), actor, stats.Health, -3)
		action.SetAnimationStartPredicate(startAnimating)
		g.actionBroker.Execute(action)
	}
}
func (g *GameInstance) PickUpItem() {
	player := g.state.GetPlayer()
	if !player.IsAlive() || !player.CanPickup() {
		return
	}
	if !g.state.GetMap().IsItemAt(player.MapPosition()) {
		return
	}
	pickUpAction := actions.NewPickUpAction(actions.AbstractCause("PickUpAction"), player)
	g.actionBroker.Execute(pickUpAction)
	g.actionBroker.Execute(actions.NewEndPlayerTurnAction(pickUpAction.Cause(), pickUpAction.GetDelayInAut()))
}
func (g *GameInstance) BeginRangedTargeting() {
	if !g.state.GetPlayer().IsAlive() {
		return
	}
	rangedAttackRules := NewRangedAttackTargetingRules(g.state.GetPlayer(), OneTargetTile, g.state.GetMap())
	g.targeter = NewTargeter(g.hybridRenderer, rangedAttackRules, g.playerAimRangedAttack)
}

func (g *GameInstance) CancelTargeting() {
	g.targeter = nil
}

func (g *GameInstance) spawnPlayer(mapPos geometry.Point) {
	player := common.NewActor(common.IconCharacterWarriorMale)
	player.SetName("Player")
	player.SetDrawPosition(mapPos.ToPointF())
	g.state.SetPlayer(player)
	g.state.GetMap().AddActor(player, mapPos)
	g.initPlayerSkillsForArcher(player)

	torch := NewTorch(g.GetWorldState(), g.randomSource, 1)
	player.GetInventory().Add(torch)
	g.actionBroker.Execute(actions.NewEquipItemAction(actions.AbstractCause("SpawnPlayer"), player, torch))
}

func (g *GameInstance) spawnItem(mapPos geometry.Point, item common.Item) {
	g.state.GetMap().AddItem(item, mapPos)
}

func (g *GameInstance) spawnItemInTown(namedLocation string, item common.Item) {
	townMap := g.state.GetTownMap(string(LevelNameTown))
	location := townMap.GetNamedLocation(namedLocation)
	townMap.AddItem(item, location)
}
func (g *GameInstance) spawnObject(mapPos geometry.Point, object common.Object) {
	g.state.GetMap().AddObject(object, mapPos)
}

func (g *GameInstance) showLog() {
	buffer := g.logBuffer
	if len(buffer) > 20 {
		buffer = buffer[len(g.logBuffer)-20:]
	}
	g.uiController.OpenTextWindow(toGradientLabelText(buffer))
	//logWindow.ScrollToBottom()
}

type QuestFlags struct {
	Giver string
	Description string
	GivenFlag string
	GoalFlag string
	RewardedFlag string
}
func (g *GameInstance) showJournal() {
	var entries []util.LabelText

	flags := g.state.GetFlags()

	flagPairs := []QuestFlags {
		{"Kirioth (Town)","Kill Meat Jack (Level 2 - Cathedral)", "quest_received:meat_jack", "killed:meat_jack", "quest_rewarded:meat_jack"},
		{"Abel (Town)", "Clear the poisoned water (Catacombs)", "quest_received:poisoned_water", string("all_clear:" + SpecialLevelPoisonedWaterSupply), "quest_rewarded:poisoned_water"},
		{"Shekel (Town)","Kill King Zedek (Level 4 - Catacombs)", "quest_received:skeleton_king", "killed:skeleton_king", "quest_rewarded:skeleton_king"},
		{"Kirioth (Town)","Kill Lucifer (Level 8 - Hell)", "quest_received:lucifer", "killed:lucifer", "quest_rewarded:lucifer"},

		{"Lucifer (Hell)","Find evidence (Town)", "quest_received:evidence_one", "found:evidence_one", "quest_rewarded:evidence_one"},
		{"Lucifer (Hell)", "Find evidence (Catacombs)", "quest_received:evidence_two", "found:evidence_two", "quest_rewarded:evidence_two"},
		{"Lucifer (Hell)","Find evidence (Water Supply)", "quest_received:evidence_three", "found:evidence_three", "quest_rewarded:evidence_three"},
		{"Lucifer (Hell)","Find evidence (Caves)", "quest_received:evidence_four", "found:evidence_four", "quest_rewarded:evidence_four"},
	}
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
					TextWithColorCodes: fmt.Sprintf("%s - [:green]DONE", questName),
					Text:               fmt.Sprintf("%s - DONE", questName),
					TextColor:          common.DarkTextColor,
				})
			} else if flags.IsSet(flagPair.GoalFlag) {
				entries = append(entries, util.LabelText{
					UseColorCodes:      true,
					TextWithColorCodes: fmt.Sprintf("%s - [:yellow]IN PROGRESS", returnMessage),
					Text:               fmt.Sprintf("%s - IN PROGRESS", returnMessage),
					TextColor:          common.DarkTextColor,
				})
			} else {
				entries = append(entries, util.LabelText{
					UseColorCodes:      true,
					TextWithColorCodes: fmt.Sprintf("%s - [:yellow]IN PROGRESS", questName),
					Text:               fmt.Sprintf("%s - IN PROGRESS", questName),
					TextColor:          common.DarkTextColor,
				})
			}
		}
	}
	g.uiController.OpenTextWindow(entries)
	//logWindow.ScrollToBottom()
}

func (g *GameInstance) showFlags() {
	asStrings := g.state.GetFlags().AsStrings()
	g.uiController.OpenTextWindow(toLabelText(asStrings))
	//logWindow.ScrollToBottom()
}

func toGradientLabelText(buffer []string) []util.LabelText {
	// we want each item to be less white than the one before
	intensityFromIndex := func(index int) uint8 {
		maxIntensity := uint8(255)
		minIntensity := uint8(100)
		maxIndex := len(buffer) - 1
		percentage := 1.0 - (float64(index) / float64(maxIndex))
		return uint8(float64(maxIntensity) - (float64(maxIntensity)-float64(minIntensity))*percentage)
	}
	var result []util.LabelText
	for i, line := range buffer {
		fadedBaseColor := color.RGBA{R: intensityFromIndex(i), G: intensityFromIndex(i), B: intensityFromIndex(i), A: 255}
		baseColorCode := common.RGBAToColorCode(fadedBaseColor)
		result = append(result, util.LabelText{
			UseColorCodes:      true,
			TextWithColorCodes: baseColorCode + line,
			Text:               common.RemoveColorCodes(line),
			TextColor:          fadedBaseColor,
		})
	}
	return result
}

func toLabelText(buffer []string) []util.LabelText {
	var result []util.LabelText
	for _, line := range buffer {
		result = append(result, common.LabelTextFromString(line))
	}
	return result
}
func toLabelTextWithColor(buffer []string, textColor color.Color) []util.LabelText {
	var result []util.LabelText
	for _, line := range buffer {
		result = append(result, common.LabelTextFromString(line).WithColor(textColor))
	}
	return result
}

func (g *GameInstance) Log(text string) {
	println(text)
	if len(g.logBuffer) > 200 {
		g.logBuffer = g.logBuffer[1:]
	}
	g.logBuffer = append(g.logBuffer, text)

	g.message.text = text
	ticks := util.SecondsToTicks(3)
	g.message.ticksLeft = ticks
	g.message.ticksStart = ticks
}

func (g *GameInstance) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	panic("should use layoutf")
}

func (g *GameInstance) LayoutF(outsideWidth, outsideHeight float64) (screenWidth, screenHeight float64) {
	//g.deviceDPIScale = ebiten.DeviceScaleFactor()
	return outsideWidth * g.deviceDPIScale, outsideHeight * g.deviceDPIScale
}

func (g *GameInstance) GetTileScaleFactor() float64 {
	return g.tileScale
}
func (g *GameInstance) GetTitle() string {
	return g.title
}

func (g *GameInstance) SetTileRenderer(tileRenderer *renderer.TileRenderer) {
	g.tileRenderer = tileRenderer
}

func (g *GameInstance) SetMap(loadedMap *gridmap.GridMap[*common.Actor, common.Item, common.Object]) {
	oldMapName := ""
	oldMap := g.state.GetMap()
	if oldMap != nil {
		oldMapName = oldMap.GetName()
	}
	g.state.SetMap(loadedMap)
	mapWindow := renderer.NewMapWindow(
		g.GetDeviceIndependentScreenSize().Sub(geometry.Point{X:g.uiBarWidth, Y: 0}),
		geometry.Point{X: loadedMap.GetWidth(), Y: loadedMap.GetHeight()},
		g.worldTexture.GetTileSize(),
		g.GetTileScaleFactor,
		g.staticMapLookup)

	g.mapRenderer = renderer.NewMapRenderer(g.tileRenderer, mapWindow)
	g.mapRenderer.SetMiniMapColorHandler(g.getMiniMapColor)
	g.hybridRenderer = NewHybridRenderer(g.tileRenderer, g.mapRenderer)
	g.hybridRenderer.SetDefaultAtlasForMap(g.worldTexture)


	g.state.SetCamera(g.mapRenderer)
	g.onMapChanged(oldMapName, loadedMap.GetName())
}
func (g *GameInstance) AddBloodDecalAt(mapPos geometry.Point) {
	icon := int32(-1)
	currentMap := g.state.GetMap()
	if existingDecal, exists := currentMap.GetDecal(mapPos); exists {
		if existingDecal == common.IconDecalBlood5 {
			return
		}
		if existingDecal < common.IconDecalBlood1 || existingDecal > common.IconDecalBlood5 {
			return
		}
		icon = existingDecal + 1
	} else {
		bloodDecals := []int32{common.IconDecalBlood1, common.IconDecalBlood2, common.IconDecalBlood3, common.IconDecalBlood4, common.IconDecalBlood5}
		icon = bloodDecals[g.randomSource.Intn(len(bloodDecals))]
	}
	currentMap.AddDecal(mapPos, icon)
}
func prepend[T any](slice []T, s T) []T {
	return append([]T{s}, slice...)
}
func (g *GameInstance) staticMapLookup(x int, y int, tick uint64) []renderer.CellDrawInfo {
	mapPos := geometry.Point{X: x, Y: y}
	currentMap := g.state.GetMap()
	if currentMap == nil || !currentMap.Contains(mapPos) {
		return []renderer.CellDrawInfo{}
	}

	if !currentMap.IsExplored(mapPos) && !g.disableFogOfWar {
		return []renderer.CellDrawInfo{}
	}

	var drawColor color.RGBA = color.RGBA{255, 255, 255, 255}
	var worldTileColor color.RGBA = color.RGBA{255, 255, 255, 255}

	tileAt := currentMap.GetCell(mapPos).TileType
	if tileAt.Special == gridmap.SpecialTileWater {
		worldTileColor = g.getWaterColor()
	}
	worldIcon := tileAt.Icon()
	// animate the worldIcon
	if g.state.GetPlayer().CanSee(mapPos) || g.disableFogOfWar {
		if !g.configuration.DrawTilesBehindActors && currentMap.IsActorAt(mapPos) {
			return []renderer.CellDrawInfo{}
		}
		if info, ok := g.tilemapAnimations[worldIcon]; ok {
			// if it is, then calculate the current frame based on the tick
			frameOffset := util.GetLoopingFrameFromTick(tick+1000-uint64(x*17)-uint64(y*19), info.DelayInSeconds, info.FrameCount)
			worldIcon += frameOffset
		}

	}
	// fake lighting
	if !g.disableFogOfWar {
		drawColor = g.applyLighting(mapPos, drawColor)
		worldTileColor = g.applyLighting(mapPos, worldTileColor)
	}

	var drawInfo []renderer.CellDrawInfo

	// 1. items
	if itemAt, isItemAt := currentMap.TryGetItemAt(mapPos); isItemAt && (!itemAt.IsHidden() || g.state.GetFlags().IsSet(string(FlagHasTrueSight))) {
		itemIcon := itemAt.Icon(g.worldTicks)
		drawItem := renderer.CellDrawInfo{
			Atlas: g.worldTexture,
			Icon:  itemIcon,
			Color: drawColor,
		}
		drawInfo = prepend(drawInfo, drawItem)
		if !g.configuration.DrawTilesBehindItems {
			return drawInfo
		}
	}


	// 2. objects
	if currentMap.IsObjectAt(mapPos) {
		objectAt := currentMap.ObjectAt(mapPos)
		objIcon := objectAt.Icon(g.worldTicks)
		drawObject := renderer.CellDrawInfo{
			Atlas: g.worldTexture,
			Icon:  objIcon,
			Color: drawColor,
		}
		drawInfo = prepend(drawInfo, drawObject)
		if !g.configuration.DrawTilesBehindObjects {
			return drawInfo
		}
	}


	// 2. decals
	if decalIcon, exists := currentMap.GetDecal(mapPos); exists {
		drawInfo = prepend(drawInfo, renderer.CellDrawInfo{
			Atlas:    g.worldTexture,
			Icon:     decalIcon,
			Color:    color.White,
		})
	}

	// 1. world
	drawInfo = prepend(drawInfo, renderer.CellDrawInfo{
		Atlas: g.worldTexture,
		Icon:  worldIcon,
		Color: worldTileColor,
	})


	return drawInfo
}

func (g *GameInstance) applyLighting(mapPos geometry.Point, drawColor color.RGBA) color.RGBA {
	distanceToPlayer := geometry.Distance(g.state.GetPlayer().MapPosition(), mapPos)
	maxDistance := float64(g.GetPlayerVisionRange() + 1)
	darkPercentage := 0.16
	lightPercentage := 1.0
	if g.isDaylight() {
		darkPercentage = 0.8
		lightPercentage = 0.95
	}

	if distanceToPlayer > maxDistance || !g.state.GetPlayer().CanSee(mapPos) {
		drawColor = multiplyColor(drawColor, darkPercentage)
	} else {
		percentageLightness := float64(distanceToPlayer) / float64(maxDistance)
		percentageLightness = 1.0 - util.EaseInExpo(percentageLightness)
		//light := 255 // 1.0 -> 100% of original value
		//dark := 20   // 0.0 -> 12,75% of original value
		percentagOfValue := darkPercentage + ((lightPercentage - darkPercentage) * percentageLightness)
		if !g.isDaylight() {
			drawColor = g.mixLightSourceColor(drawColor, color.RGBA{127,127,127,255})
		}
		drawColor = multiplyColor(drawColor, percentagOfValue)
	}
	return drawColor
}

func (g *GameInstance) isDaylight() bool {
	return g.IsPlayerInTown() && !g.state.GetFlags().IsSet("town_changed_to_night")
}

func (g *GameInstance) mixLightSourceColor(drawColor, noLightSource color.RGBA) color.RGBA {
	equipment := g.GetWorldState().GetPlayer().GetEquipment()
	if equipment.HasLightSourceEquipped() {
		lightSource := equipment.GetLightSource().(*GameItem)
		if lightSource.HasChargesLeft() {
			lightStats := lightSource.GetLightStats()
			lightColor := lightStats.LightColor
			flickerPattern := lightStats.FlickerPattern
			delay := lightStats.FlickerFrameDelay
			flickerIndex := util.GetLoopingFrameFromTick(g.worldTicks, delay, len(flickerPattern))
			lightColor = multiplyColor(lightColor, flickerPattern[flickerIndex])
			drawColor = multiplyColors(drawColor, lightColor)
		}
	} else {
		drawColor = multiplyColors(drawColor, noLightSource)
	}
	return drawColor
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

func (g *GameInstance) SetTileMapAnimations(mapping map[int32]TilemapAnimInfo) {
	g.tilemapAnimations = mapping
}

func (g *GameInstance) SetWorldTexture(atlas renderer.TextureAtlas) {
	g.worldTexture = atlas
	g.tileRenderer.SetDefaultAtlas(atlas)
}

func (g *GameInstance) GetDeviceIndependentScreenSize() geometry.Point {
	if g.isFullScreen {
		w, h := ebiten.ScreenSizeInFullscreen()
		return geometry.Point{X: w, Y: h}
	}
	return g.deviceIndependentScreenSize
}

func (g *GameInstance) GetDeviceScaleFactor() float64 {
	return g.deviceDPIScale
}
func (g *GameInstance) OnScreenSizeChanged() {
	newScreenSize := g.GetDeviceIndependentScreenSize()
	g.uiController.OnScreenSizeChanged(newScreenSize)
	g.mapRenderer.OnScreenSizeChanged(newScreenSize.Sub(geometry.Point{X: g.uiBarWidth, Y: 0}))
	g.miniMap.Layout()
	if g.setToExactFit {
		g.disableScrolling()
	}
}
func (g *GameInstance) SetMapZoom(sign int) {

	change := 0.25 * float64(sign)
	minScale := 0.25
	maxScale := 16.0

	currentScale := g.tileScale
	if sign > 0 && g.setToExactFit {
		// round to next 0.25
		currentScale = math.Floor(currentScale*4) / 4
		g.setToExactFit = false
	}
	targetScale := util.Clamp(minScale, maxScale, currentScale+ change)

	mapSize := g.state.GetMap().MapSize()
	tileSize := g.worldTexture.GetTileSize()

	targetMapSize := geometry.Point{
		X: int(float64(mapSize.X) * float64(tileSize.X) * targetScale),
		Y: int(float64(mapSize.Y) * float64(tileSize.Y) * targetScale),
	}

	windowSize := g.GetDeviceIndependentScreenSize().Sub(geometry.Point{X:g.uiBarWidth})
	centerMapPos := g.mapRenderer.GetVisibleMap().Center()

	tooSmall := targetMapSize.X < windowSize.X && targetMapSize.Y < windowSize.Y
	if tooSmall {
		g.disableScrolling()
		return
	}

	g.setTileScale(targetScale)

	g.mapRenderer.CenterOn(centerMapPos)
}

func (g *GameInstance) disableScrolling() {
	g.setToExactFit = true
	mapSize := g.state.GetMap().MapSize()
	tileSize := g.worldTexture.GetTileSize()
	windowSize := g.GetDeviceIndependentScreenSize().Sub(geometry.Point{X:g.uiBarWidth})

	// calculate exact scale to fit
	mapSizeW := float64(mapSize.X) * float64(tileSize.X)
	mapSizeH := float64(mapSize.Y) * float64(tileSize.Y)
	targetScaleH := float64(windowSize.X) / mapSizeW
	targetScaleV := float64(windowSize.Y) / mapSizeH
	targetScale := min(targetScaleH, targetScaleV)
	g.mapRenderer.ResetScrolling()

	g.setTileScale(targetScale)
}

func (g *GameInstance) setTileScale(targetScale float64) {
	g.tileScale = targetScale
	g.Log(fmt.Sprintf("Set tile scale to %.2f", targetScale))
}

func DijkstraAoE(currentMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], maxDistance int) func(origin, target geometry.Point) []geometry.Point {
	return func(origin, target geometry.Point) []geometry.Point {
		return toList(currentMap.GetDijkstraMapWithActorsNotBlocking(target, maxDistance))
	}
}

func RayAoE(currentMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], maxDistance int) func(originGrid, targetGrid geometry.Point) []geometry.Point {
	return func(originGrid, targetGrid geometry.Point) []geometry.Point {
		var result []geometry.Point
		origin := originGrid.ToCenteredPointF()
		target := targetGrid.ToCenteredPointF()

		rayDirection := target.Sub(origin)
		hitInfo := currentMap.RayCast(origin, rayDirection, func(pos geometry.Point) bool {
			return !currentMap.IsWalkable(pos)
		})

		distCounter := 0
		for _, arrayPos := range hitInfo.TraversedGridCells {
			mapPos := geometry.Point{X: int(arrayPos[0]), Y: int(arrayPos[1])}
			if mapPos == originGrid {
				continue
			}
			result = append(result, mapPos)
			distCounter++
			if distCounter >= maxDistance {
				break
			}
		}
		return result
	}
}

func BresenhamAoE(currentMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], maxDistance int) func(originGrid, targetGrid geometry.Point) []geometry.Point {
	return func(originGrid, targetGrid geometry.Point) []geometry.Point {
		var result []geometry.Point
		traversedGridCells := currentMap.BresenhamLine(originGrid, targetGrid, func(pos geometry.Point) bool {
			return !currentMap.IsWalkable(pos)
		})

		distCounter := 0
		for _, mapPos := range traversedGridCells {
			if mapPos == originGrid {
				continue
			}
			result = append(result, mapPos)
			distCounter++
			if distCounter >= maxDistance {
				break
			}
		}
		return result
	}
}

func WallAoE(currentMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], wallParts int) func(origin, target geometry.Point) []geometry.Point {
	return func(origin, target geometry.Point) []geometry.Point {
		//                   x
		//                   x
		// player --> @      X <-- Target was here
		//                   x
		//                   x

		// we want a wall of fire on either the X or Y axis, above or below the below
		// depending on where he selected the target to be
		var result []geometry.Point

		directionToTarget := target.Sub(origin)
		halfParts := wallParts / 2
		// find the largest component of the direction vector to determine the axis

		if math.Abs(float64(directionToTarget.X)) > math.Abs(float64(directionToTarget.Y)) {
			// X axis
			// we want to place the wall left or right of the target
			// so check the direction of the X component
			if directionToTarget.X > 0 {
				// right
				for i := -halfParts; i <= halfParts; i++ {
					result = append(result, target.Add(geometry.Point{X: 0, Y: i}))
				}
			} else {
				// left
				for i := -halfParts; i <= halfParts; i++ {
					result = append(result, target.Add(geometry.Point{X: 0, Y: i}))
				}
			}
		} else {
			// Y axis
			// we want to place the wall above or below the target
			// so check the direction of the Y component
			if directionToTarget.Y > 0 {
				// below
				for i := -halfParts; i <= halfParts; i++ {
					result = append(result, target.Add(geometry.Point{X: i, Y: 0}))
				}
			} else {
				// above
				for i := -halfParts; i <= halfParts; i++ {
					result = append(result, target.Add(geometry.Point{X: i, Y: 0}))
				}
			}
		}

		return result
	}
}

func (g *GameInstance) BeginActionTargeting(action *common.TargetedAction) {
	if !g.state.GetPlayer().IsAlive() {
		return
	}
	rangedAttackRules := action.GetTargetingRules(g.state.GetPlayer(), g.state.GetMap())
	g.targeter = NewTargeter(g.hybridRenderer, rangedAttackRules, func(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
		action.OnSelected(attacker, target, hitPositions)
		autTaken := action.GetTimeNeededInAut()
		if autTaken > 0 {
			g.actionBroker.Execute(actions.NewEndPlayerTurnAction(actions.AbstractCause("EndPlayerTurnAction"), autTaken))
		}
	})
}

func (g *GameInstance) playerAimRangedAttack(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) { // TODO: HMM???
	g.CancelTargeting()
	origin := g.state.GetPlayer().MapPosition()
	if origin == target {
		g.Log("You can't attack yourself like that")
		return
	}
	g.actionBroker.CancelAllAnimations()
	g.playerRangedAttack(attacker, target)
}
func (g *GameInstance) rangedAttack(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
	rangedAttackAction := actions.NewRangedAttackAction(actions.NewActorCause("Ranged attack", attacker), attacker, target)
	g.actionBroker.Execute(rangedAttackAction)
}
func (g *GameInstance) playerAutoAttack() {
	if !g.state.GetPlayer().IsAlive() {
		return
	}
	var visibleEnemies []*common.Actor

	if g.shiftIsBeingPressedThisTick {
		for actor, _ := range g.state.GetVisibility().GetVisibleActors() {
			visibleEnemies = append(visibleEnemies, actor)
		}
	} else {
		visibleEnemies = g.state.GetVisibility().GetVisibleEnemies()
	}


	if len(visibleEnemies) == 0 {
		return
	}

	player := g.state.GetPlayer()
	hasRanged := player.GetEquipment().HasRangedWeaponEquipped()
	hasMelee := player.GetEquipment().HasMeleeWeaponEquipped()
	slices.SortStableFunc(visibleEnemies, func(i, j *common.Actor) int {
		distI := i.MapPosition().ManhattanDistanceTo(player.MapPosition())
		distJ := j.MapPosition().ManhattanDistanceTo(player.MapPosition())
		return cmp.Compare(distI, distJ)
	})
	currentMap := g.state.GetMap()
	for _, nearestEnemy := range visibleEnemies {
		dist := currentMap.MoveDistance(player.MapPosition(), nearestEnemy.MapPosition())
		if dist <= 1 {
			if hasMelee || !hasRanged {
				g.playerMeleeAttack(player, nearestEnemy.MapPosition())
				return
			} else if hasRanged {
				g.playerRangedAttack(player, nearestEnemy.MapPosition())
				return
			}
		}

		if hasRanged {
			rangedRules := g.getRangedAttackRules(player)
			if rangedRules.IsValidTarget(nearestEnemy.MapPosition()) {
				g.playerRangedAttack(player, nearestEnemy.MapPosition())
				return
			}
		}

		// do melee
		freeAttackPositions := currentMap.GetFilteredNeighborsForMovement(nearestEnemy.MapPosition(), func(point geometry.Point) bool {
			return currentMap.IsCurrentlyPassable(point)
		})

		if len(freeAttackPositions) == 0 {
			continue
		}
		distMap := make([]util.Tuple[geometry.Point, int], len(freeAttackPositions))
		for _, freeAttackPosition := range freeAttackPositions {
			distanceTo := currentMap.MoveDistance(player.MapPosition(), freeAttackPosition)
			distMap = append(distMap, util.NewTuple(freeAttackPosition, distanceTo))
		}
		slices.SortStableFunc(distMap, func(i, j util.Tuple[geometry.Point, int]) int {
			return cmp.Compare(i.GetItem2(), j.GetItem2())
		})

		for _, distTuple := range distMap {
			targetPos := distTuple.GetItem1()
			walkPath := currentMap.GetJPSPath(player.MapPosition(), targetPos, currentMap.CurrentlyPassableAndSafeForActor(player))
			if len(walkPath) <= 1 {
				continue
			}
			// remove first pos
			walkPath = walkPath[1:]
			firstStep := walkPath[0]

			g.playerMoveOneStep(player, firstStep)
			return
		}
	}

}
func (g *GameInstance) playerMoveOneStep(player *common.Actor, targetPos geometry.Point) {
	moveAction := actions.NewMoveAction(actions.NewActorCause("Move", player), player, targetPos, true)
	g.actionBroker.Execute(moveAction)
	g.actionBroker.Execute(actions.NewEndPlayerTurnAction(moveAction.Cause(), moveAction.GetDelayInAut()))
}
func (g *GameInstance) playerMeleeAttack(player *common.Actor, targetPos geometry.Point) {
	meleeAction := actions.NewMeleeAttackAction(actions.NewActorCause("Melee attack", player), player, targetPos)
	g.actionBroker.Execute(meleeAction)
	g.actionBroker.Execute(actions.NewEndPlayerTurnAction(meleeAction.Cause(), meleeAction.GetDelayInAut()))
}
func (g *GameInstance) playerRangedAttack(player *common.Actor, targetPos geometry.Point) {
	rangedAttackAction := actions.NewRangedAttackAction(actions.NewActorCause("Ranged attack", player), player, targetPos)
	g.actionBroker.Execute(rangedAttackAction)
	g.actionBroker.Execute(actions.NewEndPlayerTurnAction(rangedAttackAction.Cause(), rangedAttackAction.GetDelayInAut()))
}

func (g *GameInstance) getRangedAttackRules(player *common.Actor) *RangedTargetingRules {
	targetRules := NewRangedAttackTargetingRules(player, OneTargetTile, g.state.GetMap())
	rangedAttackRules := targetRules.(*RangedTargetingRules)
	return rangedAttackRules
}
func (g *GameInstance) manualMovePlayer(direction geometry.Point) {
	if !g.state.GetPlayer().IsAlive() {
		return
	}
	g.CancelTargeting()
	g.cancelAutoMove()
	//g.actionBroker.CancelAllAnimations()
	playerAction := actions.PlayerMoveToAttack(g.state.GetPlayer(), direction, g.shiftIsBeingPressedThisTick, g)
	if playerAction == nil {
		return
	}

	g.actionBroker.Execute(playerAction)
	g.actionBroker.Execute(actions.NewEndPlayerTurnAction(playerAction.Cause(), playerAction.GetDelayInAut()))
}

func (g *GameInstance) SetFontWithScale(font renderer.BitmapFont, scale float64) {
	g.tileRenderer.SetFont(font)
	g.tileRenderer.SetFontScale(scale)
}

func (g *GameInstance) SetTTFFont(file io.ReaderAt, size float64) {
	tt, err := opentype.ParseReaderAt(file)
	if err != nil {
		println(err.Error())
		return
	}
	dpi := 72 * g.deviceDPIScale
	fontFace, faceErr := opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    size,
		DPI:     dpi,
		Hinting: font.HintingVertical,
	})
	if faceErr != nil {
		println(faceErr.Error())
	}
	//mplusBigFont = text.FaceWithLineHeight(mplusBigFont, 54) // adjust line height
	g.tileRenderer.SetTTF(fontFace)
}

func (g *GameInstance) GetWorldState() actions.WorldState {
	return &g.state
}

func (g *GameInstance) showDebugMenu() {
	g.uiController.OpenMenu([]util.MenuItem{
		{
			Text: "Map Info",
			Action: func() {
				g.uiController.OpenTextWindow(toLabelText(g.state.GetMap().GetMetaString()))
			},
		},
		{
			Text: "Debug Map",
			Action: func() {
				screenSize := g.GetDeviceIndependentScreenSize()
				visibleMap := g.mapRenderer.GetVisibleMap()
				g.Log(fmt.Sprintf("Visible map: (%d, %d) - (%d, %d)", visibleMap.Min.X, visibleMap.Min.Y, visibleMap.Max.X, visibleMap.Max.Y))
				g.Log(fmt.Sprintf("Screen size: (%d, %d)", screenSize.X, screenSize.Y))
			},
		},
		{
			Text: "Trigger nightfall in town",
			Action: func() {
				g.state.GetFlags().SetTrue("town_changed_to_night")
			},
		},
		{
			Text: "Trigger death of Lucifer",
			Action: func() {
				g.onLuciferDeath()
			},
		},
		{
			Text: "Trigger completion of poisoned water supply",
			Action: func() {
				g.state.GetFlags().SetTrue(string("all_clear:" + SpecialLevelPoisonedWaterSupply))
			},
		},
		{
			Text: "Open Stash",
			Action: g.openPlayerStashInTown,
		},
		{
			Text: "give stuff",
			Action: func() {
				playerStats := g.state.GetPlayer().GetStats()
				playerStats.SetAttribute(stats.MaxHealth, 10000)
				playerStats.GetRessource(stats.Health).Increase(10000)
				playerStats.SetAttribute(stats.MaxMana, 10000)
				playerStats.GetRessource(stats.Mana).Increase(10000)
				playerStats.SetAttribute(stats.Gold, 10000)

				inv := g.state.GetPlayer().GetInventory()
				for i := 0; i < 10; i++ {
					inv.Add(g.NewHealingPotion())
					inv.Add(g.NewManaPotion())
				}
				g.updateStatsLabel()
			},
		},
		{
			Text: "give 10k XP",
			Action: func() {
				playerStats := g.state.GetPlayer().GetStats()
				playerStats.SetAttribute(stats.Experience, 10000)
				g.updateStatsLabel()
			},
		},
	})
}

func (g *GameInstance) killLevel() {
	actorsOnMap := g.state.GetMap().Actors()
	for i := len(actorsOnMap) - 1; i >= 0; i-- {
		actor := actorsOnMap[i]
		if actor == g.state.GetPlayer() || !actor.IsAlive() || actor.IsUnique() {
			continue
		}
		currentHP := actor.GetStats().GetRessource(stats.Health).Value() * 2
		g.actionBroker.Execute(actions.NewChangeRessourceAction(actions.NewActorCause("Kill All", g.state.GetPlayer()), actor, stats.Health, -currentHP))
	}
}

func (g *GameInstance) addStatsLabel(offset geometry.Point) *ui.BasicLabel {
	label := g.uiController.AddLabel(offset)
	return label
}
func (g *GameInstance) addLevelUpButton(offset geometry.Point) *ui.ButtonOverlay {
	button := g.uiController.AddButton(offset, g.worldTexture.GetTileSize(), 141, g.openLevelUpMenu)
	button.SetTooltip("Level up")
	button.Hide()
	return button
}

func (g *GameInstance) addMiniMap(offset geometry.Point, maxHeight int, horizontalPadding int) *ui.MinimapOverlay {
	miniMap := g.uiController.AddMinimap(offset.Sub(geometry.Point{X: horizontalPadding}), maxHeight, offset.X - (horizontalPadding*2), g.mapRenderer.GetMiniMap())
	miniMap.SetOnMapClickedHandler(g.onMapClicked)
	return miniMap
}
func (g *GameInstance) updateStatsLabel() {
	player := g.state.GetPlayer()
	playerStats := player.GetStats()
	var hpColor color.Color
	var manaColor color.Color
	hpColor = common.DarkTextColor
	manaColor = common.DarkTextColor

	if playerStats.GetRessource(stats.Health).IsLessThanPercent(0.25) {
		hpColor = color.RGBA{R: 255, G: 30, B: 30, A: 255}
	} else if playerStats.GetRessource(stats.Health).IsLessThanPercent(0.5) {
		hpColor = color.RGBA{R: 255, G: 150, B: 30, A: 255}
	} else if playerStats.GetRessource(stats.Health).IsLessThanPercent(0.75) {
		hpColor = color.RGBA{R: 255, G: 255, B: 30, A: 255}
	}

	if playerStats.GetRessource(stats.Mana).IsLessThanPercent(0.25) {
		manaColor = color.RGBA{R: 255, G: 30, B: 30, A: 255}
	} else if playerStats.GetRessource(stats.Mana).IsLessThanPercent(0.5) {
		manaColor = color.RGBA{R: 255, G: 150, B: 30, A: 255}
	} else if playerStats.GetRessource(stats.Mana).IsLessThanPercent(0.75) {
		manaColor = color.RGBA{R: 255, G: 255, B: 30, A: 255}
	}
	label := g.statsLabel
	mapNameParts := toLabelTextWithColor(strings.Split(g.state.GetMap().GetDisplayName(), "\n"), common.DarkTextColor)
	statusParts := []util.LabelText{
		{
			Text:      fmt.Sprintf("HP: %s", playerStats.GetRessource(stats.Health).ToString()),
			TextColor: hpColor,
		},
		{
			Text:      fmt.Sprintf("MP: %s", playerStats.GetRessource(stats.Mana).ToString()),
			TextColor: manaColor,
		},
	}
	label.SetText(append(mapNameParts, statusParts...))
	g.miniMap.Show()
}

func (g *GameInstance) ShowFullStats() {
	player := g.state.GetPlayer()
	playerStats := player.GetStats()
	var hpColor color.Color
	var manaColor color.Color
	defaultColor := common.DarkTextColor
	hpColor = defaultColor
	manaColor = defaultColor
	colorFromModifierType := func(mType stats.ModifierImpact) color.Color {
		switch mType {
		case stats.ModifierImpactPositive:
			return common.PositiveHighlightColor
		case stats.ModifierImpactNegative:
			return common.NegativeHighlightColor
		default:
			return defaultColor
		}
	}
	if playerStats.GetRessource(stats.Health).IsLessThanPercent(0.25) {
		hpColor = color.RGBA{R: 255, G: 30, B: 30, A: 255}
	} else if playerStats.GetRessource(stats.Health).IsLessThanPercent(0.5) {
		hpColor = color.RGBA{R: 255, G: 150, B: 30, A: 255}
	} else if playerStats.GetRessource(stats.Health).IsLessThanPercent(0.75) {
		hpColor = color.RGBA{R: 255, G: 255, B: 30, A: 255}
	}

	if playerStats.GetRessource(stats.Mana).IsLessThanPercent(0.25) {
		manaColor = color.RGBA{R: 255, G: 30, B: 30, A: 255}
	} else if playerStats.GetRessource(stats.Mana).IsLessThanPercent(0.5) {
		manaColor = color.RGBA{R: 255, G: 150, B: 30, A: 255}
	} else if playerStats.GetRessource(stats.Mana).IsLessThanPercent(0.75) {
		manaColor = color.RGBA{R: 255, G: 255, B: 30, A: 255}
	}
	levelColor := defaultColor
	if player.CanLevelUp() {
		levelColor = common.LightTextColor
	}
	g.levelUpButton.Hide()
	g.miniMap.Hide()

	label := g.statsLabel

	label.SetText([]util.LabelText{
		{
			Text:      fmt.Sprintf("LVL: %d", playerStats.GetAttribute(stats.Level)),
			TextColor: levelColor,
		},
		{
			Text:      fmt.Sprintf("XP: %d", playerStats.GetAttribute(stats.Experience)),
			TextColor: levelColor,
		},
		{
			Text:      fmt.Sprintf("Next: %d", player.GetXPForLevel(playerStats.GetAttribute(stats.Level)+1)),
			TextColor: levelColor,
		},
		{
			Text:      fmt.Sprintf("HP: %s", playerStats.GetRessource(stats.Health).ToString()),
			TextColor: hpColor,
		},
		{
			Text:      fmt.Sprintf("MP: %s", playerStats.GetRessource(stats.Mana).ToString()),
			TextColor: manaColor,
		},
		{
			Text:      fmt.Sprintf("STR: %d", playerStats.GetAttribute(stats.Strength)),
			TextColor: colorFromModifierType(playerStats.GetModifierType(stats.Strength)),
		},
		{
			Text:      fmt.Sprintf("INT: %d", playerStats.GetAttribute(stats.Intelligence)),
			TextColor: colorFromModifierType(playerStats.GetModifierType(stats.Intelligence)),
		},
		{
			Text:      fmt.Sprintf("DEX: %d", playerStats.GetAttribute(stats.Dexterity)),
			TextColor: colorFromModifierType(playerStats.GetModifierType(stats.Dexterity)),
		},
		{
			Text:      fmt.Sprintf("AC: %d", player.GetArmorClass()),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("ADR: %d%%", int(player.GetArmorDamageReduction(0)*100)),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("MELEE DMG: %s", player.GetDamageInterval(common.MeleeAttack).ToString()),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("RANGED DMG: %s", player.GetDamageInterval(common.RangedAttack).ToString()),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("GOLD: %d", player.GetStats().GetAttribute(stats.Gold)),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("DMG MULT: %d%%", int(player.GetStats().GetAttributeAsFloat(stats.EnhancedDamageFactor)*100)),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("AC MULT: %d%%", int(player.GetStats().GetAttributeAsFloat(stats.EnhancedArmorClassFactor)*100)),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("HP REGEN: %d%%", int(player.GetStats().GetAttributeAsFloat(stats.HealthRegenerationFactor)*100)),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("MP REGEN: %d%%", int(player.GetStats().GetAttributeAsFloat(stats.ManaRegenerationFactor)*100)),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("HP STEAL: %d%%", int(player.GetStats().GetAttributeAsFloat(stats.LifeStealFactor)*100)),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("MP STEAL: %d%%", int(player.GetStats().GetAttributeAsFloat(stats.ManaStealFactor)*100)),
			TextColor: defaultColor,
		},
	})
}


func (g *GameInstance) addInventoryOverlay(pixelEdgeFromBottomRight geometry.Point) *ui.IconOverlay {
	isEquipped := func(uiItem ui.UIElement) bool {
		player := g.state.GetPlayer()
		itemStack := uiItem.(common.InventoryStack)
		firstItem := itemStack[0]
		return player.GetEquipment().IsEquipped(firstItem)
	}

	dataSource := ui.IconDataSource{
		IsHighlighted: isEquipped,
		GetElements: func() []ui.UIElement {
			return sortInventoryForUI(g.state.GetPlayer().GetInventory())
		},
		OnSelection:   g.onItemInInventoryClicked,
	}
	tileSize := g.worldTexture.GetTileSize()
	return g.uiController.OpenInventoryOverlay(pixelEdgeFromBottomRight, tileSize, dataSource)
}

func (g *GameInstance) openPlayerStashInTown() {
	stashInTown := g.state.GetPlayerStashInTown()
	playerInv := g.state.GetPlayer().GetInventory()
	playerEquip := g.state.GetPlayer().GetEquipment()

	toStashHandler := func(uiItem ui.UIElement) {
		itemStack := uiItem.(common.InventoryStack)
		firstItem := itemStack[0]
		typedItem := firstItem.(*GameItem)
		if playerEquip.IsEquipped(typedItem) { // unequip first..
			g.actionBroker.Execute(actions.NewUnequipItemAction(actions.NewActorCause("Unequip", g.state.GetPlayer()), g.state.GetPlayer(), typedItem))
		}
		playerInv.Remove(typedItem)
		if g.shiftIsBeingPressedThisTick {
			typedItem.DroppedByPlayer()
			g.AddItemToMapWithDistribution(typedItem, g.state.GetPlayer().MapPosition())
		} else {
			stashInTown.Add(typedItem)
		}
	}

	dataSource := ui.IconDataSource{
		IsHighlighted: func(element ui.UIElement) bool {
			return false
		},
		GetElements: func() []ui.UIElement {
			return sortInventoryForUI(stashInTown)
		},
		OnSelection: func(uiItem ui.UIElement) {
			itemStack := uiItem.(common.InventoryStack)
			firstItem := itemStack[0]
			typedItem := firstItem.(*GameItem)
			stashInTown.Remove(typedItem)

			if g.shiftIsBeingPressedThisTick { // drop
				typedItem.DroppedByPlayer()
				g.AddItemToMapWithDistribution(typedItem, g.state.GetPlayer().MapPosition())
			} else { // move
				playerInv.Add(typedItem)
			}
		},
	}

	tileSize := g.worldTexture.GetTileSize()

	screenSize := g.GetDeviceIndependentScreenSize()
	invBounds := g.inventory.GetBounds()
	paddingX := 20
	offsetFromBottomRightCorner := geometry.Point{X: screenSize.X - invBounds.Max.X + invBounds.Size().X + paddingX, Y: screenSize.Y - invBounds.Max.Y} //- invBounds.Size().X - paddingX, Y: invBounds.Min.Y}

	stashInv := g.uiController.OpenInventoryModal(offsetFromBottomRightCorner, tileSize, dataSource)

	oldHandler := g.inventory.GetOnSelectedHandler()
	g.inventory.SetOnSelectedHandler(toStashHandler)

	stashInv.SetOnCloseHandler(func() {
		g.inventory.SetOnSelectedHandler(oldHandler)
	})

	// on close of the stash modal, restore the onclick handler of the inventory..
}
func sortInventoryForUI(inv *common.Inventory) []ui.UIElement {
	items := make([]ui.UIElement, 0)
	stacks := inv.StackedItems()
	slices.SortStableFunc(stacks, func(i, j common.InventoryStack) int {
		itemI := i[0]
		itemTypeOne := itemI.GetItemType()
		itemJ := j[0]
		itemTypeTwo := itemJ.GetItemType()
		if itemTypeOne != itemTypeTwo {
			return cmp.Compare(itemTypeOne, itemTypeTwo)
		} else {
			switch itemTypeOne {
			case common.ItemCategoryWeapon:
				weaponPartOne := itemI.GetWeaponStats()
				weaponPartTwo := itemJ.GetWeaponStats()
				return cmp.Compare(weaponPartOne.Type, weaponPartTwo.Type)
			case common.ItemCategoryArmor:
				armorPartOne := itemI.GetArmorStats()
				armorPartTwo := itemJ.GetArmorStats()
				return cmp.Compare(armorPartOne.Type, armorPartTwo.Type)
			case common.ItemCategoryWand:
				fallthrough
			case common.ItemCategoryPotion:
				fallthrough
			case common.ItemCategoryScroll:
				fallthrough
			default:
				return cmp.Compare(itemI.Name(), itemJ.Name())
			}
		}
	})
	for _, item := range stacks {
		items = append(items, item)
	}
	return items
}
func (g *GameInstance) addSkillsOverlay(pixelEdgeFromBottomRight geometry.Point) *ui.IconOverlay {
	tileSize := g.worldTexture.GetTileSize()

	getList := func() []ui.UIElement {
		player := g.state.GetPlayer()
		items := make([]ui.UIElement, 0)
		for _, item := range g.GetActiveSkillsForUI(player) {
			items = append(items, item)
		}
		for _, item := range g.GetSustainedSkills(player) {
			items = append(items, item)
		}
		return items
	}
	isEquipped := func(item ui.UIElement) bool {
		return false // TODO:
	}

	dataSource := ui.IconDataSource{
		IsHighlighted: isEquipped,
		GetElements:   getList,
		OnSelection: func(element ui.UIElement) {
			player := g.state.GetPlayer()
			switch typedElement := element.(type) {
			case *common.UntargetedAction:
				g.activateUntargetedAction(player, typedElement)
			case *common.TargetedAction:
				g.BeginActionTargeting(typedElement)
			case  *SustainedAction:
				//TODO?
				//g.actionBroker.Execute(actions.NewToggleSustainedAction(actions.AbstractCause("ToggleSustainedAction"), player, typedElement))
				typedElement.TryActivate(player)
			}
		},
	}

	return g.uiController.OpenSkillsOverlay(pixelEdgeFromBottomRight, tileSize, dataSource)
}
func (g *GameInstance) activateItemActionForPlayer(itemWithAction *GameItem) {
	user := g.state.GetPlayer() // an AI actor needs to provide a target pos for targeted actions.. handle this elsewhere..
	if !user.CanUseItems() || !itemWithAction.HasAction() {
		return
	}

	if !itemWithAction.HasChargesLeft() {
		g.Log(fmt.Sprintf("%s is out of charges", itemWithAction.Name()))
		return
	}

	itemWithAction.UseCharge()

	if !itemWithAction.HasChargesLeft() && itemWithAction.SelfDestructs() {
		g.GetWorldState().GetPlayer().GetInventory().Remove(itemWithAction)
	}

	if itemWithAction.HasTargetedAction() {
		g.BeginActionTargeting(itemWithAction.GetTargetedAction())
	} else if itemWithAction.HasUntargetedAction() {
		untargetedAction := itemWithAction.GetUntargetedAction()
		g.activateUntargetedAction(user, untargetedAction)
		autTaken := untargetedAction.GetTimeNeededInAut()
		if autTaken > 0 {
			g.actionBroker.Execute(actions.NewEndPlayerTurnAction(actions.AbstractCause("ActiveItemUse"), autTaken))
		}
	}
}
func (g *GameInstance) addTimerOverlay(pixelEdgeFromBottomRight geometry.Point) *ui.IconOverlay {
	tileSize := g.worldTexture.GetTileSize()

	isEquipped := func(item ui.UIElement) bool {
		return false // TODO:
	}

	dataSource := ui.IconDataSource{
		IsHighlighted: isEquipped,
		GetElements:   g.getCurrentTimedEffects,
		OnSelection: func(element ui.UIElement) {
		},
	}

	return g.uiController.OpenTimerOverlay(pixelEdgeFromBottomRight, tileSize, dataSource)
}

func (g *GameInstance) SetTimeKeeper(keeper *TimeKeeper) {
	g.timekeeper = keeper
}

// getAIAction returns the action to execute and the cost in AUT associated with it
func (g *GameInstance) getAIAction(actor *common.Actor) (int, func()) {
	autCostMove := 100 / actor.GetMoveSpeed()
	autCostMeleeAttack := 100 / actor.GetAttackSpeed(common.MeleeAttack)
	autCostRangedAttack := 100 / actor.GetAttackSpeed(common.RangedAttack)
	minCostAttack := min(autCostMeleeAttack, autCostRangedAttack)

	minCost := min(autCostMove, minCostAttack)
	if actor.GetEnergy() < minCost { // don't act at all, conserve energy
		return 0, nil
	}
	if actor.IsVendor() { // vendors just wait by default
		return 10, func() {}
	}

	canAct := !actor.HasStatusEffect(common.StatusStunned)
	if !canAct {
		println(fmt.Sprintf("%s is still unable to act. (Stunned)", actor.Name()))
		return 10, func() {}
	}

	canMove := actor.GetEnergy() >= autCostMove && !actor.HasStatusEffect(common.StatusImmobile)
	canAttack := actor.GetEnergy() >= minCostAttack && !actor.HasStatusEffect(common.StatusPacified)


	if !canMove && !canAttack {
		println(fmt.Sprintf("%s can neither move nor attack. (Immobile & Pacified)", actor.Name()))
		return 10, func() {}
	}

	isScared := actor.HasStatusEffect(common.StatusScared)
	if isScared && canMove {
		return autCostMove, func() {
			targetPos := g.GetMoveAwayFromPlayer(actor.MapPosition())
			if g.IsClosedDoorAt(targetPos) {
				g.actionBroker.Execute(actions.NewOpenDoorAction(actions.AbstractCause("OpenDoor"), actor, targetPos))
			}
			g.actionBroker.Execute(actions.NewMoveAction(actions.AbstractCause("Movement"), actor, targetPos, false))
		}
	}


	if actor.GetState() == common.ActorStateIdle && canMove {
		return autCostMove, func() {
			freeNeighbor := g.state.GetMap().GetRandomFreeAndSafeNeighbor(g.randomSource, actor.MapPosition())
			if freeNeighbor != actor.MapPosition() {
				if g.IsClosedDoorAt(freeNeighbor) {
					g.actionBroker.Execute(actions.NewOpenDoorAction(actions.AbstractCause("OpenDoor"), actor, freeNeighbor))
				}
				g.actionBroker.Execute(actions.NewMoveAction(actions.AbstractCause("Movement"), actor, freeNeighbor, false))
			}
		}
	}

	if actor.GetState() == common.ActorStateAttacking && g.state.GetPlayer().IsAlive() {

		// can use a skill?

		targetedSkills := actor.GetTargetedSkills()
		playerPosition := g.state.GetPlayer().MapPosition()
		if len(targetedSkills) > 0 {
			for _, skill := range targetedSkills {
				targetingRules := skill.GetTargetingRules(actor, g.GetWorldState().GetMap())
				if targetingRules.IsValidTarget(playerPosition) {
					return skill.GetTimeNeededInAut(), func() {
						skill.Execute(actor, actor.MapPosition(), playerPosition)
					}
				}
			}
		}



		canRangedAttack := actor.GetEnergy() >= autCostRangedAttack && actor.GetEquipment().HasRangedWeaponEquipped() && !actor.HasStatusEffect(common.StatusPacified)
		canSeePlayer := actor.CanSee(playerPosition)
		if canRangedAttack && canSeePlayer {
			return autCostRangedAttack, func() {
				g.actionBroker.Execute(actions.NewRangedAttackAction(actions.AbstractCause("RangedAttack"), actor, playerPosition))
			}
		}
		var dist int
		dist = g.state.GetMap().MoveDistance(actor.MapPosition(), playerPosition)
		if dist <= 1 && canAttack {
			return autCostMeleeAttack, func() {
				g.actionBroker.Execute(actions.NewMeleeAttackAction(actions.AbstractCause("MeleeAttack"), actor, playerPosition))
			}
		} else if canMove {
			return autCostMove, func() {
				targetPos := g.GetMoveTowardsPlayer(actor.MapPosition())
				if g.IsClosedDoorAt(targetPos) {
					g.actionBroker.Execute(actions.NewOpenDoorAction(actions.AbstractCause("OpenDoor"), actor, targetPos))
				}
				g.actionBroker.Execute(actions.NewMoveAction(actions.AbstractCause("Movement"), actor, targetPos, false))
			}
		}
	}

	return 10, func() {}
}

func (g *GameInstance) onMapClicked(mouseButton ebiten.MouseButton, mapPos geometry.Point) {
	if g.uiController.IsUIVisible() {
		return
	}
	currentMap := g.state.GetMap()
	if mouseButton == ebiten.MouseButtonRight && currentMap.IsActorAt(mapPos) && g.state.GetPlayer().CanSee(mapPos) {
		g.showActorInfoAt(mapPos)
	} else if mouseButton == ebiten.MouseButtonLeft && currentMap.IsExplored(mapPos) {
		g.cancelAutoMove()
		g.startPlayerAutoMovement(mapPos)
	}
}

func (g *GameInstance) allowedForAutoExplore(point geometry.Point) bool {
	currentMap := g.state.GetMap()
	isWalkable := currentMap.CurrentlyPassableAndSafeForActor(g.state.GetPlayer())
	return isWalkable(point) && !currentMap.IsStairsAt(point)
}
func (g *GameInstance) allowedForAutoMove(targetPos geometry.Point) func(point geometry.Point) bool {
	return func(point geometry.Point) bool {
		currentMap := g.state.GetMap()
		isWalkable := currentMap.CurrentlyPassableAndSafeForActor(g.state.GetPlayer())
		return isWalkable(point) || point == targetPos
	}
}

func (g *GameInstance) startPlayerAutoMovement(targetPos geometry.Point) {
	player := g.state.GetPlayer()
	if !player.IsAlive() {
		return
	}
	currentMap := g.state.GetMap()
	if len(g.GetWorldState().GetVisibility().GetVisibleEnemies()) > 0 {
		// player is trying to move while enemies are visible..
		if currentMap.MoveDistance(player.MapPosition(), targetPos) == 1 {
			// this seems to be an intended action..
			direction := targetPos.Sub(player.MapPosition())
			g.manualMovePlayer(direction)
			return
		} else {
			g.Log("You can't auto-move while enemies are visible")
			return
		}
	}
	playerPosition := player.MapPosition()

	// check for item directly beneath us.. if so, pick it up
	if g.configuration.AutoPickup {
		if currentMap.IsItemAt(playerPosition) {
			itemAt := currentMap.ItemAt(playerPosition)
			if !itemAt.IsDroppedByPlayer() {
				pickUpAction := actions.NewPickUpAction(actions.AbstractCause("PickUp"), player)
				g.actionBroker.Execute(pickUpAction)
				g.actionBroker.Execute(actions.NewEndPlayerTurnAction(pickUpAction.Cause(), pickUpAction.GetDelayInAut()))

			}
		}
	}

	var isWalkable func(point geometry.Point) bool
	if g.autoExplore {
		isWalkable = g.allowedForAutoExplore
	} else {
		isWalkable = g.allowedForAutoMove(targetPos)
	}
	pathToTarget := currentMap.GetJPSPath(playerPosition, targetPos, isWalkable)
	if len(pathToTarget) <= 1 {
		g.stopAutoMove()
		return
	}
	g.actionBroker.CancelAllAnimations()
	g.actionBroker.ClearQueuedActions(func(action actions.Action) bool {
		if moveAction, ok := action.(*actions.MoveAction); ok {
			return moveAction.GetActor() == player
		}
		return false
	})
	g.playerPath = pathToTarget[1:]
	g.nextStepOnPlayerPath()
}

func (g *GameInstance) continuePlayerAutoMovement(targetPos geometry.Point) {
	currentMap := g.state.GetMap()
	var isWalkable func(point geometry.Point) bool
	if g.autoExplore {
		isWalkable = g.allowedForAutoExplore
	} else {
		isWalkable = g.allowedForAutoMove(targetPos)
	}
	pathToTarget := currentMap.GetJPSPath(g.state.GetPlayer().MapPosition(), targetPos, isWalkable)
	if len(pathToTarget) <= 1 {
		return
	}
	g.playerPath = pathToTarget[1:]
	g.nextStepOnPlayerPath()
}

func (g *GameInstance) showActorInfoAt(mapPos geometry.Point) {
	player := g.state.GetPlayer()
	actor := g.state.GetMap().ActorAt(mapPos)
	infoText := actor.GetInfoText()

	if actor != player {
		otherHasRanged := actor.GetEquipment().HasRangedWeaponEquipped()
		playerHasRanged := player.GetEquipment().HasRangedWeaponEquipped()

		playerMelee := actions.NewMeleeAttackAction(actions.AbstractCause("Simulation"), player, actor.MapPosition())
		damage := playerMelee.SimulateAttack(g)
		if damageInterval, ok := damage[actor]; ok {
			infoText = append(infoText, "", fmt.Sprintf("YOU WILL DEAL %s DMG. PER MELEE ATTACK", damageInterval.ToString()))
		}

		if playerHasRanged {
			playerRanged := actions.NewRangedAttackAction(actions.AbstractCause("Simulation"), player, actor.MapPosition())
			damage = playerRanged.SimulateAttack(g)
			if damageInterval, ok := damage[actor]; ok {
				infoText = append(infoText, "", fmt.Sprintf("YOU WILL DEAL %s DMG. PER RANGED ATTACK", damageInterval.ToString()))
			}
		}

		enemyMelee := actions.NewMeleeAttackAction(actions.AbstractCause("Simulation"), actor, player.MapPosition())
		damage = enemyMelee.SimulateAttack(g)
		if damageInterval, ok := damage[player]; ok {
			infoText = append(infoText, "", fmt.Sprintf("ENEMY WILL DEAL %s DMG. PER MELEE ATTACK", damageInterval.ToString()))
		}

		if otherHasRanged {
			enemyRanged := actions.NewRangedAttackAction(actions.AbstractCause("Simulation"), actor, player.MapPosition())
			damage = enemyRanged.SimulateAttack(g)
			if damageInterval, ok := damage[player]; ok {
				infoText = append(infoText, "", fmt.Sprintf("ENEMY WILL DEAL %s DMG. PER RANGED ATTACK", damageInterval.ToString()))
			}
		}
	}

	g.uiController.OpenTextWindow(toLabelText(infoText))
}

type Enchantment interface {
	ToString() string
}

func (g *GameInstance) TransferRandomEnchantment(source *GameItem, destination *GameItem) {
	enchantments := source.GetAllEnchantments()

	if len(enchantments) == 0 {
		return
	}

	index := rand.Intn(len(enchantments))
	randomEnchantment := enchantments[index]

	source.RemoveEnchantment(randomEnchantment)
	destination.AddEnchantment(randomEnchantment)
}

func (g *GameInstance) onItemInInventoryClicked(uiItem ui.UIElement) {
	if !g.state.GetPlayer().IsAlive() {
		return
	}

	// shift state
	shiftPressed := ebiten.IsKeyPressed(ebiten.KeyShift)
	itemStack := uiItem.(common.InventoryStack)
	firstItem := itemStack[0]
	typedItem := firstItem.(*GameItem)

	player := g.state.GetPlayer()

	wasEquipped := player.GetEquipment().IsEquipped(typedItem)
	if shiftPressed { // drop
		g.actionBroker.Execute(actions.NewDropAction(actions.AbstractCause("Drop"), player, typedItem))
		g.actionBroker.Execute(actions.NewEndPlayerTurnAction(actions.AbstractCause("DroppedItem"), 5))
		if wasEquipped {
			g.ShowFullStats()
		}
		return
	}

	// equip
	if typedItem.IsEquippable() {
		if wasEquipped {
			g.actionBroker.Execute(actions.NewUnequipItemAction(actions.AbstractCause("Unequip"), player, typedItem))
			g.actionBroker.Execute(actions.NewEndPlayerTurnAction(actions.AbstractCause("EquipmentChange"), 10))
		} else {
			g.actionBroker.Execute(actions.NewEquipItemAction(actions.AbstractCause("Equip"), player, typedItem))
			g.actionBroker.Execute(actions.NewEndPlayerTurnAction(actions.AbstractCause("EquipmentChange"), 10))
		}
		g.ShowFullStats()
		return
	}

	// use
	contextActions := typedItem.GetContextActions(g.state)
	if len(contextActions) == 0 {
		return // maybe open a description? or at least a menu with drop or sth?
	}

	if len(contextActions) == 1 {
		actionFunc := contextActions[0].Action
		actionFunc()
		// hmm, this will trigger even if unsuc
		return
	}
	g.uiController.OpenMenu(contextActions)
}

func (g *GameInstance) onMapChanged(oldMapName string, newMapName string) {
	// setup ui
	g.setupUI()

	if isTownLevel(newMapName) {
		currentMap := g.state.GetMap()
		g.state.SetTownMap(currentMap)
		for _, actor := range currentMap.Actors() {
			if !actor.IsVendor() {
				continue
			}
			g.ensureVendorInventory(actor)
		}
		if newMapName == string(LevelNameTown) {
			g.setupTownTransitions(currentMap)
		} else if newMapName == string(LevelNameTownHell) {
			g.setupHellTownTransitions(currentMap)
		}

	} else { // dungeon map
		g.state.SetDungeonMap(newMapName, g.state.GetMap())
	}

	if isTownLevel(oldMapName) {
		g.GetAudio().PlayCue("heavy_door")
	}

	player := g.state.GetPlayer()

	if !g.setToExactFit && !g.configuration.KeepPlayerCentered && player != nil {
		g.mapRenderer.CenterOnFloat(player.DrawPosition())
	}


}

func (g *GameInstance) setupTownTransitions(currentMap *gridmap.GridMap[*common.Actor, common.Item, common.Object]) {
	entrancePos := currentMap.GetNamedLocation("to_cathedral")

	entranceTile := currentMap.GetCell(entrancePos).TileType
	currentMap.SetTile(entrancePos, entranceTile.WithSpecial(gridmap.SpecialTileStairsDown))

	currentMap.AddTransitionAt(entrancePos, gridmap.Transition{
		TargetMap:      levelName(1),
		TargetLocation: "ladder_up",
	})

	entrancePos = currentMap.GetNamedLocation("to_catacombs")

	entranceTile = currentMap.GetCell(entrancePos).TileType
	currentMap.SetTile(entrancePos, entranceTile.WithSpecial(gridmap.SpecialTileStairsDown))

	currentMap.AddTransitionAt(entrancePos, gridmap.Transition{
		TargetMap:      levelName(4),
		TargetLocation: "to_town",
	})

	entrancePos = currentMap.GetNamedLocation("to_caves")

	entranceTile = currentMap.GetCell(entrancePos).TileType
	currentMap.SetTile(entrancePos, entranceTile.WithSpecial(gridmap.SpecialTileStairsDown))

	currentMap.AddTransitionAt(entrancePos, gridmap.Transition{
		TargetMap:      levelName(6),
		TargetLocation: "to_town",
	})
}
func (g *GameInstance) setupHellTownTransitions(currentMap *gridmap.GridMap[*common.Actor, common.Item, common.Object]) {
	caveEntrace := currentMap.GetNamedLocation("to_caves")

	entranceTile := currentMap.GetCell(caveEntrace).TileType
	currentMap.SetTile(caveEntrace, entranceTile.WithSpecial(gridmap.SpecialTileStairsDown))

	currentMap.AddTransitionAt(caveEntrace, gridmap.Transition{
		TargetMap:      levelName(6),
		TargetLocation: "ladder_down",
	})

	dungeonEntrance := currentMap.GetNamedLocation("to_dungeon")
	entranceTile = currentMap.GetCell(dungeonEntrance).TileType
	currentMap.SetTile(dungeonEntrance, entranceTile.WithSpecial(gridmap.SpecialTileStairsDown))

	currentMap.AddTransitionAt(dungeonEntrance, gridmap.Transition{
		TargetMap:      levelName(9),
		TargetLocation: "ladder_down",
	})

}

func (g *GameInstance) ensureVendorInventory(actor *common.Actor) {
	randomSource := g.randomSource
	inventory := actor.GetInventory()
	if !inventory.IsEmpty() {
		return
	}
	var inventoryItems []common.Item
	switch actor.GetVendorType() {
	case common.VendorTypeWands:
		inventoryItems = []common.Item{
			g.NewHealingPotion(),
			g.NewManaPotion(),
			g.NewStrengthPotion(),
			g.NewBerserkPotion(),
			g.NewScrollBlessing(),
			g.NewScrollPhaseDoor(),
			g.NewScrollWordOfRecall(),
			g.NewFiniteTargetedWand(g.getFireballAction(ActionNameFireball)),
		}
	case common.VendorTypeWeapons:
		inventoryItems = []common.Item{
			NewSword(g.GetWorldState(), randomSource, 1),
			NewAxe(g.GetWorldState(), randomSource, 1),
			NewBow(g.GetWorldState(), randomSource, 1),
		}
	case common.VendorTypeArmor:
		inventoryItems = []common.Item{
			NewArmor(g.GetWorldState(), randomSource, 1),
			NewHelmet(g.GetWorldState(), randomSource, 1),
			NewBoots(g.GetWorldState(), randomSource, 1),
			NewCape(g.GetWorldState(), randomSource, 1),
			NewGloves(g.GetWorldState(), randomSource, 1),
		}
	case common.VendorTypeGeneral:
		inventoryItems = []common.Item{
			NewTorch(g.GetWorldState(), randomSource, 1),
			NewLantern(g.GetWorldState(), randomSource, 1),
			NewStarGlass(g.GetWorldState(), randomSource, 1),
			NewAmulet(g.GetWorldState(), randomSource, 1),
			NewRing(g.GetWorldState(), randomSource, 1),
		}
	}
	for _, item := range inventoryItems {
		inventory.Add(item)
	}
}

func (g *GameInstance) GetActiveSkillsForUI(player *common.Actor) []ui.UIElement {
	var activeSkills []ui.UIElement
	for _, skill := range player.GetTargetedSkills() {
		activeSkills = append(activeSkills, skill)
	}
	for _, skill := range player.GetUntargetedSkills() {
		activeSkills = append(activeSkills, skill)
	}
	return activeSkills
}

func (g *GameInstance) initPlayerSkillsForArcher(player *common.Actor) {
	var targeted []*common.TargetedAction
	targeted = append(targeted,
	//	g.NewTargetedActionFromName(ActionNameFireball),
	//	g.NewTargetedActionFromName(ActionNameFirewall),
		g.NewTargetedActionFromName(ActionNameBlink),
		g.NewTargetedActionFromName(ActionNameMindPush),
		g.NewTargetedActionFromName(ActionNameImmobilize),
		g.NewTargetedActionFromName(ActionNameFireRay),
		g.NewTargetedActionFromName(ActionNameFireBresen),
	)
	player.SetTargetedSkills(targeted)

	untargeted := []*common.UntargetedAction{
		g.getMultiShot(5),
	}
	player.SetUntargetedSkills(untargeted)
}
func (g *GameInstance) GetSustainedSkills(player *common.Actor) []*SustainedAction {

	var activeSkills []*SustainedAction
	/*
	   for _, skillName := range activeSkillNames {
	       skillLevel := player.GetStats().GetAttribute(skillName)
	       if skillLevel == 0 {
	           continue
	       }

	   }

	*/
	activeSkills = append(activeSkills,
		g.getInnerFlame(),
	)
	return activeSkills
}

type Turner interface {
	Turn()
}

func (g *GameInstance) getTurnerAction(baseSpeed int, energy int, being Turner) (int, func()) {
	cost := 100 / baseSpeed
	if energy < cost {
		return 0, nil
	}
	return baseSpeed, being.Turn
}

func (g *GameInstance) removeObject(object common.Object) {
	g.state.GetMap().RemoveObject(object)
}

func (g *GameInstance) openTransferEnchantmentMenu() {
	// step one, choose an item
	player := g.state.GetPlayer()
	playerInventory := player.GetInventory()
	if playerInventory.IsEmpty() {
		g.Log(fmt.Sprintf("You have no items"))
		return
	}
	itemsWithEnchantments := make([]common.Item, 0)
	for _, item := range playerInventory.Items() {
		gItem := item.(*GameItem)
		if len(gItem.GetAllEnchantments()) > 0 {
			itemsWithEnchantments = append(itemsWithEnchantments, item)
		}
	}
	if len(itemsWithEnchantments) == 0 {
		g.Log(fmt.Sprintf("You have no items with enchantments"))
		return
	}
	var menuItems []util.MenuItem
	for _, item := range itemsWithEnchantments {
		gItem := item.(*GameItem)
		menuItems = append(menuItems, util.MenuItem{
			Text: gItem.Name(),
			Action: func() {
				g.openChooseEnchantmentMenu(gItem)
			},
		})
	}

	g.uiController.OpenMenu(menuItems)
}

func (g *GameInstance) openChooseEnchantmentMenu(source *GameItem) {
	var menuItems []util.MenuItem
	for _, e := range source.GetAllEnchantments() {
		enchantment := e
		menuItems = append(menuItems, util.MenuItem{
			Text: enchantment.ToString(),
			Action: func() {
				g.openChooseTransferTargetMenu(source, enchantment)
			},
		})
	}

	g.uiController.OpenMenu(menuItems)
}

func (g *GameInstance) openChooseTransferTargetMenu(source *GameItem, enchantment Enchantment) {
	player := g.state.GetPlayer()
	playerInventory := player.GetInventory()
	if playerInventory.IsEmpty() {
		return
	}
	var menuItems []util.MenuItem
	for _, item := range playerInventory.Items() {
		gItem := item.(*GameItem)
		if gItem == source {
			continue
		}

		menuItems = append(menuItems, util.MenuItem{
			Text: gItem.Name(),
			Action: func() {
				g.transferEnchantment(source, gItem, enchantment)
			},
		})
	}

	g.uiController.OpenMenu(menuItems)
}

func (g *GameInstance) transferEnchantment(source *GameItem, item *GameItem, enchantment Enchantment) {
	source.RemoveEnchantment(enchantment)
	g.Log(fmt.Sprintf("%s lost %s", source.Name(), enchantment.ToString()))
	item.AddEnchantment(enchantment)
	g.Log(fmt.Sprintf("%s gained %s", item.Name(), enchantment.ToString()))
	g.uiController.PopAll()
}

func (g *GameInstance) playerTravelToRecallLocation() {
	currentMapName := g.state.GetMap().GetName()
	isInTown := isTownLevel(currentMapName)

	var dest gridmap.Transition

	if isInTown {
		dest = gridmap.Transition{
			TargetMap:      levelName(g.state.GetLastDungeonLevel()),
			TargetLocation: "ladder_up",
		}
	} else { // in dungeon
		currentLevel := levelFromName(currentMapName)
		if g.state.GetLastDungeonLevel() < currentLevel {
			g.state.SetLastDungeonLevel(currentLevel)
		}
		dest = gridmap.Transition{TargetMap: string(LevelNameTown), TargetLocation: "spawn"}
	}

	g.instantlyTravelTo(dest)
}

func (g *GameInstance) instantlyTravelTo(dest gridmap.Transition) {
	g.actionBroker.Execute(actions.NewTransitionAction(actions.AbstractCause("Quick Travel"), dest))
	g.actionBroker.Execute(actions.NewUpdateFoVAction(actions.AbstractCause("Quick Travel"), g.state.GetPlayer()))
	g.actionBroker.Execute(actions.NewEndPlayerTurnAction(actions.AbstractCause("Quick Travel"), 20))
}

func (g *GameInstance) afterTurn() {

}

func (g *GameInstance) nextStepOnPlayerPath() {
	if !g.state.GetPlayer().IsAlive() {
		g.stopAutoMove()
		return
	}
	if len(g.playerPath) > 0 {
		if g.state.GetVisibility().GetVisibleEnemyCount(func(actor *common.Actor) bool {
			return actor.IsHostile() && actor.IsHostileTo(g.state.GetPlayer())
		}) > 0 {
			g.stopAutoMove()
			g.Log("You can see an enemy")
			return
		}

		// pop first element
		nextStep := g.playerPath[0]

		if g.state.GetMap().IsActorAt(nextStep) {
			//actorAt := g.state.GetMap().ActorAt(nextStep)
			g.stopAutoMove()
			direction := nextStep.Sub(g.state.GetPlayer().MapPosition())
			g.manualMovePlayer(direction)
			return
		}

		if g.state.GetMap().IsObjectAt(nextStep) {
			if g.IsClosedDoorAt(nextStep) {
				openDoorAction := actions.NewOpenDoorAction(actions.AbstractCause("Player Movement"), g.state.GetPlayer(), nextStep)
				g.actionBroker.QueueForExecution(openDoorAction)
				g.actionBroker.QueueForExecution(actions.NewEndPlayerTurnAction(openDoorAction.Cause(), openDoorAction.GetDelayInAut()))
				return
			} else if !g.autoExplore {
				objectAt := g.state.GetMap().ObjectAt(nextStep)
				if !objectAt.IsWalkable(g.state.GetPlayer()) {
					g.stopAutoMove()
					g.Log(fmt.Sprintf("You bump into %s", objectAt.Name()))
					objectAt.ActorBumpedInto(g.state.GetPlayer(), actions.AnimateNow())
					return
				}
			}
		}

		if !g.state.GetMap().IsCurrentlyPassable(nextStep) {
			g.stopAutoMove()
			g.Log("You can't move there")
			return
		}

		g.playerPath = g.playerPath[1:]
		autoMoveAction := actions.NewMoveAction(actions.AbstractCause("Player Movement"), g.state.GetPlayer(), nextStep, true)
		autoMoveAction.SetAnimationDelay(0.08)
		g.actionBroker.QueueForExecution(autoMoveAction)
		g.actionBroker.QueueForExecution(actions.NewEndPlayerTurnAction(autoMoveAction.Cause(), autoMoveAction.GetDelayInAut()))
	} else if g.autoExplore {
		if target, hasTarget := g.getNextAutoExploreTarget(); hasTarget {
			g.continuePlayerAutoMovement(target)
		} else {
			g.Log("Nothing left to explore")
			g.stopAutoMove()
		}
	}
}

func (g *GameInstance) getNextAutoExploreTarget() (geometry.Point, bool) {
	player := g.state.GetPlayer()
	playerInv := player.GetInventory()
	playerPos := player.MapPosition()

	if !playerInv.IsFull() && g.configuration.AutoPickup {
		nearestItem := g.state.GetMap().GetNearestExploredItem(playerPos, func(item common.Item) bool {
			return !item.IsDroppedByPlayer() && (!item.IsHidden() || g.state.GetFlags().IsSet(string(FlagHasTrueSight)))
		})
		if nearestItem != playerPos {
			return nearestItem, true
		}
	}

	unexplored := g.state.GetMap().GetNearestUnexploredWalkablePosition(playerPos)
	if unexplored != playerPos {
		return unexplored, true
	}
	return geometry.Point{}, false
}

func (g *GameInstance) cancelAutoMove() {
	g.stopAutoMove()
	g.actionBroker.CancelAllAnimations()

}
func (g *GameInstance) stopAutoMove() {
	g.autoExplore = false
	g.playerPath = nil
	g.actionBroker.ClearQueuedActions(func(action actions.Action) bool {
		if moveAction, ok := action.(*actions.MoveAction); ok {
			return moveAction.GetActor() == g.state.GetPlayer()
		}
		return false
	})
}

func (g *GameInstance) getDisplacePositionForActor(victimToDisplace *common.Actor, direction geometry.Point) geometry.Point {
	currentMap := g.state.GetMap()
	victimPos := victimToDisplace.MapPosition()
	var displacePos geometry.Point
	stepsInDegrees := 45
	for turnBy := stepsInDegrees; turnBy < 180; turnBy += stepsInDegrees {
		curDir := geometry.RotateVector(direction, float64(turnBy))
		displacePos = victimPos.Add(curDir)
		if currentMap.IsCurrentlyPassable(displacePos) {
			return displacePos
		}

		curDir = geometry.RotateVector(direction, float64(-turnBy))
		displacePos = victimPos.Add(curDir)
		if currentMap.IsCurrentlyPassable(displacePos) {
			return displacePos
		}
	}
	displacePos = victimPos.Add(direction)
	if currentMap.IsCurrentlyPassable(displacePos) {
		return displacePos
	}
	// return source
	behindUs := victimPos.Add(direction.Mul(-1))
	return behindUs
}

func (g *GameInstance) afterTimedRemoved(thing TimedBeing) {
	//TODO?
}

func (g *GameInstance) getCurrentTimedEffects() []ui.UIElement {
	var result []ui.UIElement
	for _, timedBeing := range g.timekeeper.GetBeingsByFilter(func(being TimedBeing) bool {
		switch being.(type) {
		case *common.Actor:
			return false
		case *SustainedAction:
			return true
		case *common.UntargetedAction:
			return true
		case *TimedModifier:
			return true
		default:
			return false
		}
	}) {
		result = append(result, timedBeing.(ui.UIElement))
	}
	return result
}

func (g *GameInstance) openLevelUpMenu() {
	player := g.state.GetPlayer()
	if !player.CanLevelUp() {
		return
	}
	g.levelUpButton.Hide()
	g.ShowFullStats()
	g.uiController.OpenMenu([]util.MenuItem{
		{
			Text: "STR",
			Action: func() {
				player.LevelUp(stats.Strength)
				if player.CanLevelUp() {
					g.GetAudio().PlayCue("levelup")
					g.openLevelUpMenu()
				} else {
					g.levelUpButton.Hide()
					g.ShowFullStats()
				}
			},
		},
		{
			Text: "DEX",
			Action: func() {
				player.LevelUp(stats.Dexterity)
				if player.CanLevelUp() {
					g.GetAudio().PlayCue("levelup")
					g.openLevelUpMenu()
				} else {
					g.levelUpButton.Hide()
					g.ShowFullStats()
				}
			},
		},
		{
			Text: "INT",
			Action: func() {
				player.LevelUp(stats.Intelligence)
				if player.CanLevelUp() {
					g.GetAudio().PlayCue("levelup")
					g.openLevelUpMenu()
				} else {
					g.levelUpButton.Hide()
					g.ShowFullStats()
				}
			},
		},
	})
}

func (g *GameInstance) checkForLevelUp() {
	if !g.state.GetPlayer().CanLevelUp() {
		return
	}
	if g.levelUpButton.IsVisible() {
		return
	}
	g.levelUpButton.Show()
	g.GetAudio().PlayCue("levelup")
}

func (g *GameInstance) IsClosedDoorAt(pos geometry.Point) bool {
	currentMap := g.GetWorldState().GetMap()
	if !currentMap.IsObjectAt(pos) {
		return false
	}

	objectAt := currentMap.ObjectAt(pos)
	door, isDoorHere := objectAt.(*common.Door)
	if !isDoorHere {
		return false
	}
	return door.IsClosed()
}

func (g *GameInstance) SetLDTKProject(project *ldtk_go.Project) {
	g.ldtkProject = project
}

func (g *GameInstance) getMiniMapColor(mapPos geometry.Point, tileColor *[4]byte) {
	currentMap := g.state.GetMap()
	if g.state.GetPlayer().MapPosition() == mapPos {
		*tileColor = [4]byte{100, 255, 100, 255}
		return
	}
	if !currentMap.IsExplored(mapPos) {
		*tileColor = [4]byte{10, 10, 10, 255}
		return
	}
	if currentMap.IsStairsAt(mapPos) {
		*tileColor = [4]byte{235, 52, 229, 255}
		return
	}
	if !g.state.GetPlayer().CanSee(mapPos) {
		if currentMap.IsWalkable(mapPos) {
			*tileColor = [4]byte{35, 30, 51, 255}
		} else {
			*tileColor = [4]byte{85, 95, 112, 255}
		}
		return
	}
	if currentMap.IsActorAt(mapPos) {
		actorAt := currentMap.ActorAt(mapPos)
		if actorAt.IsHostileTo(g.state.GetPlayer()) {
			*tileColor = [4]byte{255, 0, 0, 255}
			return
		}
		*tileColor = [4]byte{220, 120, 75, 255}
		return
	}
	if currentMap.IsWalkable(mapPos) {
		*tileColor = [4]byte{77, 60, 102, 255}
	} else {
		*tileColor = [4]byte{170, 191, 224, 255}
	}
}

func (g *GameInstance) decorateRoomFromTemplate(emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], template *ldtk_go.Level, room *dungen.DungeonRoom) {
	tileLayer := template.LayerByIdentifier("Tiles")
	tileSet := g.ldtkProject.TilesetByIdentifier("World")
	for _, tile := range tileLayer.Tiles {
		if tile.ID < 0 || tile.ID > 223 {
			continue
		}
		relativeMapPos, mapTile := MapTileFromLDTK(tileSet, tileLayer, tile)
		position := room.ToAbsolutePosition(relativeMapPos)
		if emptyMap.IsTileWalkable(position) && room.Contains(position) {
			emptyMap.SetTile(position, mapTile)
		}
	}

	entityLayer := template.LayerByIdentifier("Entities")
	for _, entity := range entityLayer.Entities {
		posX, posY := entityLayer.ToGridPosition(entity.Position[0], entity.Position[1])
		relativeMapPos := geometry.Point{X: posX, Y: posY}
		mapPos := room.ToAbsolutePosition(relativeMapPos)

		if entity.Identifier != "NPC" { // only unique NPCs for now
			continue
		}
		if emptyMap.IsCurrentlyPassable(mapPos) {
			if !entity.PropertyByIdentifier("UniqueName").IsNull() {
				uniqueName := entity.PropertyByIdentifier("UniqueName").AsString()
				uniqueActor := NewUniqueEnemyFromName(uniqueName)
				emptyMap.AddActor(uniqueActor, mapPos)
			}
		}

	}
}

func (g *GameInstance) getCurrentDungeonLevel() int {
	currentMapName := g.state.GetMap().GetName()
	if isTownLevel(currentMapName) {
		return 0
	}
	return levelFromName(currentMapName)
}

func (g *GameInstance) gotoHellTown() {
	dest := gridmap.Transition{TargetMap: "TownHell", TargetLocation: "spawn"}
	g.instantlyTravelTo(dest)
}

func (g *GameInstance) getWaterColor() color.RGBA {
	//all_clear:SpecialLevelPoisonedWaterSupply
	//
	poisonColor := color.RGBA{R: 199, G: 251, B: 161, A: 255}
	cleanColor := color.RGBA{R: 86, G: 112, B: 194, A: 255}

	if g.state.GetFlags().IsSet(fmt.Sprintf("all_clear:%s", SpecialLevelPoisonedWaterSupply)) {
		return cleanColor
	} else {
		return poisonColor
	}
}

func (g *GameInstance) showHelpText() {
	textColor := common.DarkTextColor
	var pickupText string
	if g.configuration.AutoPickup {
		pickupText = "Press 'g' to pick up items you dropped"
	} else {
		pickupText = "Press 'g' to pick up items"
	}
	g.uiController.OpenTextWindow([]util.LabelText{
		{
			Text:      "Welcome to heaven and hell!",
			TextColor: textColor,
		},
		{Text: "", TextColor: textColor},
		{
			Text:      "Use the 'w/a/s/d' keys or Numpad to move around",
			TextColor: textColor,
		},
		{
			Text:      "Bump into stuff and people to interact with them or attack them",
			TextColor: textColor,
		},
		{Text: "", TextColor: textColor},
		{
			Text:      "Press 'q' to quickly fire at the nearest enemy with your ranged weapon",
			TextColor: textColor,
		},
		{
			Text:      "Press 'f' to aim at a specific enemy with your ranged weapon",
			TextColor: textColor,
		},
		{
			Text:      "Press 'o' to auto-explore",
			TextColor: textColor,
		},
		{
			Text:      "Press 'tab' to auto-attack the nearest enemy using your equipped weapon",
			TextColor: textColor,
		},
		{
			Text:      "Press 'i' to open your inventory",
			TextColor: textColor,
		},
		{
			Text:      "Press 'c' to show your stats",
			TextColor: textColor,
		},
		{
			Text:      pickupText,
			TextColor: textColor,
		},
		{
			Text:      "Press 'l' to show the last lines of the message log",
			TextColor: textColor,
		},
		{
			Text:      "Press 'v' to show the quest log",
			TextColor: textColor,
		},
	})
}

func (g *GameInstance) onLuciferDeath() {
	lines := []util.LabelText{
		common.LabelTextFromStringWithColor("Lucifer", color.White),
		common.LabelTextFromString(""),
	}

	luciferLines := append(lines, NewDialogueTextFromFile("lucifer_death.txt")...)
	g.uiController.OpenTextWindow(luciferLines)

	g.spawnItemInTown("true_sight_potion", g.NewPotionOfTrueSight())
}




func OneTargetTile(origin, target geometry.Point) []geometry.Point {
	return []geometry.Point{target}
}
func toList(blocking map[geometry.Point]int) []geometry.Point {
	list := make([]geometry.Point, 0)
	for pos := range blocking {
		list = append(list, pos)
	}
	return list
}
func MapTileFromLDTK(tileSet *ldtk_go.Tileset, layer *ldtk_go.Layer, tile *ldtk_go.Tile) (geometry.Point, gridmap.Tile) {
	posX, posY := layer.ToGridPosition(tile.Position[0], tile.Position[1])
	pos := geometry.Point{X: posX, Y: posY}
	enums := tileSet.EnumsForTile(tile.ID)

	walkable := !enums.Contains("IsBlockingMovement")
	transparent := !enums.Contains("IsBlockingView")
	isDamaging := enums.Contains("IsDamaging")
	isWater := enums.Contains("IsWater")
	specialTile := gridmap.SpecialTileNone

	if isWater {
		specialTile = gridmap.SpecialTileWater
	} else if isDamaging {
		specialTile = gridmap.SpecialTileDamaging
	}

	tileType := gridmap.Tile{
		DefinedIcon:   int32(tile.ID),
		IsWalkable:    walkable,
		IsTransparent: transparent,
		Special:       specialTile,
	}
	return pos, tileType
}
