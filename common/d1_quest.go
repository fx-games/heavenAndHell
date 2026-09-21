package common

import (
	"fmt"
	"io"
	"path"
	"rpg/d1"
	"rpg/defs"
	"rpg/files"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
	"rpg/util"
	"strings"
)

// D1's quests (quests.cpp). Which quests a game has comes from its seed (InitQuests, State.D1Quests, not saved);
// what happened since lives in the saved flags under the quest's key: "quest:<key>" once done, ".active" once
// active, ".log" once in the quest log, ".v1"/".v2" its _qvar1/_qvar2. The keys, and everything else a quest
// says or places, come from the mod's quests.rec.

// QuestDefs are the mod's quests, in quest-id order.
func (s *State) QuestDefs() []defs.Quest { return s.factory.D1().D.QuestDefs }

// QuestFlag is the flag set once quest q is done. A quest id the mod's quests.rec does not reach has no flag:
// the engine names a few quests by id (the one that opens new game plus, the ones the town hands in), and a mod
// with a shorter quest list simply does not have them.
func (s *State) QuestFlag(q int) string {
	defs := s.QuestDefs()
	if q < 0 || q >= len(defs) {
		return ""
	}
	return "quest:" + defs[q].Key
}

// QuestByKey is the id of the quest a record names, or -1.
func (s *State) QuestByKey(key string) int {
	for i, q := range s.QuestDefs() {
		if q.Key == key {
			return i
		}
	}
	return -1
}

// QuestActive is quests[q]._qactive.
func (s *State) QuestActive(q int) int {
	if s.QuestFlag(q) == "" {
		return d1.QuestNotAvail // a quest this mod does not have is never active
	}
	switch f := s.GetFlags(); {
	case f.IsSet(s.QuestFlag(q)):
		return d1.QuestDone
	case f.IsSet(s.QuestFlag(q) + ".active"):
		return d1.QuestActive
	}
	return s.mapGen.d1Quest(q).Active
}

func (s *State) SetQuestActive(q, v int) {
	switch v {
	case d1.QuestDone:
		s.GetFlags().SetTrue(s.QuestFlag(q))
	case d1.QuestActive:
		s.GetFlags().SetTrue(s.QuestFlag(q) + ".active")
	}
}

func (s *State) QuestLog(q int) bool { return s.GetFlags().IsSet(s.QuestFlag(q) + ".log") }

// startQuest makes quest q active and puts it in the quest log.
func (s *State) startQuest(q int) {
	if s.QuestActive(q) != d1.QuestDone {
		s.SetQuestActive(q, d1.QuestActive)
	}
	s.GetFlags().SetTrue(s.QuestFlag(q) + ".log")
}

// startTownQuest is TalkToTowner's usual start: logged, and only a quest that was still QUEST_INIT becomes active
// with _qvar1 1.
func (s *State) startTownQuest(q int) {
	if s.QuestActive(q) == d1.QuestInit {
		s.SetQuestActive(q, d1.QuestActive)
		s.SetQuestVar(q, 1, 1)
	}
	s.GetFlags().SetTrue(s.QuestFlag(q) + ".log")
}

// QuestVar is quests[q]._qvar1 (n 1) or _qvar2 (n 2).
func (s *State) QuestVar(q, n int) int {
	if v, ok := s.GetFlags().AllEntries()[fmt.Sprintf("%s.v%d", s.QuestFlag(q), n)]; ok {
		return v
	}
	if n == 1 {
		return s.mapGen.d1Quest(q).Var1
	}
	return s.mapGen.d1Quest(q).Var2
}

func (s *State) SetQuestVar(q, n, v int) {
	s.GetFlags().SetToValue(fmt.Sprintf("%s.v%d", s.QuestFlag(q), n), v)
}

// questHere ports QuestStatus: quest q is available and the player is on its (not set) level.
func (s *State) questHere(q int) bool { return s.questOn(q, s.GetMap()) }

func (s *State) questOn(q int, m *gridmap.GridMap[*Actor, *Item, Object]) bool {
	qu := s.mapGen.d1Quest(q)
	return qu.Active != d1.QuestNotAvail && m.GetName() == defs.LevelNameFromDepth(qu.Level)
}

// completeD1Quest marks quest q done and says so.
func (f *Factory) completeD1Quest(q int) {
	if f.State.QuestActive(q) == d1.QuestDone {
		return
	}
	f.State.SetQuestActive(q, d1.QuestDone)
	f.Msg(fmt.Sprintf("Quest completed: %s", f.State.QuestDefs()[q].Name))
}

// D1SetLevel is a quest set level: its map, its set level id and quest, and the named location of its entrance
// on the quest's dungeon level (D1's _qtx/_qty; Lazarus' red portal opens later). Every field is the mod's:
// the engine only knows that some quests bring a level of their own.
type D1SetLevel struct {
	Map, Entrance, Name string
	SL, Quest           int
}

// SetLevels are the quest set levels the mod declares, in quest order.
func (s *State) SetLevels() []D1SetLevel {
	var out []D1SetLevel
	for q, def := range s.QuestDefs() {
		if def.SetMap != "" {
			out = append(out, D1SetLevel{def.SetMap, def.SetEntrance, def.SetName, def.SetLevel, q})
		}
	}
	return out
}

// setLevelOf is the set level a map name belongs to, nil for an ordinary level.
func (s *State) setLevelOf(mapName string) *D1SetLevel {
	for _, sl := range s.SetLevels() {
		if sl.Map == mapName {
			return &sl
		}
	}
	return nil
}

// onSetLevelOf reports whether m is the set level of quest q.
func (s *State) onSetLevelOf(m *gridmap.GridMap[*Actor, *Item, Object], q int) bool {
	sl := s.setLevelOf(m.GetName())
	return sl != nil && sl.Quest == q
}

// questMap is the map name of quest q's set level, "" when it has none.
func (s *State) questMap(q int) string {
	if q >= len(s.QuestDefs()) {
		return "" // a quest this mod does not have has no level of its own
	}
	return s.QuestDefs()[q].SetMap
}

// d1QuestTopic is what npc says about quest q, if S_TalkEnter lists it: the quest is active and logged. What
// each townsperson says comes from the quest's gossip lines; which of them is offered is the logic below.
// Black Mushroom's topic moves from Adria to Pepin once she has the mushroom, and Pepin's ends once he has the
// brain -- the one case the table cannot say.
func (s *State) d1QuestTopic(npc VendorType, q int) string {
	if s.QuestActive(q) != d1.QuestActive || !s.QuestLog(q) {
		return ""
	}
	id := s.QuestDefs()[q].Gossip[string(npc)]
	if v := s.QuestVar(d1.QMushroom, 1); q == d1.QMushroom && v >= d1.QSMushGiven {
		switch npc {
		case VendorTypeWands:
			id = ""
		case VendorTypeGeneral:
			if v < d1.QSBrainGiven {
				id = "TEXT_MUSH3"
			}
		}
	}
	return id
}

// D1QuestLog ports the quest log: the active quests in it, each with its text.
func (s *State) D1QuestLog() []string {
	lines := []string{"Quest Log", ""}
	for q, def := range s.QuestDefs() {
		if s.QuestActive(q) == d1.QuestActive && s.QuestLog(q) {
			lines = append(lines, def.Name, "  "+s.d1Text(d1ClassText(s, def.LogText)), "")
		}
	}
	if len(lines) == 2 {
		lines = append(lines, "No active quests.")
	}
	return lines
}

func (s *State) d1Text(id string) string { return s.factory.D1().D.Texts[id] }

// d1ClassTextPrefix picks the class each book is read aloud by (OperateBookLever): Hellfire gives the Monk (H) and
// the Bard (B) their own texts, and its Barbarian reads the Warrior's.
var d1ClassTextPrefix = [d1.NumArchetypes]string{d1.Warrior: "", d1.Rogue: "R", d1.Sorcerer: "M", d1.Monk: "H", d1.Bard: "B", d1.Barbarian: ""}

// d1ClassText picks the hero's class version of a book's text (TEXT_BONER, TEXT_RBONER, TEXT_MBONER).
func d1ClassText(s *State, id string) string {
	if !strings.HasSuffix(id, "BONER") && !strings.HasSuffix(id, "BLOODY") && !strings.HasSuffix(id, "BLINDING") && !strings.HasSuffix(id, "BLOODWAR") {
		return id
	}
	return "TEXT_" + d1ClassTextPrefix[s.GetPlayer().Hero().Class] + strings.TrimPrefix(id, "TEXT_")
}

// say ports InitQTextMsg: a quest text in a text window, and its speech.
func (f *Factory) say(speaker, id string) {
	id = d1ClassText(f.State, id)
	f.playD1Sound(f.D1().D.TextSfx[id])
	text := f.State.d1Text(id)
	if ui := f.UI(); ui != nil && text != "" {
		ui.OpenTextWindow(util.ToLabelText(append([]string{speaker, ""}, strings.Split(text, "\n\n")...)))
	}
}

// d1HasItem ports PlrHasItem: the first item of base idx in the player's pack.
func d1HasItem(player *Actor, idx int) *Item {
	for _, it := range player.GetInventory().Items() {
		if it.d1 != nil && it.d1.BaseIdx == idx {
			return it
		}
	}
	return nil
}

// d1TakeItem removes an item of base idx from the player's pack (RemoveInvItem); false if there is none.
func d1TakeItem(player *Actor, idx int) bool {
	it := d1HasItem(player, idx)
	if it != nil {
		player.GetEquipment().UnEquip(it)
		player.GetInventory().Remove(it)
	}
	return it != nil
}

func (f *Factory) dropD1Item(it *d1.Item, pos geometry.Point) {
	f.State.AddItemToMapWithDistribution(f.NewD1Item(it), pos)
}

// Monster goals (monster_goal) the talking bosses use.
const (
	d1GoalNormal    = 1
	d1GoalAttack2   = 5
	d1GoalInquiring = 6
	d1GoalTalking   = 7
)

// initD1Talk sets a quest boss' talk state from the quest's progress: D1 keeps it with the monster, here it is
// rebuilt whenever its level is. Other monsters have nothing to say.
func (f *Factory) initD1Talk(a *Actor) {
	mon, s := a.d1Monster, f.State
	mon.TalkMsg = ""
	talk := func(msg string) { mon.TalkMsg, mon.Goal = msg, d1GoalInquiring }
	switch mon.AI {
	case "AI_GARBUD":
		if v := s.QuestVar(d1.QGarbud, 1); v < 4 {
			talk(fmt.Sprintf("TEXT_GARBUD%d", v+1))
		}
	case "AI_ZHAR":
		if v := s.QuestVar(d1.QZhar, 1); v < 2 {
			talk([2]string{"TEXT_ZHAR1", "TEXT_ZHAR2"}[v])
		}
	case "AI_SNOTSPIL":
		if v := s.QuestVar(d1.QLtBanner, 1); v < 2 {
			talk("TEXT_BANNER10")
		} else if v == 2 {
			talk("TEXT_BANNER11")
		}
	case "AI_LACHDAN":
		if talk("TEXT_VEIL9"); s.QuestLog(d1.QVeil) {
			talk("TEXT_VEIL10")
		}
	case "AI_WARLORD":
		if s.QuestVar(d1.QWarlord, 1) < 2 {
			talk("TEXT_WARLRD9")
		}
	case "AI_LAZURUS", "AI_LAZHELP":
		if s.QuestVar(d1.QBetrayer, 1) < 6 {
			talk("TEXT_VILE13")
		}
	}
}

// TalkToD1Monster ports TalktoMonster: the player's hand-ins to Snotspill and Lachdanan, then the talk.
func (f *Factory) TalkToD1Monster(a *Actor) {
	mon, s, player := a.d1Monster, f.State, f.State.GetPlayer()
	if mon.AI == "AI_SNOTSPIL" || mon.AI == "AI_LACHDAN" {
		if s.questHere(d1.QLtBanner) && s.QuestVar(d1.QLtBanner, 1) == 2 && d1TakeItem(player, d1.IdiBanner) {
			f.completeD1Quest(d1.QLtBanner)
			mon.TalkMsg, mon.Goal = "TEXT_BANNER12", d1GoalInquiring
		}
		if s.questHere(d1.QVeil) && strings.HasPrefix(mon.TalkMsg, "TEXT_VEIL") && d1TakeItem(player, d1.IdiGldnElix) {
			mon.TalkMsg, mon.Goal = "TEXT_VEIL11", d1GoalInquiring
		}
	}
	f.d1DoTalk(a)
}

// d1DoTalk ports M_DoTalk: the monster says its line, and what that sets in motion.
func (f *Factory) d1DoTalk(a *Actor) {
	mon, s, pos := a.d1Monster, f.State, a.MapPosition()
	mon.Goal = d1GoalTalking
	f.say(a.Name(), mon.TalkMsg)
	dlvl := d1DungeonLevel(s)
	switch mon.AI {
	case "AI_GARBUD":
		if mon.TalkMsg == "TEXT_GARBUD1" {
			s.SetQuestActive(d1.QGarbud, d1.QuestActive)
		}
		s.GetFlags().SetTrue(s.QuestFlag(d1.QGarbud) + ".log")
		if mon.TalkMsg == "TEXT_GARBUD2" && s.QuestVar(d1.QGarbud, 2) == 0 { // MFLAG_QUEST_COMPLETE
			if it := f.Generator.MonsterDrop(mon.Data, mon.Level, true, dlvl, false); it != nil {
				f.dropD1Item(it, pos)
			}
			s.SetQuestVar(d1.QGarbud, 2, 1)
		}
	case "AI_ZHAR":
		if mon.TalkMsg == "TEXT_ZHAR1" && s.QuestVar(d1.QZhar, 2) == 0 {
			s.startQuest(d1.QZhar)
			f.dropD1Item(f.Generator.TypeItem(dlvl, d1.ITypeMisc, "IMISC_BOOK", false), pos)
			s.SetQuestVar(d1.QZhar, 2, 1)
		}
	case "AI_SNOTSPIL":
		if mon.TalkMsg == "TEXT_BANNER10" && s.QuestVar(d1.QLtBanner, 1) < 2 {
			sp := f.State.mapGen.setPiece(s.GetMap())
			f.State.mapGen.changeD1Map(s.GetMap(), sp[0], sp[1], sp[2]>>1+sp[0]+2, sp[3]>>1+sp[1]-2)
			s.SetQuestVar(d1.QLtBanner, 1, 2)
			s.SetQuestActive(d1.QLtBanner, d1.QuestActive)
		}
	case "AI_LACHDAN":
		if mon.TalkMsg == "TEXT_VEIL9" {
			s.startQuest(d1.QVeil)
		}
		if mon.TalkMsg == "TEXT_VEIL11" && s.QuestVar(d1.QVeil, 2) == 0 {
			f.dropD1Item(f.Generator.SpawnUnique(d1.UItemSteelVeil, dlvl), pos)
			s.SetQuestVar(d1.QVeil, 2, 1)
		}
	case "AI_WARLORD":
		s.SetQuestVar(d1.QWarlord, 1, 2)
	}
}

// D1TalkerTurn ports the talk states of MAI_Garbud, MAI_Zhar, MAI_SnotSpil, MAI_Lachdanan, MAI_Warlord,
// MAI_Lazurus and MAI_Lazhelp for one turn; visible is D1's BFLAG_VISIBLE (the player sees the monster). It
// reports whether the monster fights this turn.
func (f *Factory) D1TalkerTurn(a *Actor, visible bool) bool {
	mon, s := a.d1Monster, f.State
	quiet := func() { mon.Squelch, mon.TalkMsg, mon.Goal = 255, "", d1GoalNormal }
	talking := mon.Goal == d1GoalTalking
	switch mon.AI {
	case "AI_GARBUD":
		if v := s.QuestVar(d1.QGarbud, 1); v < 3 && !visible && talking {
			s.SetQuestVar(d1.QGarbud, 1, v+1)
			mon.TalkMsg, mon.Goal = fmt.Sprintf("TEXT_GARBUD%d", v+2), d1GoalInquiring
		}
		if visible && mon.TalkMsg == "TEXT_GARBUD4" && talking {
			s.SetQuestVar(d1.QGarbud, 1, 4)
			quiet()
		}
	case "AI_ZHAR":
		if mon.TalkMsg == "TEXT_ZHAR1" && !visible && talking {
			s.SetQuestVar(d1.QZhar, 1, 1)
			mon.TalkMsg, mon.Goal = "TEXT_ZHAR2", d1GoalInquiring
		}
		if visible && mon.TalkMsg == "TEXT_ZHAR2" && talking {
			s.SetQuestVar(d1.QZhar, 1, 2)
			quiet()
		}
	case "AI_SNOTSPIL":
		if mon.TalkMsg == "TEXT_BANNER10" && !visible && talking {
			mon.TalkMsg, mon.Goal = "TEXT_BANNER11", d1GoalInquiring
		}
		if mon.TalkMsg == "TEXT_BANNER11" && s.QuestVar(d1.QLtBanner, 1) == 3 {
			mon.TalkMsg, mon.Goal = "", d1GoalNormal
		}
		if visible && mon.TalkMsg == "TEXT_BANNER12" && talking {
			s.SetQuestVar(d1.QLtBanner, 1, 3)
			UpdateD1Quests(s, s.GetMap())
			quiet()
		}
		return visible && s.QuestVar(d1.QLtBanner, 1) == 3 && (mon.Goal == d1GoalNormal || mon.Goal == d1GoalAttack2)
	case "AI_LACHDAN":
		if mon.TalkMsg == "TEXT_VEIL9" && !visible && talking {
			mon.TalkMsg, mon.Goal = "TEXT_VEIL10", d1GoalInquiring
		}
		if visible && mon.TalkMsg == "TEXT_VEIL11" && talking {
			mon.TalkMsg = ""
			f.completeD1Quest(d1.QVeil)
			f.ActionBroker.Execute(NewChangeResourceAction(AbstractCause("Lachdanan's rest"), a, stats.Health, -1e6)) // M_StartKill(i, -1)
		}
		return false
	case "AI_WARLORD":
		if visible && mon.TalkMsg == "TEXT_WARLRD9" && mon.Goal == d1GoalInquiring {
			f.d1DoTalk(a)
		} else if visible && mon.TalkMsg == "TEXT_WARLRD9" && talking {
			quiet()
		}
	case "AI_LAZURUS": // he speaks once the hero stands on the circle in front of him, then opens his helpers' rooms
		if visible && mon.TalkMsg == "TEXT_VILE13" && mon.Goal == d1GoalInquiring && s.GetPlayer().MapPosition() == pieceCell(d1VileAltar) {
			s.SetQuestVar(d1.QBetrayer, 1, 5)
			f.d1DoTalk(a)
		} else if visible && mon.TalkMsg == "TEXT_VILE13" && talking {
			s.SetQuestVar(d1.QBetrayer, 1, 6)
			UpdateD1Quests(s, s.GetMap())
			quiet()
		}
	case "AI_LAZHELP":
		if visible && s.QuestVar(d1.QBetrayer, 1) <= 5 {
			mon.Goal = d1GoalInquiring
		} else if visible {
			mon.TalkMsg, mon.Goal = "", d1GoalNormal
		}
	}
	return mon.Goal != d1GoalInquiring && mon.Goal != d1GoalTalking
}

var (
	d1StairsDown = gridmap.Tile{DefinedIcon: defs.IconStairsDown, DefinedDescription: "stairs leading down", IsWalkable: true, IsTransparent: true, Special: gridmap.SpecialTileStairsDown}
	d1RedPortal  = gridmap.Tile{DefinedIcon: 72, DefinedDescription: "a red portal", IsWalkable: true, IsTransparent: true}
)

// UpdateD1Quests brings a level up to date with the quests. It is idempotent and runs on arrival and whenever
// something changes: the red portal on 15 once Cain has the staff (CheckQuests) and the stairs down once Lazarus
// is dead (InitVPTriggers), the portal back from his lair, the map changes that levers, books, the Pedestal of
// Blood and Snotspill made (ObjChangeMap), and the quest items D1 spawns on arrival (ResyncQuests).
func UpdateD1Quests(s *State, m *gridmap.GridMap[*Actor, *Item, Object]) {
	if m == nil || defs.IsTownLevel(m.GetName()) {
		return
	}
	clear(d1GossipSeeds) // the town's towners are rolled anew on the way back
	f := s.factory
	f.d1DungMsgs(m)
	open := func(loc string, tile gridmap.Tile, to gridmap.Transition) {
		p := m.GetNamedLocation(loc)
		if _, done := m.GetTransitionAt(p); done || m.GetNamedLocationByPos(p) != loc {
			return
		}
		m.SetTile(p, tile)
		m.AddTransitionAt(p, to)
	}
	betrayer := s.QuestActive(d1.QBetrayer)
	switch m.GetName() {
	case defs.LevelNameFromDepth(15):
		if s.QuestVar(d1.QBetrayer, 1) >= 2 && betrayer != d1.QuestInit {
			if betrayer == d1.QuestActive && s.QuestVar(d1.QBetrayer, 1) == 2 {
				s.SetQuestVar(d1.QBetrayer, 1, 3)
			}
			open("red_portal", d1RedPortal, gridmap.Transition{TargetMap: s.questMap(d1.QBetrayer), TargetLocation: "ladder_up"})
		}
		if betrayer == d1.QuestDone {
			open("ladder_down", d1StairsDown, gridmap.Transition{TargetMap: defs.LevelNameFromDepth(16), TargetLocation: "ladder_up"})
		}
		if q := d1.QVeil; s.QuestActive(q) == d1.QuestActive && s.QuestVar(q, 1) == 0 {
			s.SetQuestVar(q, 1, 1)
			f.spawnD1QuestItem(m, d1.IdiGldnElix)
		}
	case s.questMap(d1.QBetrayer):
		if betrayer == d1.QuestDone { // M_StartKill's MIS_RPORTAL at (35, 32)
			open("exit", d1RedPortal, gridmap.Transition{TargetMap: defs.LevelNameFromDepth(15), TargetLocation: "red_portal"})
		}
	case defs.LevelNameFromDepth(9):
		if q := d1.QMushroom; s.QuestActive(q) == d1.QuestInit && s.QuestVar(q, 1) == d1.QSInit {
			s.SetQuestVar(q, 1, d1.QSTomeSpawned)
			f.spawnD1QuestItem(m, d1.IdiFungalTm)
		}
	}
	replayD1MapChanges(s, m)
}

// replayD1MapChanges redoes the map changes of UpdateD1Quests. A loaded level runs it before its saved monsters
// are restored, so none of them (nor the hero) is pushed out of a room a lever or quest had opened.
func replayD1MapChanges(s *State, m *gridmap.GridMap[*Actor, *Item, Object]) {
	gen := s.mapGen
	sp := gen.setPiece(m)
	if s.questOn(d1.QLtBanner, m) {
		if v := s.QuestVar(d1.QLtBanner, 1); v == 2 {
			gen.changeD1Map(m, sp[0], sp[1], sp[2]>>1+sp[0]+2, sp[3]>>1+sp[1]-2)
		} else if v == 3 {
			gen.changeD1Map(m, sp[0], sp[1], sp[0]+sp[2]+1, sp[1]+sp[3]+1)
		}
	}
	if s.questOn(d1.QBlood, m) {
		stages := [3][4]int{{sp[0], sp[1] + 3, sp[0] + 2, sp[1] + 7}, {sp[0] + 6, sp[1] + 3, sp[0] + sp[2], sp[1] + 7}, {sp[0], sp[1], sp[0] + sp[2], sp[1] + sp[3]}}
		for _, r := range stages[:min(s.QuestVar(d1.QBlood, 2), 3)] {
			gen.changeD1Map(m, r[0], r[1], r[2], r[3])
		}
		if lvl := gen.d1Levels[m.GetName()]; lvl != nil && s.QuestVar(d1.QBlood, 2) >= 3 { // OperatePedistal: Blood2.DUN's torches light the hall
			for _, o := range s.factory.D1().D.DunObjects("l2/blood2", 2*sp[0], 2*sp[1]) {
				d1PlaceObject(s.factory, m, lvl, o, toCell(o.X, o.Y))
			}
			delete(s.glowCache, m)
		}
	}
	if s.onSetLevelOf(m, d1.QBetrayer) { // ResyncQuests: the way to Lazarus, then to Red Vex and Black Jade
		if s.QuestVar(d1.QBetrayer, 1) >= 4 || s.QuestActive(d1.QBetrayer) == d1.QuestDone {
			gen.changeD1Map(m, 1, 11, 20, 18)
		}
		if s.QuestVar(d1.QBetrayer, 1) >= 6 || s.QuestActive(d1.QBetrayer) == d1.QuestDone {
			gen.changeD1Map(m, 1, 18, 20, 24)
		}
	}
	for _, obj := range m.Objects() {
		o, ok := obj.(*D1Object)
		if !ok || o.O.Active {
			continue
		}
		switch o.O.Kind {
		case d1.ObjLEVER, d1.ObjBLINDBOOK, d1.ObjSTEELTOME, d1.ObjBOOK2R:
			gen.changeD1Map(m, o.O.Var1, o.O.Var2, o.O.Var3, o.O.Var4)
		case d1.ObjBOOK2L:
			if s.onSetLevelOf(m, d1.QBetrayer) {
				gen.changeD1Map(m, o.O.Var1, o.O.Var2, o.O.Var3, o.O.Var4)
			}
		}
	}
	for _, obj := range m.Objects() {
		o, ok := obj.(*D1Object)
		if !ok {
			continue
		}
		if o.O.Kind == d1.ObjSWITCHSKL && d1ObjectGroupUsed(m, isD1Skull, o.O.Var8) || isD1Crux(o.O.Kind) && d1ObjectGroupUsed(m, isD1Crux, o.O.Var8) {
			gen.changeD1Map(m, o.O.Var1, o.O.Var2, o.O.Var3, o.O.Var4)
		}
	}
}

// Lazarus' lair (Vile2.DUN), in piece coordinates: the magic circles in front of the two Books of Vileness and where
// they teleport the hero, the central circle and the one in front of Lazarus it leads to, and the return portal.
var (
	d1VileCircles = map[geometry.Point]geometry.Point{{X: 26, Y: 46}: {X: 27, Y: 29}, {X: 45, Y: 47}: {X: 43, Y: 29}}
	d1VileCenter  = geometry.Point{X: 35, Y: 36}
	d1VileAltar   = geometry.Point{X: 35, Y: 46}
	D1VilePortal  = geometry.Point{X: 35, Y: 32}
)

func pieceCell(p geometry.Point) geometry.Point { return toCell(p.X, p.Y) }

// operateD1VileBook ports OperateBook in Lazarus' lair: read while standing on the magic circle in front of it, a
// Book of Vileness teleports the hero into the sealed room behind it and opens that room.
func (f *Factory) operateD1VileBook(o *D1Object, player *Actor) {
	to, onCircle := geometry.Point{}, false
	for circle, dest := range d1VileCircles {
		if player.MapPosition() == pieceCell(circle) {
			to, onCircle = dest, true
		}
	}
	if !o.O.Active || !onCircle {
		return
	}
	o.O.Active = false
	f.d1VileTeleport(player, pieceCell(to))
	UpdateD1Quests(f.State, f.State.GetMap())
}

// d1VileTeleport ports MIS_RNDTELEPORT onto a lair circle: the hero lands on it, or next to it when taken
// (GetVileMissPos).
func (f *Factory) d1VileTeleport(player *Actor, p geometry.Point) {
	m := f.State.GetMap()
	if !m.CanPlaceActorHere(p) {
		if near := freeCellsNear(m, p, 1, m.CanPlaceActorHere); len(near) > 0 {
			p = near[0]
		}
	}
	f.d1MovePlayer(player, p, "Teleport")
}

// d1MagicCircle ports Obj_Circle: once both Books of Vileness are read, the central circle opens the way to
// Lazarus and teleports the hero in front of him.
func (f *Factory) d1MagicCircle(o *D1Object, player *Actor) {
	m, s := f.State.GetMap(), f.State
	if !s.onSetLevelOf(m, d1.QBetrayer) || o.pos != pieceCell(d1VileCenter) || d1VileBooksRead(m) < 2 {
		return
	}
	// 1.09 sets _qvar1 4 on every step onto the circle, even after Lazarus spoke (its BUGFIX note); not done again here
	if s.QuestActive(d1.QBetrayer) == d1.QuestActive && s.QuestVar(d1.QBetrayer, 1) < 4 {
		s.SetQuestVar(d1.QBetrayer, 1, 4)
	}
	s.mapGen.changeD1Map(m, o.O.Var1, o.O.Var2, o.O.Var3, o.O.Var4)
	UpdateD1Quests(s, m)
	f.d1VileTeleport(player, pieceCell(d1VileAltar))
}

// d1VileBooksRead counts the lair's used Books of Vileness (the central circle's _oVar5 minus 1).
func d1VileBooksRead(m *gridmap.GridMap[*Actor, *Item, Object]) (n int) {
	for _, obj := range m.Objects() {
		if o, ok := obj.(*D1Object); ok && o.O.Kind == d1.ObjBOOK2L && !o.O.Active {
			n++
		}
	}
	return
}

func isD1Skull(kind int) bool { return kind == d1.ObjSWITCHSKL }

// d1ObjectGroupUsed ports OperateLever's and BreakCrux's check: every object of the kind in group v is used.
func d1ObjectGroupUsed(m *gridmap.GridMap[*Actor, *Item, Object], kind func(int) bool, v int) bool {
	for _, obj := range m.Objects() {
		if o, ok := obj.(*D1Object); ok && kind(o.O.Kind) && o.O.Var8 == v && o.O.Active {
			return false
		}
	}
	return true
}

// spawnD1QuestItem ports SpawnQuestItem at a random spot (randarea 5).
func (f *Factory) spawnD1QuestItem(m *gridmap.GridMap[*Actor, *Item, Object], idx int) {
	if p, ok := randomQuestSpot(m, nil); ok {
		m.AddItem(f.NewD1Item(f.Generator.PlainItem(idx)), p)
	}
}

// randomQuestSpot picks a random open cell (all 8 neighbours passable) that is reachable from the stairs up and
// away from them.
func randomQuestSpot(m *gridmap.GridMap[*Actor, *Item, Object], keep func(geometry.Point) bool) (geometry.Point, bool) {
	up := m.GetNamedLocation("ladder_up")
	cells := freeCellsNear(m, up, d1.LevelSize*d1.LevelSize, func(p geometry.Point) bool {
		_, isTransition := m.GetTransitionAt(p)
		return m.CanPlaceItemHere(p) && !m.IsObjectAt(p) && !isTransition &&
			geometry.DistanceChebyshev(p, up) > 10 && len(m.GetFilteredNeighbors(p, m.IsWalkable)) == 8 && (keep == nil || keep(p))
	})
	if len(cells) == 0 {
		return geometry.Point{}, false
	}
	return cells[d1Rng.Intn(len(cells))], true
}

// d1QuestKill ports Poisoned Water's check of CheckQuests -- the set level is clean once its last monster dies,
// which is a whole-map question no condition can ask -- and then hands the death to the mod's on_kill triggers,
// which carry CheckQuestKill and M_DiabloDeath's quest.
func d1QuestKill(ctx ActionContext, mon *D1Monster) {
	state := ctx.GameState
	f := state.factory
	clean := mapClearedBy(state.GetMap(), mon)
	if state.onSetLevelOf(state.GetMap(), d1.QPWater) && state.QuestActive(d1.QPWater) != d1.QuestInit {
		if clean && state.QuestActive(d1.QPWater) != d1.QuestDone {
			f.completeD1Quest(d1.QPWater)
			f.PlayD1Sfx("IS_QUESTDN", state.GetPlayer().MapPosition())
			state.mapGen.cleanD1Water(state.GetMap())
		}
	}
	f.fireQuestTriggers(EventKill, mon.Data.ID, "")
	if mon.Unique != nil {
		f.fireQuestTriggers(EventKill, mon.Unique.Name, "")
	}
	if clean {
		f.fireQuestTriggers(EventLevelCleared, state.GetMap().GetName(), "")
	}
}

// mapClearedBy is whether mon was the last monster standing on m (golems do not count).
func mapClearedBy(m *gridmap.GridMap[*Actor, *Item, Object], mon *D1Monster) bool {
	for _, a := range m.Actors() {
		if a.d1Monster != nil && a.d1Monster != mon && !a.d1Monster.Golem && a.IsAlive() {
			return false
		}
	}
	return true
}

// d1EndTextWindow is the end_text outcome: Diablo's death screen.
func (f *Factory) d1EndTextWindow() {
	PlayUICue("you_win")
	if ui := f.UI(); ui != nil {
		ui.OpenTextWindow(util.ToLabelText(append(d1EndText(), "",
			"Yet what dwells below has taken his measure. Every level of the dungeon now breeds",
			"stronger things, and richer spoils. Gillian collects for the town, should you wish",
			"to see them walk again.")))
	}
	f.Msg("The dungeon stirs: monsters and loot below are stronger now.")
}

func d1EndText() []string {
	r := files.MustOpenReader(path.Join("assets", "endMessage.txt"))
	defer r.Close()
	data, _ := io.ReadAll(r)
	return strings.Split(string(data), "\n")
}

// d1QuestItemPickedUp ports CheckQuestItem: the mod's on_item_pickup triggers, named by the item picked up.
func (f *Factory) d1QuestItemPickedUp(it *d1.Item) {
	items := f.D1().D.Items
	if it.BaseIdx < 0 || it.BaseIdx >= len(items) {
		return
	}
	f.fireQuestTriggers(EventItemPickup, items[it.BaseIdx].InternalName, "")
}

// d1QuestDrop ports SpawnItem's Black Mushroom branch: once Adria has the mushroom, the next ordinary monster
// killed drops a brain instead of its loot.
func (f *Factory) d1QuestDrop(mon *D1Monster) *d1.Item {
	if q := d1.QMushroom; mon.Unique == nil && f.State.QuestActive(q) == d1.QuestActive && f.State.QuestVar(q, 1) == d1.QSMushGiven {
		f.State.SetQuestVar(q, 1, d1.QSBrainSpawned)
		return f.Generator.PlainItem(d1.IdiBrain)
	}
	return nil
}

// operateD1QuestObject ports OperateObject for the quest objects; false for any other object.
func (f *Factory) operateD1QuestObject(o *D1Object, player *Actor) bool {
	obj, s, gen, m := o.O, f.State, f.Generator, f.State.GetMap()
	sp := s.mapGen.setPiece(m)
	cell := func(px, py int) geometry.Point { return geometry.Point{X: px - d1.CellOrigin, Y: py - d1.CellOrigin} }
	switch obj.Kind {
	case d1.ObjMUSHPATCH: // OperateMushPatch
		if q := d1.QMushroom; s.QuestActive(q) != d1.QuestActive || s.QuestVar(q, 1) < d1.QSTomeGiven {
			f.PlayD1HeroSound(13)
			f.Msg("I can't open this yet.")
		} else if obj.Active {
			obj.Active = false
			f.PlayD1Sfx("IS_CHEST", o.pos)
			f.dropD1Item(gen.PlainItem(d1.IdiMushroom), o.pos)
			s.SetQuestVar(q, 1, d1.QSMushSpawned)
		}
	case d1.ObjSIGNCHEST: // OperateInnSignChest
		if s.QuestVar(d1.QLtBanner, 1) != 2 {
			f.PlayD1HeroSound(24)
			f.Msg("I can't open this yet.")
		} else if obj.Active {
			obj.Active = false
			f.PlayD1Sfx("IS_CHEST", o.pos)
			f.dropD1Item(gen.PlainItem(d1.IdiBanner), o.pos)
		}
	case d1.ObjBLINDBOOK, d1.ObjBLOODBOOK, d1.ObjSTEELTOME: // OperateBookLever
		q := map[int]int{d1.ObjBLINDBOOK: d1.QBlind, d1.ObjBLOODBOOK: d1.QBlood, d1.ObjSTEELTOME: d1.QWarlord}[obj.Kind]
		if obj.Active {
			obj.Active = false
			if s.QuestVar(q, 1) == 0 {
				s.startQuest(q)
				s.SetQuestVar(q, 1, 1)
				if q == d1.QBlood {
					for _, c := range [3][2]int{{19, 26}, {31, 26}, {25, 33}} {
						m.AddItem(f.NewD1Item(gen.PlainItem(d1.IdiBldStone)), cell(2*sp[0]+c[0], 2*sp[1]+c[1]))
					}
				}
			}
			UpdateD1Quests(s, m)
			if q == d1.QBlind {
				f.dropD1Item(gen.SpawnUnique(d1.UItemOptAmulet, d1DungeonLevel(s)), cell(2*sp[0]+16+5, 2*sp[1]+16+5))
			}
		}
		f.say(o.Name(), s.QuestDefs()[q].LogText)
	case d1.ObjPEDISTAL: // OperatePedistal
		q := d1.QBlood
		stones := s.QuestVar(q, 2)
		switch {
		case stones >= 3:
			f.Msg("The pedestal is full. Its three blood stones are set.")
		case !d1TakeItem(player, d1.IdiBldStone):
			f.Msg(fmt.Sprintf("Three hollows are cut into the pedestal; %d still wait for a blood stone.", 3-stones))
		default:
			s.SetQuestVar(q, 2, stones+1)
			f.PlayD1Sfx(map[bool]string{false: "LS_PUDDLE", true: "LS_BLODSTAR"}[stones+1 == 3], o.pos)
			UpdateD1Quests(s, m)
			if stones+1 < 3 {
				f.Msg(fmt.Sprintf("You set a blood stone into the pedestal. %d to go.", 3-(stones+1)))
				break
			}
			obj.Active = false
			f.Msg("The last blood stone sinks home. The pedestal drinks, and the hall below lights up.")
			f.dropD1Item(gen.SpawnUnique(d1.UItemArmOfVal, d1DungeonLevel(s)), cell(2*sp[0]+25, 2*sp[1]+19))
		}
	case d1.ObjBOOK2R: // OperateSChambBk
		if q := d1.QSchamb; obj.Active {
			obj.Active = false
			if s.QuestActive(q) == d1.QuestInit {
				s.startQuest(q)
			}
			UpdateD1Quests(s, m)
		}
		f.say(o.Name(), s.QuestDefs()[d1.QSchamb].LogText)
	case d1.ObjBOOK2L: // OperateBook
		if s.onSetLevelOf(m, d1.QBetrayer) {
			f.operateD1VileBook(o, player)
		} else if obj.Active && s.onSetLevelOf(m, d1.QSchamb) {
			obj.Active = false
			h := player.hero
			h.SpellLevels[d1.SplGuardian] = min(h.SpellLevels[d1.SplGuardian]+1, 15)
			f.RefreshD1Spells(player)
			f.completeD1Quest(d1.QSchamb)
			f.PlayD1Sfx("IS_QUESTDN", o.pos)
			f.Msg("Arcane knowledge gained!")
			f.ActionBroker.Execute(NewSpellAction(player, d1.SplGuardian, h.SpellLevels[d1.SplGuardian], o.pos.Add(geometry.Point{X: -2, Y: -4})))
		}
	case d1.ObjBOOKCASEL, d1.ObjBOOKCASER: // OperateBookCase: Zhar, already aware of the hero, has had enough
		if obj.Active && s.questHere(d1.QZhar) {
			for _, a := range m.Actors() {
				if mon := a.d1Monster; mon != nil && mon.AI == "AI_ZHAR" && a.IsAlive() && mon.Squelch == 255 {
					mon.TalkMsg = "TEXT_ZHAR2"
					s.SetQuestVar(d1.QZhar, 1, 1)
					f.d1DoTalk(a)
				}
			}
		}
		return false
	default:
		return false
	}
	return true
}

// D1Level is the generated D1 level behind a built map, nil for any other.
func (s *State) D1Level(m *gridmap.GridMap[*Actor, *Item, Object]) *d1.Level {
	return s.mapGen.d1Levels[m.GetName()]
}

// D1MonsterGreeting ports ProcessMonsters' first-sight lines: the Butcher's "fresh meat".
func (f *Factory) D1MonsterGreeting(a *Actor) {
	if a.d1Monster.Data.ID == "MT_CLEAVER" {
		f.playD1Sound("USFX_CLEAVER")
	}
}

// d1DungMsgs ports PlayDungMsgs: the mod's on_enter_level triggers, named by the depth reached. Set levels and
// town have none. The "each once" is the triggers' own business -- they set and test a flag.
func (f *Factory) d1DungMsgs(m *gridmap.GridMap[*Actor, *Item, Object]) {
	dlvl := LevelFromName(m.GetName())
	if f.State.setLevelOf(m.GetName()) != nil || dlvl == 0 {
		return
	}
	f.fireQuestTriggers(EventEnterLevel, fmt.Sprint(dlvl), "")
}
