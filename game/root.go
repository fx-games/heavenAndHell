package game

import (
	"fmt"
	"github.com/flopp/go-findfont"
	"math"
	"math/rand"
	"os"
	"path"
	"rpg/audio"
	"rpg/common"
	"rpg/d1"
	"rpg/defs"
	"rpg/files"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/ldtk"
	"rpg/stats"
	"rpg/util"
	"strings"
	"time"
)

type GameInstance struct {
	// base infos / config
	title                       string
	deviceIndependentScreenSize geometry.Point
	configuration               *common.Configuration
	startInfo                   common.StartInfo

	// dependencies
	uiController common.UserInterface
	audioPlayer  *audio.Player

	// world state
	state        common.State
	actionBroker *common.Broker // registered hooks

	// random
	randomSource *rand.Rand
	worldTicks   uint64

	// rendering

	// input
	mousePosInPixels geometry.Point
	mousePosOnMap    geometry.Point

	// textserver
	serverStats     *TextServer
	serverInventory *TextServer
	serverEquipment *TextServer
	serverLog       *TextServer

	// player state
	isResting bool

	// game state
	shouldQuitGame   bool
	quitToAscii      bool
	debugModeEnabled bool
	godMode          bool
	heroLevel        int // last seen character level, for the level-up sound

	// event handler
	executeAfterTurn []func()

	// data definitions
	dataDefinitions  defs.DataContainer
	d1Data           *d1.Data // with Hellfire
	d1Classic        *d1.Data // plain D1 1.09
	hellfire         bool     // which of the two this game runs on
	levelSeed        int64
	levelSnapshots   map[string]*common.LevelSnapshot
	autosave         bool // on once a game was started or loaded through the menus
	slot             int  // the save slot this game autosaves into; 0 for the throwaway game behind the start menu
	refreshThumbnail bool // the slot's screenshot predates this game; the first player turn saves once more
	permadeath       bool
	premium          d1.Premium // Griswold's premium stock
	boyItem          *d1.Item   // Wirt's item
	boyLevel         int
	storesShown      bool // stores were stocked this session, so missing items were bought

	actions *Actions
	factory *common.Factory

	logHook     func(string)
	logBuffer   []string
	onMapChange func()
}

func (g *GameInstance) SetConsoleMode(consoleMode bool) {
	// only disable audio for now
	g.audioPlayer.SetDisablePlayback(consoleMode)
}

func (g *GameInstance) GetActionBroker() *common.Broker {
	return g.actionBroker
}
func (g *GameInstance) GetMoveTowardsPlayer(from geometry.Point) geometry.Point {
	return g.getMoveOnPlayerDijkstraMap(from, true)
}

func (g *GameInstance) GetMoveAwayFromPlayer(from geometry.Point) geometry.Point {
	return g.getMoveOnPlayerDijkstraMap(from, false)
}
func (g *GameInstance) getMoveOnPlayerDijkstraMap(from geometry.Point, towardsPlayer bool) geometry.Point {
	dijkstraMap := g.state.GetPlayerDijkstraMap()
	if dijkstraMap == nil {
		return from
	}
	currentDistanceToPlayer, exist := dijkstraMap[from]
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
		neighborDist, neighborExists := dijkstraMap[pos]
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
		neighborDist, _ := dijkstraMap[neighbor]
		//if neighborDist < nearestDist {  // depends on rolling direction on our dijkstra map
		if compareFunc(neighborDist, nearestDist) {
			nearestDist = neighborDist
			nearestPos = neighbor
		}
	}

	return nearestPos
}

func (g *GameInstance) GetAudio() *audio.Player {
	return g.audioPlayer
}

func NewGameInstance(startInfo common.StartInfo, uiController common.UserInterface) *GameInstance {
	g := &GameInstance{
		startInfo:    startInfo,
		uiController: uiController,
		randomSource: rand.New(rand.NewSource(time.Now().UnixNano())),
		audioPlayer:  audio.NewAudioPlayer(startInfo.FileSystem, audio.SampleRate44100),
	}
	println(fmt.Sprintf("GameInstance created.."))
	println(fmt.Sprintf("Random source loaded.."))
	cfg := startInfo.UserConfiguration
	g.audioPlayer.SetSoundVolume(cfg.SoundVolume)
	g.audioPlayer.SetMusicVolume(cfg.MusicVolume)
	g.audioPlayer.SetVoiceVolume(cfg.VoiceVolume)
	println(fmt.Sprintf("Audio player loaded.."))
	g.dataDefinitions = defs.LoadGameData(func(err string) {
		println(err)
	})
	println(fmt.Sprintf("Data definitions loaded.."))
	d1Data, err := d1.Load(files.GetFileSystem())
	if err != nil {
		panic(err)
	}
	g.d1Data = d1Data
	if g.d1Classic, err = d1.LoadD1(files.GetFileSystem()); err != nil {
		panic(err)
	}
	g.hellfire = false // the game behind the start menu, which is the first game when there is no save yet: plain D1
	g.ResetGame(g.dataDefinitions.ClassDefinitions[0])
	return g
}

func (g *GameInstance) hasEnergyForAction(being common.TimedBeing) (int, func()) {
	switch typedBeing := being.(type) {
	case *common.Actor:
		return g.getAIAction(typedBeing)
	case Turner:
		return g.getTurnerAction(being.GetBaseSpeed(), being.GetEnergy(), typedBeing)
	}
	return 0, nil
}
func (g *GameInstance) giveAndEquipItemToActor(actor *common.Actor, item *common.Item) {
	actor.GetInventory().Add(item)
	g.actionBroker.Execute(common.NewEquipItemAction(common.AbstractCause("Game Start"), actor, item))
}

// getActionContext requires the following fields to be initialized:
// g.audioPlayer, g.dataDefinitions, g.state
func (g *GameInstance) getActionContext() common.ActionContext {
	return common.ActionContext{
		GameState:     g.GetWorldState(),
		UserInterface: g.GetUserInterface,
		AudioPlayer:   g.audioPlayer,
		Data:          g.dataDefinitions,
		Msg:           g.PlayerMessage,
	}
}

func (g *GameInstance) ResetGame(playerStartDefinition defs.LoadoutDefinition) {
	g.resetGame(playerStartDefinition, g.randomSource.Int63())
}

func (g *GameInstance) resetGame(playerStartDefinition defs.LoadoutDefinition, seed int64) {
	g.levelSeed = seed
	g.heroLevel = 0
	g.levelSnapshots = map[string]*common.LevelSnapshot{}
	g.premium, g.boyItem, g.boyLevel, g.storesShown = d1.Premium{}, nil, 0, false
	// we have loaded at this point:
	// - random source
	// - audio player
	// - data definitions
	g.actions = nil
	g.actionBroker = nil
	g.factory = nil
	//g.uiController =

	g.configuration = g.startInfo.UserConfiguration
	g.deviceIndependentScreenSize = g.startInfo.ScreenSize

	g.startWindowServers()

	ldtkFileName := g.startInfo.LDTKMapFile
	ldtkFile, _ := files.ReadFile(ldtkFileName)
	ldtkProject, _ := ldtk.Open(ldtkFile)

	timeKeeper := common.NewTimeKeeper(g.hasEnergyForAction)
	println(fmt.Sprintf("Timekeeper created.."))

	g.state = common.NewState(ldtkProject)
	g.state.SetPlayerMessage(g.PlayerMessage)
	g.state.SetTimeKeeper(timeKeeper)
	g.state.SelectedClass = playerStartDefinition

	g.actionBroker = common.NewBroker(g.getActionContext())
	g.actionBroker.RegisterHook(common.NewOnHealthChangedHook())

	g.actionBroker.RegisterHook(common.NewMainOnMoveHook(g.configuration.AutoPickup)) // update fov, etc.
	g.actionBroker.RegisterHook(common.NewMainOnMeleeHook())
	g.actionBroker.RegisterHook(common.NewMainOnRangedHook())
	g.actionBroker.RegisterHook(common.NewMainOnKillHook())

	g.actionBroker.RegisterHook(common.NewMainOnDropHook())
	g.actionBroker.RegisterHook(common.NewMainOnEquipHook())

	g.factory = &common.Factory{
		ActionBroker:   g.actionBroker,
		RandomSource:   g.randomSource,
		State:          g.GetWorldState(),
		UI:             g.GetUserInterface,
		GetTimeKeeper:  g.state.GetTimeKeeper,
		Msg:            g.PlayerMessage,
		QueueAfterTurn: g.queueAfterTurn,
		Generator:      d1.NewGenerator(g.gameData(), g.randomSource.Int31()),
		Audio:          g.audioPlayer,
	}

	g.state.SetPlayerMessage(g.PlayerMessage)
	g.state.SetPlayerTurnHandler(g.onPlayerTurnTaken)
	g.state.SetAnimator(g.actionBroker)
	g.state.SetActionBroker(g.actionBroker)
	g.state.SetFactory(g.factory)

	g.createPlayer(playerStartDefinition)
	g.actions = NewActions(PlayerActionsContext{
		Player:         g.state.GetPlayer(),
		ActionBroker:   g.actionBroker,
		State:          g.GetWorldState(),
		UI:             g.GetUserInterface,
		GetTimeKeeper:  g.state.GetTimeKeeper,
		Msg:            g.PlayerMessage,
		Animator:       g.actionBroker,
		Configuration:  g.configuration,
		QueueAfterTurn: g.queueAfterTurn,
	})
	g.state.SetOnMapChanged(g.onMapChanged)
	g.state.MapTransition(gridmap.Transition{
		TargetMap:      string(defs.LevelNameTown),
		TargetLocation: "spawn",
	})

	mapGen := common.NewMapGenerator(g.randomSource)
	mapGen.SetFactory(g.factory)
	mapGen.SetSeed(g.levelSeed)
	mapGen.SetSnapshots(g.levelSnapshots)

	// set references
	g.state.SetMapGenerator(mapGen)
	g.state.ApplyHellfireOptions()                    // D1 only: no Hellfire quests or towners
	common.UpdateD1Quests(&g.state, g.state.GetMap()) // the town was built before there were quests

	townSpawn := g.state.GetMap().GetNamedLocation("spawn")
	g.state.SetTownSpawn(townSpawn)

	//g.spawnActorOnMap(g.state.GetPlayer(), townSpawn, playerStartDefinition)

	// load audio: sound events (assets/data/sounds.rec, ./remap_sounds.sh) pick their files from sfx/ang_sfx and the
	// optional diablo_sfx (only present if the player dropped their own converted Diablo assets in, see AGENTS.md).
	if g.configuration.AudioEnabled {
		diablo := g.configuration.DiabloAudioEnabled
		g.audioPlayer.RegisterSoundEvents(path.Join("assets", "data", "sounds.rec"), func(f string) bool {
			return diablo || !strings.HasPrefix(f, "diablo_")
		})
		preload := []string{"enter_dungeon", "level_up", "missile_hit_human", "heavy_hit_human", "skeleton_hit"}
		g.audioPlayer.PreLoadCuesIntoMemory(preload)
		g.audioPlayer.RegisterMusic(path.Join("assets", "ambience"))
		if g.configuration.DiabloAudioEnabled {
			g.audioPlayer.RegisterMusic(path.Join("assets", "diablo_music")) // dtowne/dlvla-d renamed to town/cathedral/catacombs/caves/hell
		}
		g.audioPlayer.RegisterVoices(path.Join("assets", "voice"))
		if diablo {
			for id, file := range d1.SfxFiles { // D1's speech and quest sounds, played by their _sfx_id
				g.audioPlayer.RegisterVoice(id, path.Join("assets", "diablo_sfx", file))
			}
		}
	}

	g.state.GetFlags().SetTrue("game_started")

	g.actionBroker.Execute(common.NewUpdateFoVAction(common.AbstractCause("Game Start"), g.state.GetPlayer()))

	g.uiController.UpdateStatsLabel()

	g.state.GetFlags().SetTrue(string(common.FlagStoryModeEnabled))
}

// SetAndInitUserInterface will be called after the game has finished initialization
// So we have to inform every component about the ui
func (g *GameInstance) SetAndInitUserInterface(ui common.UserInterface) {
	g.uiController = ui
	ui.SetPlayerMessage(g.PlayerMessage)
	ui.OnMapChanged()
	ui.SetDebugActions(g.getDebugActions)
}
func (g *GameInstance) queueAfterTurn(call func()) {
	g.executeAfterTurn = append(g.executeAfterTurn, call)
}

func (g *GameInstance) GetUserInterface() common.UserInterface {
	return g.uiController
}

func (g *GameInstance) handleLightSource() {
	player := g.state.GetPlayer()
	if player.GetEquipment().HasLightSourceEquipped() && !g.state.PlayerStandsInLitArea() {
		lightSource := player.GetEquipment().GetLightSource()
		asGameItem := lightSource
		asGameItem.UseCharge()
	}
}
func (g *GameInstance) onPlayerTurnTaken(autUsed int) {
	player := g.state.GetPlayer()

	g.state.GetTimeKeeper().PlayerTakesTurn(autUsed)
	// D1 hit recovery: the world goes on while the hero flinches; a hard hit meanwhile restarts it.
	// ponytail: at most 50 recoveries in a row, so a stun-lock can't freeze the game.
	for n := 0; n < 50 && player.IsAlive(); n++ {
		lost := player.TakeHitRecovery()
		if lost == 0 {
			break
		}
		g.state.GetTimeKeeper().PlayerTakesTurn(lost)
	}
	// ISPL_DRAINLIFE (ProcessPlayers): a cursed item drains 4/64 HP per tick outside town
	if player.IsAlive() && player.Hero() != nil && player.HeroSheet().Flags&d1.ISplDrainLife != 0 && !defs.IsTownLevel(g.state.GetMap().GetName()) {
		g.actionBroker.Execute(common.NewChangeResourceAction(common.AbstractCause("Drain life"), player, stats.Health, -float64(autUsed*d1.RefWalkTicks*4)/10/64))
	}

	common.D1BurningCross(&g.state, player, autUsed)

	// execute after turn events, =?
	for _, event := range g.executeAfterTurn {
		event()
	}
	g.executeAfterTurn = make([]func(), 0)

	g.checkForLevelUp()

	player.BeforeTurn()
	g.handleLightSource()
	if g.godMode {
		player.GetStats().GetResource(stats.Health).SetToMax()
		player.GetStats().GetResource(stats.Mana).SetToMax()
	}

	// automatic pickup & player movement..
	if g.configuration.AutoPickup && g.state.GetMap().IsItemAt(player.MapPosition()) {
		itemAtLocation := g.state.GetMap().ItemAt(player.MapPosition())
		if itemAtLocation != nil && player.CanPickup(itemAtLocation) && !itemAtLocation.IsDroppedByPlayer() {
			pickUpAction := common.NewPickUpAction(common.AbstractCause("AutoPickup"), player)
			endTurnAction := common.NewEndPlayerTurnAction(pickUpAction.Cause(), pickUpAction.GetDelayInAut())
			g.actionBroker.Execute(pickUpAction)
			g.actionBroker.Execute(endTurnAction)
		}
	} else {
		g.actions.nextStepOnPlayerPath()
	}

	g.uiController.OnPlayerTurnTaken()

	if g.refreshThumbnail { // once, so the new hero's slot shows their own game and not the menu behind it
		g.refreshThumbnail = false
		g.SaveGame()
	}
}

func (g *GameInstance) updateStatsDisplay() {
	g.uiController.UpdateStatsLabel()
	g.sendStatsToWindowClient()
}

func (g *GameInstance) createPlayer(playerStartDefinition defs.LoadoutDefinition) *common.Actor {
	player := common.NewActor(playerStartDefinition.Icon)
	player.SetAsciiIcon('@')
	player.SetCP437Icon(int32(util.UnicodeToCP437Byte('☺')))
	player.SetColor(defs.White)
	player.SetName("Player")
	g.state.SetPlayer(player)

	hero := d1.NewHero(d1ClassIndex(playerStartDefinition.InternalName))
	player.SetHero(hero)
	gen := g.factory.D1()
	// CreatePlrItems as Hellfire does it, on its item table (24 is the healing potion, 3 the warrior's spare club):
	// the Sorcerer's Short Staff of Mana (5) comes with healing potions, the Monk takes IDI_SHORTSTAFF (36), the
	// Bard IDI_BARDSWORD and IDI_BARDDAGGER (37, 38), and the Barbarian item 139 with the warrior's shield.
	for _, idx := range map[int][]int{
		d1.Warrior: {1, 2, 3, 24, 24}, d1.Rogue: {4, 24, 24}, d1.Sorcerer: {5, 24, 24},
		d1.Monk: {36, 24, 24}, d1.Bard: {37, 38, 24, 24}, d1.Barbarian: {139, 2, 24, 24},
	}[hero.Class] {
		item := g.factory.NewD1Item(gen.PlainItem(idx))
		if item.IsEquippable() && idx != 3 { // CreatePlrItems: the warrior's club goes into the pack
			g.giveAndEquipItemToActor(player, item)
		} else {
			player.GetInventory().Add(item)
		}
	}
	player.GetStats().SetAttribute(stats.Gold, 100)
	common.SyncHero(player)
	player.GetStats().GetResource(stats.Health).SetToMax()
	player.GetStats().GetResource(stats.Mana).SetToMax()
	player.GetStats().SetOnChangeHandler(g.updateStatsDisplay)
	g.factory.RefreshD1Spells(player)
	player.GetEquipment().SetOnChangeHandler(func() {
		common.SyncHero(player)
		g.factory.RefreshD1Spells(player)
		g.sendEquipmentToWindowClient()
		g.uiController.RefreshIconBars() // equipping takes no turn and leaves the pack as it is
	})
	player.GetInventory().SetOnChangeHandler(func() {
		g.sendInventoryToWindowClient()
		g.uiController.RefreshIconBars() // buying, selling and the stash change the pack without taking a turn
	})
	return player
}

func (g *GameInstance) gameData() *d1.Data {
	if g.hellfire {
		return g.d1Data
	}
	return g.d1Classic
}

func (g *GameInstance) GetFactory() *common.Factory { return g.factory }

func (g *GameInstance) GetWorldState() *common.State {
	return &g.state
}

// toggleHellfireOption flips one of Hellfire's command.txt switches and rebuilds the quests from the seed.
func (g *GameInstance) toggleHellfireOption(flag string) {
	f := g.state.GetFlags()
	f.Set(flag, !f.IsSet(flag))
	g.state.ApplyHellfireOptions()
	g.PlayerMessage(fmt.Sprintf("%s: %v", flag, f.IsSet(flag)))
}

// d1DebugItems: go to a level, god mode, complete a quest (decision 16).
func (g *GameInstance) d1DebugItems() []util.MenuItem {
	var levels, quests []util.MenuItem
	for dlvl := 1; dlvl <= 24; dlvl++ {
		name, at := defs.LevelNameFromDepth(dlvl), "ladder_up"
		if dlvl == 17 || dlvl == 21 { // entered from town only
			at = "warp"
		}
		levels = append(levels, util.MenuItem{MainText: fmt.Sprintf("Level %d", dlvl), Action: func() {
			g.state.MapTransition(gridmap.Transition{TargetMap: name, TargetLocation: at})
		}})
	}
	for _, set := range common.D1SetLevels {
		levels = append(levels, util.MenuItem{MainText: set.Name, Action: func() {
			g.state.MapTransition(gridmap.Transition{TargetMap: set.Map, TargetLocation: "ladder_up"})
		}})
	}
	for q, name := range d1.QuestNames {
		quests = append(quests, util.MenuItem{MainText: name, Action: func() {
			g.state.GetFlags().SetTrue(common.QuestFlag(q))
			common.UpdateD1Quests(&g.state, g.state.GetMap())
		}})
	}
	quests = append(quests, util.MenuItem{MainText: "Cain has the Staff of Lazarus", Action: func() {
		g.state.SetQuestActive(d1.QBetrayer, d1.QuestActive)
		g.state.SetQuestVar(d1.QBetrayer, 1, 2)
		common.UpdateD1Quests(&g.state, g.state.GetMap())
	}})
	return []util.MenuItem{
		{MainText: "Go to level", Action: func() { g.uiController.OpenMenu(levels) }},
		{MainText: "Toggle god mode", Action: func() { g.godMode = !g.godMode }},
		{MainText: "Toggle Hellfire Cowquest", Action: func() { g.toggleHellfireOption(common.FlagCowQuest) }},
		{MainText: "Toggle Hellfire Theoquest", Action: func() { g.toggleHellfireOption(common.FlagTheoQuest) }},
		{MainText: "Complete quest", Action: func() { g.uiController.OpenMenu(quests) }},
		{MainText: "Spawn random unique", Action: func() {
			player := g.state.GetPlayer()
			if a := g.factory.SpawnRandomD1Unique(player.MapPosition()); a != nil {
				g.uiController.ShowLogMessage(fmt.Sprintf("Spawned %s", a.Name()))
			} else {
				g.uiController.ShowLogMessage("No free space to spawn it")
			}
		}},
	}
}

func (g *GameInstance) showDebugMenu() {
	g.uiController.OpenMenu(append(g.d1DebugItems(), []util.MenuItem{
		{
			MainText: "Toggle Debug Mode",
			Action: func() {
				g.debugModeEnabled = !g.debugModeEnabled
			},
		},
		{
			MainText: "Open class selection",
			Action: func() {
				g.uiController.OpenNewGameScreen(g.NewGame, func() {}) // back returns to the game
			},
		},
		{
			MainText: "Drop to ASCII Mode",
			Action: func() {
				g.quitToAscii = true
			},
		},
		{
			MainText: "Toggle Map Info",
			Action:   g.uiController.ToggleMapInfo,
		},
		{
			MainText: "Enchant Stash to +10",
			Action: func() {
				stash := g.state.GetPlayerStashInTown()
				for _, item := range stash.Items() {
					gameItem := item
					if item.IsWeapon() {
						gameItem.GetWeaponStats().Plus = 10
					}
					if item.IsArmor() {
						gameItem.GetArmorStats().Plus = 10
					}
				}
			},
		},
		{
			MainText: "Toggle UI Overlays", // should be an ui specific option
			Action: func() {
				if g.uiController.HasOverlays() {
					//g.uiController.ClearAllOverlays()
					//g.mapRenderer.OnScreenSizeChanged(g.GetDeviceIndependentScreenSize())
				} else {
					//g.setupUI()
				}
			},
		},
		{
			MainText: "Debug Map Info",
			Action: func() {
				visibleMap := g.uiController.GetVisibleMap()
				g.PlayerMessage(fmt.Sprintf("Visible map: (%d, %d) - (%d, %d)", visibleMap.Min.X, visibleMap.Min.Y, visibleMap.Max.X, visibleMap.Max.Y))
			},
		},
		{
			MainText: "Test Font Loading",
			Action: func() {
				foundFonts := findfont.List()
				for _, fontFile := range foundFonts {
					if strings.HasSuffix(fontFile, ".ttf") {
						g.PlayerMessage(fontFile)
					}
				}
			},
		},
		{
			MainText: "Open Stash",
			Action:   g.uiController.OpenPlayerStashInTown,
		},
		{
			MainText: "give stuff",
			Action: func() {
				playerStats := g.state.GetPlayer().GetStats()
				playerStats.SetAttribute(stats.MaxHealth, 10000)
				playerStats.GetResource(stats.Health).Increase(10000)
				playerStats.SetAttribute(stats.MaxMana, 10000)
				playerStats.GetResource(stats.Mana).Increase(10000)
				playerStats.SetAttribute(stats.Gold, 200000)

				inv := g.state.GetPlayer().GetInventory()
				for i := 0; i < 10; i++ {
					inv.Add(g.factory.NewD1Item(g.factory.D1().PlainItem(d1.IdxHeal)))
					inv.Add(g.factory.NewD1Item(g.factory.D1().PlainItem(d1.IdxMana)))
				}
				g.uiController.UpdateStatsLabel()
			},
		},
		{
			MainText: "give 10k XP",
			Action: func() {
				player := g.state.GetPlayer()
				player.Hero().AddExperience(player.Hero().Level, 10000)
				common.SyncHero(player)
				g.uiController.UpdateStatsLabel()
			},
		},
	}...))
}

func (g *GameInstance) killLevel() {
	actorsOnMap := g.state.GetMap().Actors()
	for i := len(actorsOnMap) - 1; i >= 0; i-- {
		actor := actorsOnMap[i]
		if actor == g.state.GetPlayer() || !actor.IsAlive() || actor.IsUnique() {
			continue
		}
		currentHP := actor.GetStats().GetResource(stats.Health).Value() * 2
		g.actionBroker.Execute(common.NewChangeResourceAction(common.NewActorCause("Kill All", g.state.GetPlayer()), actor, stats.Health, float64(-currentHP)))
	}
}

// getAIAction returns the action to execute and the cost in AUT associated with it
func (g *GameInstance) getAIAction(actor *common.Actor) (int, func()) {
	if actor.D1Monster() != nil {
		return g.getD1MonsterAction(actor)
	}
	autCostMove := actor.MoveCost()
	autCostMeleeAttack := actor.AttackCost(common.MeleeAttack)
	autCostRangedAttack := actor.AttackCost(common.RangedAttack)
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
		//println(fmt.Sprintf("%s is still unable to act. (Stunned)", actor.Name()))
		return 10, func() {}
	}

	canMove := actor.GetEnergy() >= autCostMove && !actor.HasStatusEffect(common.StatusImmobile)
	canAttack := actor.GetEnergy() >= minCostAttack && !actor.HasStatusEffect(common.StatusPacified)

	if !canMove && !canAttack {
		//println(fmt.Sprintf("%s can neither move nor attack. (Immobile & Pacified)", actor.Name()))
		return 10, func() {}
	}

	isScared := actor.HasStatusEffect(common.StatusScared)
	if isScared && canMove {
		return autCostMove, func() {
			targetPos := g.GetMoveAwayFromPlayer(actor.MapPosition())
			if g.state.IsClosedDoorAt(targetPos) {
				g.actionBroker.Execute(common.NewOpenDoorAction(common.AbstractCause("OpenDoor"), actor, targetPos))
			}
			g.actionBroker.Execute(common.NewMoveAction(common.AbstractCause("Movement"), actor, targetPos, false))
		}
	}

	isConfused := actor.HasStatusEffect(common.StatusConfused)

	if isConfused && canMove {
		return autCostMove, func() {
			possibleNeighbors := g.state.GetMap().GetFilteredNeighbors(actor.MapPosition(), func(point geometry.Point) bool {
				return g.state.GetMap().IsWalkable(point)
			})
			if len(possibleNeighbors) == 0 {
				return
			}
			randomNeighbor := possibleNeighbors[g.randomSource.Intn(len(possibleNeighbors))]
			if g.state.IsClosedDoorAt(randomNeighbor) {
				g.actionBroker.Execute(common.NewOpenDoorAction(common.AbstractCause("OpenDoor"), actor, randomNeighbor))
			}
			if g.state.GetMap().IsActorAt(randomNeighbor) {
				actorAt := g.state.GetMap().ActorAt(randomNeighbor)
				// attack
				g.actionBroker.Execute(common.NewMeleeAttackAction(common.AbstractCause("MeleeAttack"), actor, actorAt))
			} else {
				g.actionBroker.Execute(common.NewMoveAction(common.AbstractCause("Movement"), actor, randomNeighbor, false))
			}
		}
	}

	// random movement for idle actors
	if actor.GetState() == common.ActorStateIdle && canMove {
		return autCostMove, func() {
			freeNeighbor := g.state.GetMap().GetRandomFreeAndSafeNeighbor(g.randomSource, actor.MapPosition())
			if freeNeighbor != actor.MapPosition() {
				if g.state.IsClosedDoorAt(freeNeighbor) {
					g.actionBroker.Execute(common.NewOpenDoorAction(common.AbstractCause("OpenDoor"), actor, freeNeighbor))
				}
				moveAction := common.NewMoveAction(common.AbstractCause("Movement"), actor, freeNeighbor, false)
				if g.actions.IsPlayerAutoMoving() {
					moveAction.DisableAnimation()
				}
				g.actionBroker.Execute(moveAction)
			}
		}
	}

	player := g.state.GetPlayer()
	if actor.GetState() == common.ActorStateAttacking && player.IsAlive() {
		// can use a skill?
		playerPosition := player.MapPosition()

		untargetedSkills := actor.GetUntargetedSkills()
		if len(untargetedSkills) > 0 {
			for _, skill := range untargetedSkills {
				if !skill.CanActivate(actor) {
					continue
				}
				skillUseChance := g.randomSource.Intn(10) == 0
				if !skillUseChance {
					continue
				}
				return skill.GetTimeNeededInAut(), func() {
					skill.Invoke(actor, "ai_skill_usage")
				}
			}
		}

		targetedSkills := actor.GetTargetedSkills()
		if len(targetedSkills) > 0 {
			for _, skill := range targetedSkills {
				skillUseChance := g.randomSource.Intn(10) == 0
				if !skillUseChance {
					continue
				}
				targetingRules := skill.GetTargetingRules(actor, g.GetWorldState().GetMap())
				targetInfo := targetingRules.OnTargetChanged(playerPosition)
				if !targetInfo.IsValid {
					continue
				}
				return skill.GetTimeNeededInAut(), func() {
					skill.Execute(actor, actor.MapPosition(), playerPosition)
				}
			}
		}

		canRangedAttack := actor.GetEnergy() >= autCostRangedAttack && actor.GetEquipment().HasRangedWeaponEquipped() && !actor.HasStatusEffect(common.StatusPacified)
		canSeePlayer := actor.CanSee(playerPosition)
		if canRangedAttack && canSeePlayer {
			return autCostRangedAttack, func() {
				g.actionBroker.Execute(common.NewRangedAttackAction(common.AbstractCause("RangedAttack"), actor, playerPosition))
			}
		}
		var dist int
		dist = g.state.GetMap().MoveDistance(actor.MapPosition(), playerPosition)
		if dist <= 1 && canAttack {
			return autCostMeleeAttack, func() {
				g.actionBroker.Execute(common.NewMeleeAttackAction(common.AbstractCause("MeleeAttack"), actor, player))
			}
		} else if canMove {
			return autCostMove, func() {
				targetPos := g.GetMoveTowardsPlayer(actor.MapPosition())
				if g.state.IsClosedDoorAt(targetPos) {
					g.actionBroker.Execute(common.NewOpenDoorAction(common.AbstractCause("OpenDoor"), actor, targetPos))
				}
				g.actionBroker.Execute(common.NewMoveAction(common.AbstractCause("Movement"), actor, targetPos, false))
			}
		}
	}

	return 10, func() {}
}

func (g *GameInstance) onMapChanged(oldMapName string, newMapName string) {
	g.actions.stopAutoMove() // path & queued moves belong to the old map

	if defs.IsTownLevel(newMapName) {
		currentMap := g.state.GetMap()
		g.state.SetTownMap(currentMap)
		if newMapName == string(defs.LevelNameTown) {
			g.restockD1Town(currentMap)
			common.PlaceD1Portal(g.GetWorldState(), currentMap)
		}
	} else { // dungeon map
		g.state.SetDungeonMap(newMapName, g.state.GetMap())
		g.state.GetFlags().SetTrue(common.FlagBeenUnderground) // town brightens up once the player has been below
		if dlvl := common.LevelFromName(newMapName); dlvl > g.state.GetLastDungeonLevel() {
			g.state.SetLastDungeonLevel(dlvl) // opens town gates, raises store item levels
		}
	}

	if defs.IsTownLevel(oldMapName) {
		g.GetAudio().PlayCue("enter_dungeon")
	}
	g.onD1MapChanged(oldMapName, newMapName)

	g.actions.updateFoV()
	g.SaveGame()
	g.uiController.OnMapChanged()

	if g.onMapChange != nil {
		g.onMapChange()
	}
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

// checkForLevelUp plays the level-up sound once per new character level (CanLevelUp stays true while stat points
// are unspent). The first turn after a new game or load only records the level.
func (g *GameInstance) checkForLevelUp() {
	h := g.state.GetPlayer().Hero()
	if h == nil {
		return
	}
	if g.heroLevel != 0 && h.Level > g.heroLevel {
		g.GetAudio().PlayCue("level_up")
	}
	g.heroLevel = h.Level
}

func (g *GameInstance) ErrLog(errorMessage string) {
	// write to stderr
	fmt.Fprintln(os.Stderr, errorMessage)
}

func (g *GameInstance) GetGameData() defs.DataContainer {
	return g.dataDefinitions
}

func (g *GameInstance) GetActions() common.PlayerActions {
	return g.actions
}

func (g *GameInstance) PlayerMessage(message string) {
	//println(text)
	rawText := common.RemoveColorCodes(message)
	if g.logHook != nil {
		g.logHook(rawText)
	}
	if len(g.logBuffer) > 200 {
		g.logBuffer = g.logBuffer[1:]
	}
	g.logBuffer = append(g.logBuffer, message)

	g.GetUserInterface().ShowLogMessage(message)
}

func (g *GameInstance) SetLogHook(hook func(string)) {
	g.logHook = hook
}

func (g *GameInstance) GetLog() []string {
	return g.logBuffer
}

func (g *GameInstance) getDebugActions() common.DebugActions {
	return common.DebugActions{
		KillLevel:      g.killLevel,
		OpenDebugMenu:  g.showDebugMenu,
		GoOneLevelDown: g.actions.goOneLevelDown,
		GoOneLevelUp:   g.actions.goOneLevelUp,
	}
}

func d1ClassIndex(internalName string) int {
	switch internalName {
	case "rogue", "archer":
		return d1.Rogue
	case "sorcerer", "mage":
		return d1.Sorcerer
	case "monk":
		return d1.Monk
	case "bard":
		return d1.Bard
	case "barbarian":
		return d1.Barbarian
	}
	return d1.Warrior
}

func (g *GameInstance) GenerateLoot(dungeonLevel int, count int) []*common.Item {
	var result []*common.Item
	for len(result) < count {
		if it := g.factory.D1().FloorItem(dungeonLevel, true, dungeonLevel > 12); it != nil {
			result = append(result, g.factory.NewD1Item(it))
		}
	}
	return result
}
