package game

import (
	"encoding/json"
	"fmt"
	"image/png"
	"maps"
	"os"
	"path/filepath"
	"rpg/common"
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
	"rpg/util"
	"time"
)

const defaultSaveDir = "saves"

// SaveDir holds the save slots: slot<N>.json with its screenshot next to it as slot<N>.png.
var SaveDir = defaultSaveDir

// LegacySaveFile is the single save slot the game had before slots existed; it is moved into slot 1 on the
// first listing, so an in-progress game survives the upgrade.
const LegacySaveFile = "savegame.json"

// MaxSaveSlots is how many slots the load/save screen offers.
const MaxSaveSlots = 5

func slotPath(slot int) string      { return filepath.Join(SaveDir, fmt.Sprintf("slot%d.json", slot)) }
func slotThumbPath(slot int) string { return filepath.Join(SaveDir, fmt.Sprintf("slot%d.png", slot)) }

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
	Belt           [common.BeltSize]common.SavedBeltSlot
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
	ManaShield     bool               // only in saves from before Status was kept
	Status         map[string]int     // the hero's status effects and their turns left
	SavedAt        time.Time          // when this slot was last written; zero in saves from before slots
}

// ListSaveSlots reads every slot's metadata, in slot order, without loading a game. A slot that can't be read
// counts as empty, so a corrupt file is offered for overwriting rather than blocking the screen.
// ponytail: parses each slot's whole JSON for its header; write the metadata to its own small file if listing
// five saves ever becomes noticeable.
func ListSaveSlots() []common.SaveSlot {
	migrateLegacySave()
	slots := make([]common.SaveSlot, 0, MaxSaveSlots)
	for slot := 1; slot <= MaxSaveSlots; slot++ {
		slots = append(slots, readSaveSlot(slot))
	}
	return slots
}

func readSaveSlot(slot int) common.SaveSlot {
	info := common.SaveSlot{Slot: slot}
	data, err := os.ReadFile(slotPath(slot))
	if err != nil {
		return info
	}
	var sg saveGame
	if err := json.Unmarshal(data, &sg); err != nil {
		return info
	}
	info.Used, info.Name, info.Level = true, sg.Name, sg.Hero.Level
	info.Deepest, info.Permadeath, info.SavedAt = sg.Deepest, sg.Permadeath, sg.SavedAt
	if sg.Hero.Class >= 0 && sg.Hero.Class < len(d1.ClassNames) {
		info.Class = d1.ClassNames[sg.Hero.Class]
	}
	if _, err := os.Stat(slotThumbPath(slot)); err == nil {
		info.ThumbPath = slotThumbPath(slot)
	}
	return info
}

// migrateLegacySave moves a pre-slots savegame.json into slot 1 when that slot is still free. Only the real
// save directory takes part: a test or a smoke run pointing SaveDir somewhere else must not walk off with the
// player's own game.
func migrateLegacySave() {
	if SaveDir != defaultSaveDir {
		return
	}
	if _, err := os.Stat(LegacySaveFile); err != nil {
		return
	}
	if _, err := os.Stat(slotPath(1)); err == nil {
		return
	}
	if err := os.MkdirAll(SaveDir, 0o755); err != nil {
		return
	}
	_ = os.Rename(LegacySaveFile, slotPath(1))
}

// HasSave reports whether any slot holds a game.
func HasSave() bool { return LatestSaveSlot() > 0 }

// LatestSaveSlot is the most recently played slot, which is what Continue loads. 0 when every slot is empty.
func LatestSaveSlot() int {
	latest := 0
	var newest time.Time
	for _, s := range ListSaveSlots() {
		if s.Used && (latest == 0 || s.SavedAt.After(newest)) {
			latest, newest = s.Slot, s.SavedAt
		}
	}
	return latest
}

// FirstFreeSaveSlot is the lowest empty slot, or 0 when every slot is taken.
func FirstFreeSaveSlot() int {
	for _, s := range ListSaveSlots() {
		if !s.Used {
			return s.Slot
		}
	}
	return 0
}

// SaveGame autosaves into the slot the running game belongs to. It does nothing until a game was started or
// loaded through the menus, so no save is overwritten by the throwaway game behind the start menu.
func (g *GameInstance) SaveGame() {
	if g.QuitSaves() {
		g.writeSlot(g.slot)
	}
}

// SaveSlot is the slot the running game autosaves into; 0 when it belongs to no slot yet.
func (g *GameInstance) SaveSlot() int { return g.slot }

// QuitSaves reports whether SaveGame, and so quitting (decision 11), would write the game right now.
func (g *GameInstance) QuitSaves() bool {
	if !g.autosave || g.slot <= 0 {
		return false
	}
	p := g.state.GetPlayer()
	return p != nil && p.IsAlive() && p.Hero() != nil
}

// SaveToSlot writes the running game into slot and makes it the one autosaving continues into. The caller has
// already confirmed the overwrite (confirmOverwrite).
func (g *GameInstance) SaveToSlot(slot int) {
	g.slot = slot
	g.writeSlot(slot)
}

// writeSlot writes the game as it stands into a slot, with a screenshot beside it. Slot 0 means the game is
// bound to no slot yet (the menu's throwaway game), which is never written.
func (g *GameInstance) writeSlot(slot int) {
	player := g.state.GetPlayer()
	if slot <= 0 || player == nil || !player.IsAlive() || player.Hero() == nil {
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
		Status: maps.Clone(player.GetStatusEffects().AllEntries()), SavedAt: time.Now(),
	}
	if golem := common.D1Golem(g.state.GetMap()); golem != nil {
		saved := common.SaveD1Golem(golem)
		sg.Golem = &saved
	}
	inv, eq := player.GetInventory(), player.GetEquipment()
	var savedItems []*common.Item // sg.Items' items, for the belt
	for _, it := range inv.Items() {
		if it.D1() != nil {
			sg.Items = append(sg.Items, savedInvItem{Item: *it.D1(), Equipped: eq.IsEquipped(it)})
			savedItems = append(savedItems, it)
		}
	}
	for _, it := range eq.AllItems() {
		if it.D1() != nil && !inv.Has(it) {
			sg.Items = append(sg.Items, savedInvItem{Item: *it.D1(), Equipped: true})
			savedItems = append(savedItems, it)
		}
	}
	sg.Belt = common.SaveBelt(player, savedItems)
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
	if err := os.MkdirAll(SaveDir, 0o755); err != nil {
		g.ErrLog(fmt.Sprintf("saving failed: %v", err))
		return
	}
	path := slotPath(slot)
	tmp := path + ".tmp" // write then rename, so a crash never leaves a half-written save
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		g.ErrLog(fmt.Sprintf("saving failed: %v", err))
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		g.ErrLog(fmt.Sprintf("saving failed: %v", err))
		return
	}
	g.writeSlotThumbnail(slot)
}

// writeSlotThumbnail stores the last frame of the world next to the save, for the load screen. A UI that can't
// draw (the terminal, the headless tests) simply has no screenshot, and a slot that just changed hands drops
// the one it had, so the picture and the metadata can never disagree.
func (g *GameInstance) writeSlotThumbnail(slot int) {
	shot := g.uiController.CaptureThumbnail()
	if shot == nil {
		os.Remove(slotThumbPath(slot))
		return
	}
	file, err := os.Create(slotThumbPath(slot) + ".tmp")
	if err != nil {
		return
	}
	if err := png.Encode(file, shot); err != nil {
		file.Close()
		_ = os.Remove(file.Name())
		return
	}
	file.Close()
	_ = os.Rename(file.Name(), slotThumbPath(slot))
}

func (g *GameInstance) classByName(internalName string) defs.LoadoutDefinition {
	for _, c := range g.dataDefinitions.ClassDefinitions {
		if c.InternalName == internalName {
			return c
		}
	}
	return g.dataDefinitions.ClassDefinitions[0]
}

// LoadGame reloads the running game's own slot, and the most recently played one when there is no running game
// (Continue from the start menu). Going by the directory alone was wrong for a game that has a slot: it reads
// back whatever file happens to be newest - another hero's save, or slot 0 when the autosave wrote nothing.
func (g *GameInstance) LoadGame() error {
	if g.slot > 0 {
		return g.LoadSlot(g.slot)
	}
	return g.LoadSlot(LatestSaveSlot())
}

// LoadSlot restores a save slot: the world is rebuilt from the seed, then hero, items and level snapshots are
// applied. Autosaving continues into the slot that was loaded.
func (g *GameInstance) LoadSlot(slot int) error {
	data, err := os.ReadFile(slotPath(slot))
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
	g.resetGame(class, sg.Seed)
	g.permadeath = sg.Permadeath

	player := g.state.GetPlayer()
	if sg.Name != "" {
		player.SetName(sg.Name)
	}
	*player.Hero() = sg.Hero
	copy(g.factory.D1().UniqueDropped, sg.UniqueDropped)
	for k, v := range sg.Flags {
		g.state.GetFlags().SetToValue(k, v)
	}
	inv, eq := player.GetInventory(), player.GetEquipment()
	for _, it := range append([]*common.Item(nil), eq.AllItems()...) {
		eq.UnEquip(it)
	}
	for _, it := range append([]*common.Item(nil), inv.Items()...) {
		inv.Remove(it)
	}
	var loadedItems []*common.Item // sg.Items' items, for the belt
	for _, si := range sg.Items {
		d := si.Item
		item := g.factory.NewD1Item(&d)
		loadedItems = append(loadedItems, item)
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
	common.LoadBelt(player, sg.Belt, loadedItems)
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
	maps.Copy(player.GetStatusEffects().AllEntries(), sg.Status)
	g.actionBroker.Execute(common.NewUpdateFoVAction(common.AbstractCause("Load game"), player))
	g.uiController.OnPlayerTurnTaken() // the transition drew the UI at the ladder, before the saved position
	g.slot = slot
	g.autosave = true
	return nil
}

// NewGame asks which slot the new hero goes into, free ones included: the player always picks. A taken slot is
// confirmed first, so a new hero never silently writes over an old one.
func (g *GameInstance) NewGame(class defs.LoadoutDefinition, permadeath bool, name string) {
	back := func() { g.uiController.OpenUncancelableMenu(g.StartMenuItems(false)) }
	g.uiController.OpenSaveSlots("New game - pick a save slot", ListSaveSlots(),
		// ownSlot 0: a new hero is nobody's running game, so replacing even the slot that is open right now asks
		func(slot int) {
			g.confirmOverwrite(slot, 0, func() { g.startNewGame(class, permadeath, name, slot) })
		}, back)
}

func (g *GameInstance) startNewGame(class defs.LoadoutDefinition, permadeath bool, name string, slot int) {
	g.autosave = false
	g.resetGame(class, g.randomSource.Int63())
	if name != "" {
		g.state.GetPlayer().SetName(name)
	}
	g.permadeath = permadeath
	g.slot = slot
	g.autosave = true
	g.SaveGame()
	// this save was written on the frame the menu was still up, so its screenshot is of whatever was on screen
	// then. The first turn saves once more, by which time the frame is this hero's own game.
	g.refreshThumbnail = true
}

// confirmOverwrite asks before a save is written over, and never writes on a "no". An empty slot and ownSlot -
// the slot the game asking is already autosaving into - need no question, since neither loses a hero.
func (g *GameInstance) confirmOverwrite(slot, ownSlot int, write func()) {
	info := readSaveSlot(slot)
	if !info.Used || slot == ownSlot {
		write()
		return
	}
	g.uiController.Confirm(fmt.Sprintf("Overwrite %s?", info.Summary()), write)
}

// OpenSaveMenu is the pause menu's "Save game": pick a slot, confirm if it holds someone else's game.
func (g *GameInstance) OpenSaveMenu(back func()) {
	g.uiController.OpenSaveSlots("Save game", ListSaveSlots(), func(slot int) {
		g.confirmOverwrite(slot, g.slot, func() {
			g.SaveToSlot(slot)
			g.PlayerMessage(fmt.Sprintf("Game saved to slot %d.", slot))
		})
	}, back)
}

// OpenLoadMenu is "Load game" in the start and pause menus. An empty slot is simply not loadable.
func (g *GameInstance) OpenLoadMenu(back func()) {
	g.uiController.OpenSaveSlots("Load game", ListSaveSlots(), func(slot int) {
		if !readSaveSlot(slot).Used {
			g.OpenLoadMenu(back) // an empty slot holds nothing to load: put the screen back up
			return
		}
		if err := g.LoadSlot(slot); err != nil {
			g.PlayerMessage(fmt.Sprintf("Could not load the game: %v", err))
			back()
		}
	}, back)
}

// IsPermadeath reports whether the running game deletes its save on death, which is where the load/save menus
// are hidden: reloading would undo the one rule permadeath has.
func (g *GameInstance) IsPermadeath() bool { return g.permadeath }

// StartMenuItems builds the start menu, or the death menu when afterDeath is set (which ends autosaving,
// and deletes the save in permadeath games).
func (g *GameInstance) StartMenuItems(afterDeath bool) []util.MenuItem {
	var items []util.MenuItem
	back := func() { g.uiController.OpenUncancelableMenu(g.StartMenuItems(false)) }
	if afterDeath {
		g.autosave = false
		if g.permadeath && g.slot > 0 {
			os.Remove(slotPath(g.slot))
			os.Remove(slotThumbPath(g.slot))
		}
		back = g.uiController.ShowGameOverMenu
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
		}}, util.MenuItem{MainText: "Load game", Action: func() { g.OpenLoadMenu(back) }})
	}
	return append(items, util.MenuItem{MainText: "New game", Action: func() { g.uiController.OpenNewGameScreen(g.NewGame, back) }},
		util.MenuItem{MainText: "Quit", Action: g.uiController.QuitGame})
}

// BackToMainMenu leaves a running game for the start menu. It saves first, so that picking Continue puts the
// player back where they left off rather than at the last level change (the same promise as quitting, decision 11).
func (g *GameInstance) BackToMainMenu() {
	g.SaveGame()
	g.uiController.OpenUncancelableMenu(g.StartMenuItems(false))
}

// ShowStartMenu opens the start menu when a save exists; without one the default game autosaves right away.
func (g *GameInstance) ShowStartMenu() {
	if !HasSave() {
		g.slot = FirstFreeSaveSlot()
		g.autosave = true
		return
	}
	g.uiController.OpenUncancelableMenu(g.StartMenuItems(false))
}

// QuickSave writes the running game into its own slot (F5). Permadeath games and the menu's throwaway game
// have no slot to write, and say so.
func (g *GameInstance) QuickSave() {
	if g.permadeath || g.slot <= 0 {
		g.PlayerMessage("There is nothing to quick save.")
		return
	}
	g.writeSlot(g.slot)
	g.PlayerMessage(fmt.Sprintf("Quick saved to slot %d.", g.slot))
}

// QuickLoad reloads the running game's own slot (F9), throwing away everything since the last save.
func (g *GameInstance) QuickLoad() {
	if g.permadeath || g.slot <= 0 {
		g.PlayerMessage("There is nothing to quick load.")
		return
	}
	if err := g.LoadSlot(g.slot); err != nil {
		g.PlayerMessage("Quick load failed: " + err.Error())
	}
}
