package game

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"maps"
	"os"
	"path/filepath"
	"rpg/files"
	"rpg/src/common"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/rules"
	"rpg/src/stats"
	"rpg/src/util"
	"time"
)

var DefaultSaveDir = files.AppPath("saves") // next to the binary in a release, like config.rec

// SaveDir holds the save slots, one directory per mod: <mod>/slot<N>.json with its screenshot beside it as
// slot<N>.png. A mod's games are its own: another mod neither lists nor loads them.
var SaveDir = DefaultSaveDir

// saveMod is the mod whose saves are in play: the active one, or "fallback" for the engine's own room.
func saveMod() string {
	if m := files.ActiveMod(); m != "" {
		return m
	}
	return "fallback"
}

func modSaveDir() string { return filepath.Join(SaveDir, saveMod()) }

// LegacySaveFile is the single save slot the game had before slots existed; it is moved into slot 1 on the
// first listing, so an in-progress game survives the upgrade.
const LegacySaveFile = "savegame.json"

// MaxSaveSlots is how many slots the load/save screen offers.
const MaxSaveSlots = 5

func slotPath(slot int) string { return filepath.Join(modSaveDir(), fmt.Sprintf("slot%d.json", slot)) }
func slotThumbPath(slot int) string {
	return filepath.Join(modSaveDir(), fmt.Sprintf("slot%d.png", slot))
}

type savedInvItem struct {
	Item     rules.Item
	Equipped bool
}

type saveGame struct {
	Mod            string // the mod the game was played on; empty in saves from before mods, which were D1's
	Build          string // common.BuildHash of the binary that wrote it
	Class          string
	Female         bool
	Name           string
	Permadeath     bool
	Seed           int64
	Hero           rules.Hero
	HP, Mana, Gold int
	Items          []savedInvItem
	Belt           [common.BeltSize]common.SavedBeltSlot
	Stash          []rules.Item // the town stash
	UniqueDropped  []string     // unique_items.rec internal_names of the uniques already found
	Flags          map[string]int
	Map            string
	X, Y           int
	Levels         map[string]*common.LevelSnapshot
	Deepest        int    // shown in the slot list
	DeepestLevel   string // the deepest level and town visited (State.Visit)
	DeepestTown    string
	Premium        rules.Premium
	Boy            *rules.Item
	BoyLevel       int
	Portal         *common.Portal
	Golem          *common.SavedGolem  // the golem follows the hero (onDungeonMapChanged)
	ManaShield     bool                // only in saves from before Status was kept
	Status         map[string]int      // the hero's status effects and their turns left
	HeroID         uint64              `json:",omitempty"` // the hero's Actor.ID
	ShieldBy       uint64              `json:",omitempty"` // the ID of whoever cast the hero's Shield
	SavedAt        time.Time           // when this slot was last written; zero in saves from before slots
	Dungeon        common.DungeonClock // mod.rec's respawn timers and each level's generation
}

// ListSaveSlots reads every slot's metadata, in slot order, without loading a game. A slot that can't be read
// counts as empty, so a corrupt file is offered for overwriting rather than blocking the screen.
// ponytail: parses each slot's whole JSON for its header; write the metadata to its own small file if listing
// five saves ever becomes noticeable.
func ListSaveSlots(rules *rules.Data) []common.SaveSlot {
	migrateLegacySave()
	slots := make([]common.SaveSlot, 0, MaxSaveSlots)
	for slot := 1; slot <= MaxSaveSlots; slot++ {
		info, _ := readSaveSlot(slot, rules)
		slots = append(slots, info)
	}
	return slots
}

// readSaveSlot reads one slot's metadata. A slot that holds no game of this mod's, or one it can't load, reads as
// empty; the error says why, nil for a slot with no file.
func readSaveSlot(slot int, rules *rules.Data) (common.SaveSlot, error) {
	info := common.SaveSlot{Slot: slot}
	data, err := os.ReadFile(slotPath(slot))
	if err != nil {
		return info, nil
	}
	var sg saveGame
	if err := json.Unmarshal(data, &sg); err != nil {
		return info, fmt.Errorf("save file is corrupt: %w", err)
	}
	if !sg.forThisMod() {
		return info, nil
	}
	if rules != nil {
		if err := sg.check(rules); err != nil { // a save this mod can't load counts as empty, like a corrupt one
			return info, err
		}
	}
	info.Used, info.Name, info.Level = true, sg.Name, sg.Hero.Level
	info.Deepest, info.Permadeath, info.SavedAt = sg.Deepest, sg.Permadeath, sg.SavedAt
	if rules != nil {
		info.Class = rules.Class(sg.Class).Name
	}
	if _, err := os.Stat(slotThumbPath(slot)); err == nil {
		info.ThumbPath = slotThumbPath(slot)
	}
	return info, nil
}

// migrateLegacySave moves a pre-slots savegame.json into slot 1 when that slot is still free. Only the real
// save directory takes part: a test or a smoke run pointing SaveDir somewhere else must not walk off with the
// player's own game.
func migrateLegacySave() {
	if SaveDir != DefaultSaveDir {
		return
	}
	// Slots written before saves were kept per mod lie in SaveDir itself, and every one of them is D1's.
	for slot := 1; slot <= MaxSaveSlots; slot++ {
		for _, name := range []string{fmt.Sprintf("slot%d.json", slot), fmt.Sprintf("slot%d.png", slot)} {
			old, moved := filepath.Join(SaveDir, name), filepath.Join(SaveDir, legacyMod, name)
			if _, err := os.Stat(old); err != nil {
				continue
			}
			if _, err := os.Stat(moved); err == nil {
				continue
			}
			if os.MkdirAll(filepath.Dir(moved), 0o755) == nil {
				_ = os.Rename(old, moved)
			}
		}
	}
	legacy := filepath.Join(SaveDir, legacyMod, "slot1.json")
	if _, err := os.Stat(LegacySaveFile); err != nil {
		return
	}
	if _, err := os.Stat(legacy); err == nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		return
	}
	_ = os.Rename(LegacySaveFile, legacy)
}

// legacyMod is whose saves the ones from before mods are: the game was D1 then.
const legacyMod = "d1"

// forThisMod is whether the save belongs to the running mod. A save from before mods is D1's.
func (sg saveGame) forThisMod() bool {
	mod := sg.Mod
	if mod == "" {
		mod = legacyMod
	}
	return mod == saveMod()
}

// HasSave reports whether any slot holds a game.
func HasSave() bool { return LatestSaveSlot() > 0 }

// LatestSaveSlot is the most recently played slot, which is what Continue loads. 0 when every slot is empty.
func LatestSaveSlot() int {
	latest := 0
	var newest time.Time
	for _, s := range ListSaveSlots(nil) {
		if s.Used && (latest == 0 || s.SavedAt.After(newest)) {
			latest, newest = s.Slot, s.SavedAt
		}
	}
	return latest
}

// FirstFreeSaveSlot is the lowest empty slot, or 0 when every slot is taken.
func FirstFreeSaveSlot() int {
	for _, s := range ListSaveSlots(nil) {
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
	for id, m := range g.state.Maps() {
		if g.state.Level(id).Town {
			g.forgetBoughtStock(m)
		}
	}
	st := player.GetStats()
	pos := player.MapPosition()
	sg := saveGame{
		Mod: saveMod(), Class: g.state.SelectedClass.InternalName, Female: g.state.SelectedClass.IsFemale(),
		Name: player.Name(), Permadeath: g.permadeath, Seed: g.levelSeed, Hero: *player.Hero(),
		HP: max(st.GetResource(stats.Health).Value(), 1), Mana: st.GetResource(stats.Mana).Value(), Gold: st.GetAttribute(stats.Gold),
		UniqueDropped: g.factory.Gen().DroppedUniques(), Flags: g.state.GetFlags().AllEntries(),
		Map: g.state.GetMap().GetName(), X: pos.X, Y: pos.Y, Levels: map[string]*common.LevelSnapshot{},
		Deepest: g.state.Deepest(), DeepestLevel: g.state.DeepestLevel(), DeepestTown: g.state.DeepestTown(), Premium: g.premium, Boy: g.boyItem, BoyLevel: g.boyLevel, Portal: g.state.Portal(),
		Status: maps.Clone(player.GetStatusEffects().AllEntries()), HeroID: player.ID(), ShieldBy: player.ShieldedBy(), SavedAt: time.Now(), Build: common.BuildHash(), Dungeon: g.state.DungeonClock(),
	}
	if golem := common.LivingGolem(g.state.GetMap()); golem != nil {
		saved := common.SaveGolem(golem)
		sg.Golem = &saved
	}
	inv, eq := player.GetInventory(), player.GetEquipment()
	var savedItems []*common.Item // sg.Items' items, for the belt
	for _, it := range inv.Items() {
		if it.Roll() != nil {
			sg.Items = append(sg.Items, savedInvItem{Item: *it.Roll(), Equipped: eq.IsEquipped(it)})
			savedItems = append(savedItems, it)
		}
	}
	for _, it := range eq.AllItems() {
		if it.Roll() != nil && !inv.Has(it) {
			sg.Items = append(sg.Items, savedInvItem{Item: *it.Roll(), Equipped: true})
			savedItems = append(savedItems, it)
		}
	}
	sg.Belt = common.SaveBelt(player, savedItems)
	for _, it := range g.state.GetPlayerStashInTown().Items() {
		if it.Roll() != nil {
			sg.Stash = append(sg.Stash, *it.Roll())
		}
	}
	for name, snap := range g.levelSnapshots {
		sg.Levels[name] = snap
	}
	for name, m := range g.state.Maps() {
		if !g.state.Level(name).Town {
			sg.Levels[name] = common.SnapshotLevel(&g.state, m)
		}
	}
	data, err := json.Marshal(sg)
	if err != nil {
		g.ErrLog(fmt.Sprintf("saving failed: %v", err))
		return
	}
	if err := os.MkdirAll(modSaveDir(), 0o755); err != nil {
		g.ErrLog(fmt.Sprintf("saving failed: %v", err))
		return
	}
	if err := util.WriteFileAtomic(slotPath(slot), data); err != nil {
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
	var buf bytes.Buffer
	if png.Encode(&buf, shot) == nil {
		_ = util.WriteFileAtomic(slotThumbPath(slot), buf.Bytes())
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

// LoadGame reloads the running game's own slot, and the most recently played one when there is no running game
// (Continue from the start menu). Going by the directory alone was wrong for a game that has a slot: it reads
// back whatever file happens to be newest - another hero's save, or slot 0 when the autosave wrote nothing.
func (g *GameInstance) LoadGame() error {
	if g.slot > 0 {
		return g.LoadSlot(g.slot)
	}
	return g.LoadSlot(g.latestSlot())
}

// latestSlot is LatestSaveSlot among the saves this game can load: what the mod's data rejects, or what failed to
// load this session, is skipped.
func (g *GameInstance) latestSlot() int {
	latest := 0
	var newest time.Time
	for _, s := range ListSaveSlots(g.data) {
		if s.Used && !g.broken[s.Slot] && (latest == 0 || s.SavedAt.After(newest)) {
			latest, newest = s.Slot, s.SavedAt
		}
	}
	return latest
}

// LoadSlot restores a save slot: the world is rebuilt from the seed, then hero, items and level snapshots are
// applied. Autosaving continues into the slot that was loaded.
// check reports why the running mod can't load this save: every item must name a base item the mod has. It
// links them as it goes.
func (sg *saveGame) check(d *rules.Data) error {
	for _, it := range sg.items() {
		if !d.Relink(it) {
			return fmt.Errorf("its %s names no base item this mod has (a save from an older version?)", it.Name)
		}
	}
	return nil
}

// items are all the save's items: the hero's, the stash, the stores' and those lying on its levels.
func (sg *saveGame) items() []*rules.Item {
	var out []*rules.Item
	for i := range sg.Items {
		out = append(out, &sg.Items[i].Item)
	}
	for i := range sg.Stash {
		out = append(out, &sg.Stash[i])
	}
	for _, it := range sg.Premium.Items {
		if it != nil {
			out = append(out, it)
		}
	}
	if sg.Boy != nil {
		out = append(out, sg.Boy)
	}
	for _, snap := range sg.Levels {
		if snap != nil {
			for i := range snap.Items {
				out = append(out, &snap.Items[i].Item)
			}
		}
	}
	return out
}

func (g *GameInstance) LoadSlot(slot int) (err error) {
	defer func() {
		if r := recover(); r != nil { // a save that breaks halfway leaves a half-loaded game: start clean instead
			err = fmt.Errorf("loading slot %d failed: %v", slot, r)
			g.broken[slot] = true
			g.ResetGame(g.dataDefinitions.ClassDefinitions[0])
			g.slot, g.autosave = 0, false
		}
	}()
	data, err := os.ReadFile(slotPath(slot))
	if err != nil {
		return err
	}
	var sg saveGame
	if err := json.Unmarshal(data, &sg); err != nil {
		return fmt.Errorf("save file is corrupt: %w", err)
	}
	if !sg.forThisMod() {
		return fmt.Errorf("this game was saved with mod %q; load %q to continue it", sg.Mod, sg.Mod)
	}
	if err := sg.check(g.data); err != nil {
		return err
	}
	g.autosave = false
	class := g.classByName(sg.Class)
	if sg.Female {
		class.Icon = class.FemaleIcon
	}
	g.resetGame(class, sg.Seed)
	g.permadeath = sg.Permadeath

	player := g.state.GetPlayer()
	if sg.Name != "" {
		player.SetName(sg.Name)
	}
	*player.Hero() = sg.Hero
	player.Hero().Class = class.InternalName // saved once, as the save's Class
	g.data.Adopt(player.Hero())              // the rules pointer does not travel through JSON
	g.factory.Gen().SetDroppedUniques(sg.UniqueDropped)
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
		item := g.factory.NewRolledItem(&d)
		loadedItems = append(loadedItems, item)
		inv.Add(item)
		if si.Equipped {
			eq.Equip(item)
		}
	}
	for _, d := range sg.Stash {
		g.state.GetPlayerStashInTown().Add(g.factory.NewRolledItem(&d))
	}
	st := player.GetStats()
	st.SetAttribute(stats.Gold, sg.Gold)
	common.SyncHero(player)
	g.factory.RefreshSpells(player)
	common.LoadBelt(player, sg.Belt, loadedItems)
	common.SetResource(st.GetResource(stats.Health), sg.HP)
	common.SetResource(st.GetResource(stats.Mana), sg.Mana)
	for name, snap := range sg.Levels {
		g.levelSnapshots[name] = snap
	}
	g.state.SetVisited(sg.DeepestLevel, sg.DeepestTown)
	g.premium, g.boyItem, g.boyLevel = sg.Premium, sg.Boy, sg.BoyLevel
	g.state.SetPortal(sg.Portal)
	g.storesShown = false // the reset stocked the town; keep the saved premium and Wirt items on the next visit

	target := gridmap.Transition{TargetMap: sg.Map, TargetLocation: "ladder_up"}
	if g.state.Level(sg.Map).Town {
		target.TargetLocation = "spawn"
	}
	g.state.SetDungeonClock(sg.Dungeon) // before the transition: the level rebuilds as its saved generation
	g.state.MapTransition(target)
	g.state.SetDungeonClock(sg.Dungeon) // and after it: arriving is no town visit, and no turn passed
	m := g.state.GetMap()
	if p := (geometry.Point{X: sg.X, Y: sg.Y}); p != player.MapPosition() && m.CanPlaceActorHere(p) {
		m.MoveActor(player, p)
		player.SetDrawPosition(p.ToPointF())
	}
	if snap := sg.Levels[sg.Map]; snap != nil {
		snap.RestoreExplored(m) // the transition's arrival at the stairs saw cells the saved game never had
	}
	player.SetID(sg.HeroID)
	if sg.Golem != nil {
		g.factory.PlaceGolemNear(player, *sg.Golem)
	}
	if sg.ManaShield {
		player.AddStatusEffect(common.StatusManaShield, -1)
	}
	maps.Copy(player.GetStatusEffects().AllEntries(), sg.Status)
	player.SetShieldedBy(sg.ShieldBy)
	common.DropOrphanShields(g.state.GetMap(), player)
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
	g.uiController.OpenSaveSlots("New game - pick a save slot", ListSaveSlots(g.data),
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
	info, err := readSaveSlot(slot, g.data)
	if !info.Used && err == nil || slot == ownSlot { // a save that doesn't load is still somebody's file: ask

		write()
		return
	}
	g.uiController.Confirm(fmt.Sprintf("Overwrite %s?", info.Summary()), write)
}

// OpenSaveMenu is the pause menu's "Save game": pick a slot, confirm if it holds someone else's game.
func (g *GameInstance) OpenSaveMenu(back func()) {
	g.uiController.OpenSaveSlots("Save game", ListSaveSlots(g.data), func(slot int) {
		g.confirmOverwrite(slot, g.slot, func() {
			g.SaveToSlot(slot)
			g.PlayerMessage(fmt.Sprintf("Game saved to slot %d.", slot))
		})
	}, back)
}

// OpenLoadMenu is "Load game" in the start and pause menus. An empty slot is simply not loadable.
func (g *GameInstance) OpenLoadMenu(back func()) {
	g.uiController.OpenSaveSlots("Load game", ListSaveSlots(g.data), func(slot int) {
		if info, _ := readSaveSlot(slot, g.data); !info.Used {
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
	if g.latestSlot() > 0 {
		if afterDeath {
			items = append(items, util.MenuItem{MainText: "Load last save", Action: func() {
				if err := g.LoadGame(); err != nil {
					g.PlayerMessage(fmt.Sprintf("Could not load the game: %v", err))
				}
			}})
		} else {
			// ShowStartMenu already loaded it, so the menu sits over that exact scene: closing is all Continue does.
			items = append(items, util.MenuItem{MainText: "Continue"})
		}
		items = append(items, util.MenuItem{MainText: "Load game", Action: func() { g.OpenLoadMenu(back) }})
	}
	items = append(items, util.MenuItem{MainText: "New game", Action: func() { g.uiController.OpenNewGameScreen(g.NewGame, back) }})
	if mods := files.Mods(); !afterDeath && len(mods) > 1 {
		items = append(items, util.MenuItem{MainText: "Mod: " + files.ActiveMod(), Action: func() { g.openModPicker(mods, back) }})
	}
	if !afterDeath {
		items = append(items, util.MenuItem{MainText: "Settings", Action: func() { g.uiController.OpenSettingsMenu(back) }})
	}
	return append(items, util.MenuItem{MainText: "Quit", Action: g.uiController.QuitGame})
}

// openModPicker switches mods in place: the choice goes into the config file, this game's sounds stop, and the UI
// builds a fresh game on the new mod (SetModSwitcher) -- every icon, sprite, sound and definition is read anew.
// SetModSwitcher is how the UI replaces this game with one on the mod just saved to the config file.
func (g *GameInstance) SetModSwitcher(switchMod func()) { g.switchMod = switchMod }

func (g *GameInstance) openModPicker(mods []string, back func()) {
	var items []util.MenuItem
	for _, mod := range mods {
		if mod == files.ActiveMod() {
			continue
		}
		items = append(items, util.MenuItem{MainText: mod, Action: func() {
			if err := common.SaveMod(common.ConfigFile, mod); err != nil {
				g.PlayerMessage(fmt.Sprintf("Could not switch mods: %v", err))
				return
			}
			g.audioPlayer.StopAll()
			g.switchMod()
		}})
	}
	g.uiController.OpenUncancelableMenu(append(items, util.MenuItem{MainText: "Back", Action: back}))
}

// BackToMainMenu leaves a running game for the start menu. It saves first, so that picking Continue puts the
// player back where they left off rather than at the last level change (the same promise as quitting, decision 11).
func (g *GameInstance) BackToMainMenu() {
	g.SaveGame()
	g.uiController.OpenUncancelableMenu(g.StartMenuItems(false))
}

// ShowStartMenu opens the start menu when a save exists or there is a mod to pick; without one the default game autosaves right away.
// A continuable save is loaded first, so the menu comes up over that exact scene: Continue then just closes it.
func (g *GameInstance) ShowStartMenu() {
	// the newest save that loads; one that doesn't is skipped, with the reason on stderr
	for slot := 1; slot <= MaxSaveSlots; slot++ {
		if _, err := readSaveSlot(slot, g.data); err != nil {
			fmt.Fprintf(os.Stderr, "skipping save slot %d: %v\n", slot, err)
		}
	}
	for slot := g.latestSlot(); slot > 0; slot = g.latestSlot() {
		err := g.LoadSlot(slot)
		if err == nil {
			break
		}
		fmt.Fprintf(os.Stderr, "skipping save slot %d: %v\n", slot, err)
		g.broken[slot] = true
	}
	if g.slot == 0 && len(files.Mods()) < 2 && g.latestSlot() == 0 { // with several mods the menu is where the mod is picked
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
