package game

import (
	"encoding/json"
	"fmt"
	"os"
	"rpg/common"
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
	"rpg/util"
)

// SaveFile is the single save slot (decision 11).
var SaveFile = "savegame.json"

type savedInvItem struct {
	Item     d1.Item
	Equipped bool
}

type saveGame struct {
	Class          string
	Female         bool
	Name           string
	Permadeath     bool
	Seed           int64
	Hero           d1.Hero
	HP, Mana, Gold int
	Items          []savedInvItem
	Stash          []d1.Item // the town stash
	UniqueDropped  []bool
	Flags          map[string]int
	Map            string
	X, Y           int
	Levels         map[string]*common.LevelSnapshot
	Deepest        int
	Premium        d1.Premium
	Boy            *d1.Item
	BoyLevel       int
	Portal         *common.D1Portal
	Golem          *common.SavedGolem // the golem follows the hero (onD1MapChanged)
	ManaShield     bool
	D1Only         bool // played without Hellfire; false for saves from before the toggle
}

func HasSave() bool {
	_, err := os.Stat(SaveFile)
	return err == nil
}

// SaveGame writes the save slot. It does nothing until a game was started or loaded through the menus,
// so a save file is never overwritten by the throwaway game behind the start menu.
func (g *GameInstance) SaveGame() {
	player := g.state.GetPlayer()
	if !g.autosave || player == nil || !player.IsAlive() || player.Hero() == nil {
		return
	}
	g.forgetBoughtD1Stock(g.state.GetTownMap(string(defs.LevelNameTown)))
	st := player.GetStats()
	pos := player.MapPosition()
	sg := saveGame{
		Class: g.state.SelectedClass.InternalName, Female: g.state.SelectedClass.Icon == defs.FemaleIconByClass[g.state.SelectedClass.InternalName],
		Name: player.Name(), Permadeath: g.permadeath, Seed: g.levelSeed, Hero: *player.Hero(),
		HP: max(st.GetResource(stats.Health).Value(), 1), Mana: st.GetResource(stats.Mana).Value(), Gold: st.GetAttribute(stats.Gold),
		UniqueDropped: g.factory.D1().UniqueDropped, Flags: g.state.GetFlags().AllEntries(),
		Map: g.state.GetMap().GetName(), X: pos.X, Y: pos.Y, Levels: map[string]*common.LevelSnapshot{},
		Deepest: g.state.GetLastDungeonLevel(), Premium: g.premium, Boy: g.boyItem, BoyLevel: g.boyLevel, Portal: g.state.D1Portal(),
		ManaShield: player.HasStatusEffect(common.StatusManaShield), D1Only: !g.hellfire,
	}
	if golem := common.D1Golem(g.state.GetMap()); golem != nil {
		saved := common.SaveD1Golem(golem)
		sg.Golem = &saved
	}
	inv, eq := player.GetInventory(), player.GetEquipment()
	for _, it := range inv.Items() {
		if it.D1() != nil {
			sg.Items = append(sg.Items, savedInvItem{Item: *it.D1(), Equipped: eq.IsEquipped(it)})
		}
	}
	for _, it := range eq.AllItems() {
		if it.D1() != nil && !inv.Has(it) {
			sg.Items = append(sg.Items, savedInvItem{Item: *it.D1(), Equipped: true})
		}
	}
	for _, it := range g.state.GetPlayerStashInTown().Items() {
		if it.D1() != nil {
			sg.Stash = append(sg.Stash, *it.D1())
		}
	}
	for name, snap := range g.levelSnapshots {
		sg.Levels[name] = snap
	}
	for name, m := range g.state.DungeonMaps() {
		sg.Levels[name] = common.SnapshotLevel(&g.state, m)
	}
	data, err := json.Marshal(sg)
	if err != nil {
		g.ErrLog(fmt.Sprintf("saving failed: %v", err))
		return
	}
	tmp := SaveFile + ".tmp" // write then rename, so a crash never leaves a half-written save
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		g.ErrLog(fmt.Sprintf("saving failed: %v", err))
		return
	}
	if err := os.Rename(tmp, SaveFile); err != nil {
		g.ErrLog(fmt.Sprintf("saving failed: %v", err))
	}
}

func (g *GameInstance) classByName(internalName string) defs.LoadoutDefinition {
	for _, c := range g.dataDefinitions.ClassDefinitions {
		if c.InternalName == internalName {
			return c
		}
	}
	return g.dataDefinitions.ClassDefinitions[0]
}

// LoadGame restores the save slot: the world is rebuilt from the seed, then hero, items and level snapshots are applied.
func (g *GameInstance) LoadGame() error {
	data, err := os.ReadFile(SaveFile)
	if err != nil {
		return err
	}
	var sg saveGame
	if err := json.Unmarshal(data, &sg); err != nil {
		return fmt.Errorf("save file is corrupt: %w", err)
	}
	g.autosave = false
	class := g.classByName(sg.Class)
	if sg.Female {
		class.Icon = defs.FemaleIconByClass[class.InternalName]
	}
	g.hellfire = !sg.D1Only || g.configuration.Hellfire // turning Hellfire on upgrades a D1 save, never back
	if sg.D1Only && g.hellfire {                        // Hellfire rolls levels 1-16 differently: the snapshots wouldn't fit, start over in town
		sg.Levels, sg.Portal, sg.Map = nil, nil, string(defs.LevelNameTown)
	}
	g.resetGame(class, sg.Seed)
	g.permadeath = sg.Permadeath

	player := g.state.GetPlayer()
	if sg.Name != "" {
		player.SetName(sg.Name)
	}
	*player.Hero() = sg.Hero
	copy(g.factory.D1().UniqueDropped, sg.UniqueDropped) // Hellfire's uniques come after D1's
	for k, v := range sg.Flags {
		g.state.GetFlags().SetToValue(k, v)
	}
	g.state.ApplyHellfireOptions()
	inv, eq := player.GetInventory(), player.GetEquipment()
	for _, it := range append([]*common.Item(nil), eq.AllItems()...) {
		eq.UnEquip(it)
	}
	for _, it := range append([]*common.Item(nil), inv.Items()...) {
		inv.Remove(it)
	}
	for _, si := range sg.Items {
		d := si.Item
		item := g.factory.NewD1Item(&d)
		inv.Add(item)
		if si.Equipped {
			eq.Equip(item)
		}
	}
	for _, d := range sg.Stash {
		g.state.GetPlayerStashInTown().Add(g.factory.NewD1Item(&d))
	}
	st := player.GetStats()
	st.SetAttribute(stats.Gold, sg.Gold)
	common.SyncHero(player)
	g.factory.RefreshD1Spells(player)
	common.SetResource(st.GetResource(stats.Health), sg.HP)
	common.SetResource(st.GetResource(stats.Mana), sg.Mana)
	for name, snap := range sg.Levels {
		g.levelSnapshots[name] = snap
	}
	g.state.SetLastDungeonLevel(max(sg.Deepest, 1))
	g.premium, g.boyItem, g.boyLevel = sg.Premium, sg.Boy, sg.BoyLevel
	g.state.SetD1Portal(sg.Portal)
	g.storesShown = false // the reset stocked the town; keep the saved premium and Wirt items on the next visit

	target := gridmap.Transition{TargetMap: sg.Map, TargetLocation: "ladder_up"}
	if defs.IsTownLevel(sg.Map) {
		target.TargetLocation = "spawn"
	} else if dlvl := common.LevelFromName(sg.Map); dlvl == 17 || dlvl == 21 { // the Nest and the Crypt start at a warp
		target.TargetLocation = "warp"
	}
	g.state.MapTransition(target)
	m := g.state.GetMap()
	if p := (geometry.Point{X: sg.X, Y: sg.Y}); p != player.MapPosition() && m.CanPlaceActorHere(p) {
		m.MoveActor(player, p)
		player.SetDrawPosition(p.ToPointF())
	}
	if sg.Golem != nil {
		g.factory.PlaceD1GolemNear(player, *sg.Golem)
	}
	if sg.ManaShield {
		player.AddStatusEffect(common.StatusManaShield, -1)
	}
	g.actionBroker.Execute(common.NewUpdateFoVAction(common.AbstractCause("Load game"), player))
	g.uiController.OnPlayerTurnTaken() // the transition drew the UI at the ladder, before the saved position
	g.autosave = true
	return nil
}

// NewGame starts a fresh game and enables autosaving.
func (g *GameInstance) NewGame(class defs.LoadoutDefinition, permadeath bool, name string) {
	g.autosave = false
	g.hellfire = g.configuration.Hellfire
	g.resetGame(class, g.randomSource.Int63())
	if name != "" {
		g.state.GetPlayer().SetName(name)
	}
	g.permadeath = permadeath
	g.autosave = true
	g.SaveGame()
}

// StartMenuItems builds the start menu, or the death menu when afterDeath is set (which ends autosaving,
// and deletes the save in permadeath games).
func (g *GameInstance) StartMenuItems(afterDeath bool) []util.MenuItem {
	var items []util.MenuItem
	if afterDeath {
		g.autosave = false
		if g.permadeath {
			os.Remove(SaveFile)
		}
	}
	if HasSave() {
		label := "Continue"
		if afterDeath {
			label = "Load last save"
		}
		items = append(items, util.MenuItem{MainText: label, Action: func() {
			if err := g.LoadGame(); err != nil {
				g.PlayerMessage(fmt.Sprintf("Could not load the game: %v", err))
			}
		}})
	}
	back := func() { g.uiController.OpenUncancelableMenu(g.StartMenuItems(false)) }
	if afterDeath {
		back = g.uiController.ShowGameOverMenu
	}
	return append(items, util.MenuItem{MainText: "New game", Action: func() { g.uiController.OpenNewGameScreen(g.NewGame, back) }},
		common.HellfireSetting(g.configuration, back),
		util.MenuItem{MainText: "Quit", Action: g.uiController.QuitGame})
}

// BackToMainMenu leaves a running game for the start menu. It saves first, so that picking Continue puts the
// player back where they left off rather than at the last level change (the same promise as quitting, decision 11).
func (g *GameInstance) BackToMainMenu() {
	g.SaveGame()
	g.uiController.OpenUncancelableMenu(g.StartMenuItems(false))
}

// ShowStartMenu opens Continue/New game when a save exists; without one the default game autosaves right away.
func (g *GameInstance) ShowStartMenu() {
	if !HasSave() {
		g.autosave = true
		return
	}
	g.uiController.OpenUncancelableMenu(g.StartMenuItems(false))
}
