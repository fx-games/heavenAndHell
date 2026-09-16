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
// active, ".log" once in the quest log, ".v1"/".v2" its _qvar1/_qvar2.
var d1QuestKeys = [d1.NumQuests]string{"rock", "mushroom", "garbud", "zhar", "veil", "diablo", "butcher", "banner",
	"blind", "blood", "anvil", "warlord", "leoric", "pwater", "schamb", "lazarus"}

// QuestFlag is the flag set once quest q is done.
func QuestFlag(q int) string { return "quest:" + d1QuestKeys[q] }

// QuestActive is quests[q]._qactive.
func (s *State) QuestActive(q int) int {
	switch f := s.GetFlags(); {
	case f.IsSet(QuestFlag(q)):
		return d1.QuestDone
	case f.IsSet(QuestFlag(q) + ".active"):
		return d1.QuestActive
	}
	return s.D1Quests()[q].Active
}

func (s *State) SetQuestActive(q, v int) {
	switch v {
	case d1.QuestDone:
		s.GetFlags().SetTrue(QuestFlag(q))
	case d1.QuestActive:
		s.GetFlags().SetTrue(QuestFlag(q) + ".active")
	}
}

func (s *State) QuestLog(q int) bool { return s.GetFlags().IsSet(QuestFlag(q) + ".log") }

// startQuest makes quest q active and puts it in the quest log.
func (s *State) startQuest(q int) {
	if s.QuestActive(q) != d1.QuestDone {
		s.SetQuestActive(q, d1.QuestActive)
	}
	s.GetFlags().SetTrue(QuestFlag(q) + ".log")
}

// startTownQuest is TalkToTowner's usual start: logged, and only a quest that was still QUEST_INIT becomes active
// with _qvar1 1.
func (s *State) startTownQuest(q int) {
	if s.QuestActive(q) == d1.QuestInit {
		s.SetQuestActive(q, d1.QuestActive)
		s.SetQuestVar(q, 1, 1)
	}
	s.GetFlags().SetTrue(QuestFlag(q) + ".log")
}

// QuestVar is quests[q]._qvar1 (n 1) or _qvar2 (n 2).
func (s *State) QuestVar(q, n int) int {
	if v, ok := s.GetFlags().AllEntries()[fmt.Sprintf("%s.v%d", QuestFlag(q), n)]; ok {
		return v
	}
	if n == 1 {
		return s.D1Quests()[q].Var1
	}
	return s.D1Quests()[q].Var2
}

func (s *State) SetQuestVar(q, n, v int) {
	s.GetFlags().SetToValue(fmt.Sprintf("%s.v%d", QuestFlag(q), n), v)
}

// questHere ports QuestStatus: quest q is available and the player is on its (not set) level.
func (s *State) questHere(q int) bool { return s.questOn(q, s.GetMap()) }

func (s *State) questOn(q int, m *gridmap.GridMap[*Actor, *Item, Object]) bool {
	qu := s.D1Quests()[q]
	return qu.Active != d1.QuestNotAvail && m.GetName() == defs.LevelNameFromDepth(qu.Level)
}

// completeD1Quest marks quest q done and says so.
func (f *Factory) completeD1Quest(q int) {
	if f.State.QuestActive(q) == d1.QuestDone {
		return
	}
	f.State.SetQuestActive(q, d1.QuestDone)
	f.Msg(fmt.Sprintf("Quest completed: %s", d1.QuestNames[q]))
}

// D1SetLevel is a quest set level: its map, its D1 set level and quest, and the named location of its entrance
// on the quest's dungeon level (D1's _qtx/_qty; Lazarus' red portal opens later).
type D1SetLevel struct {
	Map, Entrance, Name string
	SL, Quest           int
}

const (
	D1LeoricLevel  = "!gen_leoric_3"
	D1LazarusLevel = "!gen_lazarus_15"
)

var D1SetLevels = []D1SetLevel{
	{D1LeoricLevel, "leoric", "King Leoric's Tomb", d1.SLSkelKing, d1.QSkelKing},
	{"!gen_bonechamb_6", "schamb", "The Chamber of Bone", d1.SLBoneChamb, d1.QSchamb},
	{"!gen_pwater_2", "pwater", "A Dark Passage", d1.SLPoisonWater, d1.QPWater},
	{D1LazarusLevel, "red_portal", "Lazarus' Lair", d1.SLVileBetrayer, d1.QBetrayer},
}

func d1SetLevelOf(mapName string) *D1SetLevel {
	for i := range D1SetLevels {
		if D1SetLevels[i].Map == mapName {
			return &D1SetLevels[i]
		}
	}
	return nil
}

// d1TalkList ports towners.cpp's Qtalklist: what each townsperson says about a quest ("" for nothing).
var d1TalkList = map[VendorType][d1.NumQuests]string{
	VendorTypeWeapons: {"INFRA6", "MUSH6", "", "", "VEIL5", "", "BUTCH5", "BANNER6", "BLIND5", "BLOOD5", "ANVIL6", "WARLRD5", "KING7", "POISON7", "BONE5", "VILE9"},
	VendorTypeGeneral: {"INFRA3", "", "", "", "VEIL3", "", "BUTCH3", "BANNER4", "BLIND3", "BLOOD3", "ANVIL3", "WARLRD3", "KING5", "POISON4", "BONE3", "VILE7"},
	VendorTypeOgden:   {"INFRA2", "MUSH2", "", "", "VEIL2", "", "BUTCH2", "", "BLIND2", "BLOOD2", "ANVIL2", "WARLRD2", "KING3", "POISON2", "BONE2", "VILE4"},
	VendorTypeCain:    {"INFRA1", "MUSH1", "", "", "VEIL1", "VILE3", "BUTCH1", "BANNER1", "BLIND1", "BLOOD1", "ANVIL1", "WARLRD1", "KING1", "POISON1", "BONE1", "VILE2"},
	VendorTypeDrunk:   {"INFRA8", "MUSH7", "", "", "VEIL6", "", "BUTCH6", "BANNER7", "BLIND6", "BLOOD6", "ANVIL8", "WARLRD6", "KING8", "POISON8", "BONE6", "VILE10"},
	VendorTypeWands:   {"INFRA9", "MUSH9", "", "", "VEIL7", "", "BUTCH7", "BANNER8", "BLIND7", "BLOOD7", "ANVIL9", "WARLRD7", "KING9", "POISON9", "BONE7", "VILE11"},
	VendorTypeBarmaid: {"INFRA4", "MUSH5", "", "", "VEIL4", "", "BUTCH4", "BANNER5", "BLIND4", "BLOOD4", "ANVIL4", "WARLRD4", "KING6", "POISON6", "BONE4", "VILE8"},
	VendorTypeWirt:    {"INFRA10", "MUSH13", "", "", "VEIL8", "", "BUTCH8", "BANNER9", "BLIND8", "BLOOD8", "ANVIL10", "WARLRD8", "KING10", "POISON10", "BONE8", "VILE12"},
}

// d1QuestTopic is what npc says about quest q, if S_TalkEnter lists it: the quest is active and logged. Black
// Mushroom's topic moves from Adria to Pepin once she has the mushroom, and Pepin's ends once he has the brain.
func (s *State) d1QuestTopic(npc VendorType, q int) string {
	if s.QuestActive(q) != d1.QuestActive || !s.QuestLog(q) {
		return ""
	}
	id := d1TalkList[npc][q]
	if v := s.QuestVar(d1.QMushroom, 1); q == d1.QMushroom && v >= d1.QSMushGiven {
		switch npc {
		case VendorTypeWands:
			id = ""
		case VendorTypeGeneral:
			if v < d1.QSBrainGiven {
				id = "MUSH3"
			}
		}
	}
	if id == "" {
		return ""
	}
	return "TEXT_" + id
}

// D1QuestLog ports the quest log: the active quests in it, each with its text.
func (s *State) D1QuestLog() []string {
	lines := []string{"Quest Log", ""}
	for q := range d1QuestKeys {
		if s.QuestActive(q) == d1.QuestActive && s.QuestLog(q) {
			lines = append(lines, d1.QuestNames[q], "  "+s.d1Text(d1ClassText(s, d1.QuestMsgs[q])), "")
		}
	}
	if len(lines) == 2 {
		lines = append(lines, "No active quests.")
	}
	return lines
}

func (s *State) d1Text(id string) string { return s.factory.D1().D.Texts[id] }

// d1ClassText picks the hero's class version of a book's text (TEXT_BONER, TEXT_RBONER, TEXT_MBONER).
func d1ClassText(s *State, id string) string {
	if !strings.HasSuffix(id, "BONER") && !strings.HasSuffix(id, "BLOODY") && !strings.HasSuffix(id, "BLINDING") && !strings.HasSuffix(id, "BLOODWAR") {
		return id
	}
	return "TEXT_" + [3]string{"", "R", "M"}[s.GetPlayer().Hero().Class] + strings.TrimPrefix(id, "TEXT_")
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

// d1TownerQuests ports TalkToTowner's quest steps; true if the townsperson said something, which D1 shows instead
// of the store.
func (f *Factory) d1TownerQuests(npc *Actor) bool {
	s, player, gen, pos := f.State, f.State.GetPlayer(), f.Generator, npc.MapPosition()
	visited := func(dlvl int) bool { return s.GetLastDungeonLevel() >= dlvl }
	avail := func(q int) bool { return s.D1Quests()[q].Active != d1.QuestNotAvail }
	active, v1, v2 := s.QuestActive, func(q int) int { return s.QuestVar(q, 1) }, func(q int) int { return s.QuestVar(q, 2) }
	name := npc.Name()
	switch npc.GetVendorType() {
	case VendorTypeOgden:
		if !visited(2) || !avail(d1.QSkelKing) { // 1.09 checks Ogden's Sign only inside Leoric's check
			break
		}
		if v2(d1.QSkelKing) == 0 {
			s.SetQuestVar(d1.QSkelKing, 2, 1)
			s.startTownQuest(d1.QSkelKing)
			f.say(name, "TEXT_KING2")
			return true
		}
		if active(d1.QSkelKing) == d1.QuestDone && v2(d1.QSkelKing) == 1 {
			s.SetQuestVar(d1.QSkelKing, 2, 2)
			s.SetQuestVar(d1.QSkelKing, 1, 2)
			f.say(name, "TEXT_KING4")
			return true
		}
		if visited(3) && avail(d1.QLtBanner) {
			if active(d1.QLtBanner) != d1.QuestDone && v2(d1.QLtBanner) == 0 {
				s.SetQuestVar(d1.QLtBanner, 2, 1)
				s.startTownQuest(d1.QLtBanner)
				f.say(name, "TEXT_BANNER2")
				return true
			}
			if v2(d1.QLtBanner) == 1 && d1TakeItem(player, d1.IdiBanner) {
				f.completeD1Quest(d1.QLtBanner)
				s.SetQuestVar(d1.QLtBanner, 1, 3)
				f.dropD1Item(gen.SpawnUnique(d1.UItemHarCrest, 0), pos)
				f.say(name, "TEXT_BANNER3")
				return true
			}
		}
	case VendorTypeDeadGuy:
		if q := d1.QButcher; active(q) == d1.QuestInit || active(q) == d1.QuestActive && v1(q) == 0 {
			s.startQuest(q)
			s.SetQuestVar(q, 1, 1)
			f.say(name, "TEXT_BUTCH9")
		} else if active(q) == d1.QuestActive {
			f.playD1HeroSound(8) // his last words are said
		}
		return true
	case VendorTypeWeapons:
		if visited(4) && avail(d1.QRock) {
			if v2(d1.QRock) == 0 {
				s.SetQuestVar(d1.QRock, 2, 1)
				s.startTownQuest(d1.QRock)
				f.say(name, "TEXT_INFRA5")
				return true
			}
			if v2(d1.QRock) == 1 && d1TakeItem(player, d1.IdiRock) {
				f.completeD1Quest(d1.QRock)
				s.SetQuestVar(d1.QRock, 2, 2)
				s.SetQuestVar(d1.QRock, 1, 2)
				f.dropD1Item(gen.SpawnUnique(d1.UItemInfraRing, 0), pos)
				f.say(name, "TEXT_INFRA7")
				return true
			}
		}
		if visited(9) && avail(d1.QAnvil) {
			if active(d1.QAnvil) != d1.QuestDone && v2(d1.QAnvil) == 0 &&
				(v2(d1.QRock) == 2 || active(d1.QRock) == d1.QuestActive && v2(d1.QRock) == 1) {
				s.SetQuestVar(d1.QAnvil, 2, 1)
				s.startTownQuest(d1.QAnvil)
				f.say(name, "TEXT_ANVIL5")
				return true
			}
			if v2(d1.QAnvil) == 1 && d1TakeItem(player, d1.IdiAnvil) {
				f.completeD1Quest(d1.QAnvil)
				s.SetQuestVar(d1.QAnvil, 2, 2)
				s.SetQuestVar(d1.QAnvil, 1, 2)
				f.dropD1Item(gen.SpawnUnique(d1.UItemGriswold, 0), pos)
				f.say(name, "TEXT_ANVIL7")
				return true
			}
		}
	case VendorTypeWands:
		q := d1.QMushroom
		switch {
		case active(q) == d1.QuestInit && d1TakeItem(player, d1.IdiFungalTm):
			s.startQuest(q)
			s.SetQuestVar(q, 1, d1.QSTomeGiven)
			f.say(name, "TEXT_MUSH8")
			return true
		case active(q) != d1.QuestActive:
		case v1(q) >= d1.QSTomeGiven && v1(q) <= d1.QSMushPicked:
			if d1TakeItem(player, d1.IdiMushroom) {
				s.SetQuestVar(q, 1, d1.QSMushGiven)
				f.say(name, "TEXT_MUSH10")
				return true
			}
			if v2(q)&1 == 0 { // D1's _qmsg != TEXT_MUSH9: said once
				s.SetQuestVar(q, 2, v2(q)|1)
				f.say(name, "TEXT_MUSH9")
				return true
			}
		case d1HasItem(player, d1.IdiSpecElix) != nil:
			f.completeD1Quest(q)
			f.say(name, "TEXT_MUSH12")
			return true
		case d1HasItem(player, d1.IdiBrain) != nil && v2(q)&2 == 0: // _qvar2 != TEXT_MUSH11
			s.SetQuestVar(q, 2, v2(q)|2)
			f.say(name, "TEXT_MUSH11")
			return true
		}
	case VendorTypeGeneral:
		said := false
		if q := d1.QPWater; visited(1) {
			if active(q) == d1.QuestInit {
				s.startQuest(q)
				s.SetQuestVar(q, 1, 1)
				f.say(name, "TEXT_POISON3")
				said = true
			} else if active(q) == d1.QuestDone && v1(q) != 2 {
				s.SetQuestVar(q, 1, 2)
				f.dropD1Item(gen.SpawnUnique(d1.UItemTRing, 0), pos)
				f.say(name, "TEXT_POISON5")
				said = true
			}
		}
		if q := d1.QMushroom; active(q) == d1.QuestActive && v1(q) >= d1.QSMushGiven && v1(q) < d1.QSBrainGiven && d1TakeItem(player, d1.IdiBrain) {
			f.dropD1Item(gen.PlainItem(d1.IdiSpecElix), pos)
			s.SetQuestVar(q, 1, d1.QSBrainGiven)
			f.say(name, "TEXT_MUSH4")
			said = true
		}
		return said
	case VendorTypeCain:
		q := d1.QBetrayer
		if active(q) == d1.QuestInit && d1TakeItem(player, d1.IdiLazStaff) {
			s.SetQuestVar(q, 1, 2)
			s.startQuest(q)
			f.say(name, "TEXT_VILE1")
			UpdateD1Quests(s, s.GetMap())
			return true
		}
		if active(q) == d1.QuestDone && v1(q) == 7 {
			s.SetQuestVar(q, 1, 8)
			s.GetFlags().SetTrue(QuestFlag(d1.QDiablo) + ".log")
			f.say(name, "TEXT_VILE3")
			return true
		}
	}
	return false
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
		s.GetFlags().SetTrue(QuestFlag(d1.QGarbud) + ".log")
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
	f := s.factory
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
			open("red_portal", d1RedPortal, gridmap.Transition{TargetMap: D1LazarusLevel, TargetLocation: "ladder_up"})
		}
		if betrayer == d1.QuestDone {
			open("ladder_down", d1StairsDown, gridmap.Transition{TargetMap: defs.LevelNameFromDepth(16), TargetLocation: "ladder_up"})
		}
		if q := d1.QVeil; s.QuestActive(q) == d1.QuestActive && s.QuestVar(q, 1) == 0 {
			s.SetQuestVar(q, 1, 1)
			f.spawnD1QuestItem(m, d1.IdiGldnElix)
		}
	case D1LazarusLevel:
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
		if lvl := gen.levels[m.GetName()]; lvl != nil && s.QuestVar(d1.QBlood, 2) >= 3 { // OperatePedistal: Blood2.DUN's torches light the hall
			for _, o := range s.factory.D1().D.DunObjects("l2/blood2", 2*sp[0], 2*sp[1]) {
				d1PlaceObject(s.factory, m, lvl, o, toCell(o.X, o.Y))
			}
			delete(s.glowCache, m)
		}
	}
	if m.GetName() == D1LazarusLevel { // ResyncQuests: the way to Lazarus, then to Red Vex and Black Jade
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
			if m.GetName() == D1LazarusLevel {
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
	if m.GetName() != D1LazarusLevel || o.pos != pieceCell(d1VileCenter) || d1VileBooksRead(m) < 2 {
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

// d1QuestKill ports CheckQuestKill and M_DiabloDeath's quest, plus Poisoned Water's check of CheckQuests: the
// set level is clean once its last monster dies.
func d1QuestKill(ctx ActionContext, mon *D1Monster) {
	state := ctx.GameState
	f := state.factory
	q := -1
	switch {
	case mon.Data.ID == "MT_SKING":
		q = d1.QSkelKing
	case mon.Data.ID == "MT_CLEAVER":
		q = d1.QButcher
	case mon.Data.ID == "MT_DIABLO":
		q = d1.QDiablo
	case mon.Unique != nil:
		if uq, ok := map[string]int{"Gharbad the Weak": d1.QGarbud, "Zhar the Mad": d1.QZhar, "Arch-Bishop Lazarus": d1.QBetrayer,
			"Warlord of Blood": d1.QWarlord}[mon.Unique.Name]; ok {
			q = uq
		}
	}
	if sl := d1SetLevelOf(state.GetMap().GetName()); sl != nil && sl.SL == d1.SLPoisonWater && state.QuestActive(d1.QPWater) != d1.QuestInit {
		clean := true
		for _, a := range state.GetMap().Actors() {
			clean = clean && (a.d1Monster == nil || a.d1Monster == mon || a.d1Monster.Golem || !a.IsAlive())
		}
		if clean && state.QuestActive(d1.QPWater) != d1.QuestDone {
			f.completeD1Quest(d1.QPWater)
			f.playD1Sound("IS_QUESTDN")
			state.mapGen.cleanD1Water(state.GetMap())
		}
	}
	if q < 0 || state.QuestActive(q) == d1.QuestDone {
		return
	}
	f.completeD1Quest(q)
	if n, ok := map[int]int{d1.QSkelKing: 82, d1.QButcher: 80, d1.QGarbud: 61, d1.QZhar: 62, d1.QBetrayer: 83, d1.QWarlord: 94}[q]; ok {
		f.playD1HeroSound(n)
	}
	switch q {
	case d1.QBetrayer:
		state.SetQuestVar(q, 1, 7)
		state.SetQuestVar(q, 2, 4)
		state.SetQuestActive(d1.QDiablo, d1.QuestActive)
		UpdateD1Quests(state, state.GetMap())
	case d1.QDiablo:
		if ui := ctx.UserInterface(); ui != nil {
			ui.OpenTextWindow(util.ToLabelText(d1EndText()))
		}
	}
}

func d1EndText() []string {
	r := files.MustOpenReader(path.Join("assets", "endMessage.txt"))
	defer r.Close()
	data, _ := io.ReadAll(r)
	return strings.Split(string(data), "\n")
}

// d1QuestItemPickedUp ports CheckQuestItem.
func (f *Factory) d1QuestItemPickedUp(it *d1.Item) {
	s := f.State
	switch it.BaseIdx {
	case d1.IdiOptAmulet:
		f.completeD1Quest(d1.QBlind)
	case d1.IdiMushroom:
		if s.QuestActive(d1.QMushroom) == d1.QuestActive && s.QuestVar(d1.QMushroom, 1) == d1.QSMushSpawned {
			f.playD1HeroSound(95)
			s.SetQuestVar(d1.QMushroom, 1, d1.QSMushPicked)
		}
	case d1.IdiAnvil, d1.IdiRock:
		q := map[int]int{d1.IdiAnvil: d1.QAnvil, d1.IdiRock: d1.QRock}[it.BaseIdx]
		if s.QuestActive(q) == d1.QuestInit {
			s.SetQuestActive(q, d1.QuestActive)
			s.SetQuestVar(q, 1, 1)
		}
		if s.QuestLog(q) {
			f.playD1HeroSound(map[int]int{d1.QAnvil: 89, d1.QRock: 87}[q])
		}
	case d1.IdiGldnElix:
		f.playD1HeroSound(88)
	case d1.IdiArmOfVal:
		f.completeD1Quest(d1.QBlood)
		f.playD1HeroSound(91)
	}
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
			f.playD1HeroSound(13)
			f.Msg("I can't open this yet.")
		} else if obj.Active {
			obj.Active = false
			f.dropD1Item(gen.PlainItem(d1.IdiMushroom), o.pos)
			s.SetQuestVar(q, 1, d1.QSMushSpawned)
		}
	case d1.ObjSIGNCHEST: // OperateInnSignChest
		if s.QuestVar(d1.QLtBanner, 1) != 2 {
			f.playD1HeroSound(24)
			f.Msg("I can't open this yet.")
		} else if obj.Active {
			obj.Active = false
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
		f.say(o.Name(), d1.QuestMsgs[q])
	case d1.ObjPEDISTAL: // OperatePedistal
		q := d1.QBlood
		if stones := s.QuestVar(q, 2); stones < 3 && d1TakeItem(player, d1.IdiBldStone) {
			s.SetQuestVar(q, 2, stones+1)
			UpdateD1Quests(s, m)
			if stones+1 == 3 {
				obj.Active = false
				f.dropD1Item(gen.SpawnUnique(d1.UItemArmOfVal, d1DungeonLevel(s)), cell(2*sp[0]+25, 2*sp[1]+19))
			}
		}
	case d1.ObjBOOK2R: // OperateSChambBk
		if q := d1.QSchamb; obj.Active {
			obj.Active = false
			if s.QuestActive(q) == d1.QuestInit {
				s.startQuest(q)
			}
			UpdateD1Quests(s, m)
		}
		f.say(o.Name(), d1.QuestMsgs[d1.QSchamb])
	case d1.ObjBOOK2L: // OperateBook
		if m.GetName() == D1LazarusLevel {
			f.operateD1VileBook(o, player)
		} else if sl := d1SetLevelOf(m.GetName()); obj.Active && sl != nil && sl.SL == d1.SLBoneChamb {
			obj.Active = false
			h := player.hero
			h.SpellLevels[d1.SplGuardian] = min(h.SpellLevels[d1.SplGuardian]+1, 15)
			f.RefreshD1Spells(player)
			f.completeD1Quest(d1.QSchamb)
			f.playD1Sound("IS_QUESTDN")
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
