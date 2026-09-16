package common

import (
	"encoding/json"
	"fmt"
	"os"
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
	"rpg/util"
)

// Hellfire's quests (towners.cpp, inv.cpp, monster.cpp and objects.cpp's HELLFIRE branches, checked against
// DevilutionX): Farmer's Orchard or The Jersey's Jersey open the Nest, Grave Matters opens the Crypt, then the
// Defiler, Na-Krul, the Little Girl and the Cornerstone of the World.

// Hellfire's command.txt switches, kept with the save: Cowquest swaps Lester for the Complete Nut, Theoquest adds
// Celia. The debug menu sets them.
const (
	FlagCowQuest  = "hellfire:cowquest"
	FlagTheoQuest = "hellfire:theoquest"
)

// Hellfire's townspeople, by their maps.ldtk InternalName.
const (
	VendorTypeFarmer  VendorType = "lester"
	VendorTypeCowFarm VendorType = "nut"
	VendorTypeGirl    VendorType = "celia"
)

// D1CornerstoneFile keeps the item on the Cornerstone of the World between games, like Hellfire's registry entry.
var D1CornerstoneFile = "cornerstone.json"

// ApplyHellfireOptions rebuilds the game's quests from its seed with the switches in the flags (InitQuests,
// then the towners InitTowners creates).
func (s *State) ApplyHellfireOptions() {
	g := s.mapGen
	q := d1.InitQuests(g.d1Seeds)
	q.SetHellfireOptions(s.GetFlags().IsSet(FlagCowQuest), s.GetFlags().IsSet(FlagTheoQuest))
	*g.d1Quests = q
	if town := s.GetTownMap(string(defs.LevelNameTown)); town != nil {
		updateD1Town(s, town)
	}
}

// D1Level is the generated D1 level behind a built map, nil for any other.
func (s *State) D1Level(m *gridmap.GridMap[*Actor, *Item, Object]) *d1.Level {
	return s.mapGen.levels[m.GetName()]
}

// questMsg is quests[q]._qmsg: the text the quest log shows, which Grave Matters and Na-Krul change as they go.
func (s *State) questMsg(q int) string {
	switch v := s.GetFlags().AllEntries()[QuestFlag(q)+".msg"]; {
	case v == 0:
		return d1.QuestMsgs[q]
	case q == d1.QNakrul:
		return fmt.Sprintf("TEXT_BOOK%d", v)
	default:
		return fmt.Sprintf("TEXT_GRAVE%d", v)
	}
}

func (s *State) setQuestMsg(q, v int) { s.GetFlags().SetToValue(QuestFlag(q)+".msg", v) }

// logQuest makes quest q active and puts it in the log with message v (Hellfire's _qactive/_qlog/_qmsg triple).
func (s *State) logQuest(q, msg int) {
	s.startQuest(q)
	if msg != 0 {
		s.setQuestMsg(q, msg)
	}
}

// d1HellfireTownerQuests ports TalkToTowner's Hellfire branches; true if the townsperson said something.
func (f *Factory) d1HellfireTownerQuests(npc *Actor) bool {
	s, player, gen, pos := f.State, f.State.GetPlayer(), f.Generator, npc.MapPosition()
	visited := func(dlvl int) bool { return s.GetLastDungeonLevel() >= dlvl }
	name := npc.Name()
	beside := pos.Add(geometry.Point{X: 1})
	switch npc.GetVendorType() {
	case VendorTypeBarmaid: // Grave Matters' hint, once the hero carries the Cathedral Map
		if !visited(21) && d1HasItem(player, d1.IdiMapOfDoom) != nil && s.questMsg(d1.QGrave) != "TEXT_GRAVE8" {
			s.logQuest(d1.QGrave, 8)
			f.say(name, "TEXT_GRAVE8")
			return true
		}
	case VendorTypeFarmer:
		q := d1.QFarmer
		switch {
		case s.QuestVar(q, 2) == 1: // QUEST_HIVE_DONE: rewarded, and he goes home
		case s.QuestActive(q) == d1.QuestDone:
			f.say(name, "TEXT_FARMER4")
			f.dropD1Item(gen.PlainItem(d1.IdiAuric), beside)
			s.SetQuestVar(q, 2, 1)
		case s.QuestActive(q) == d1.QuestActive:
			f.say(name, map[bool]string{true: "TEXT_FARMER2", false: "TEXT_FARMER3"}[d1HasItem(player, d1.IdiRuneBomb) != nil])
		case d1HasItem(player, d1.IdiRuneBomb) != nil:
			s.SetQuestVar(q, 1, 1)
			s.logQuest(q, 0)
			f.say(name, "TEXT_FARMER2")
		case !visited(9) && player.hero.Level < 15:
			msg := "TEXT_FARMER8"
			for _, v := range [][2]any{{2, "TEXT_FARMER5"}, {5, "TEXT_FARMER7"}, {7, "TEXT_FARMER9"}} {
				if visited(v[0].(int)) {
					msg = v[1].(string)
				}
			}
			f.say(name, msg)
		default:
			s.SetQuestVar(q, 1, 1)
			s.logQuest(q, 0)
			f.dropD1Item(gen.PlainItem(d1.IdiRuneBomb), beside)
			f.say(name, "TEXT_FARMER1")
		}
		return true
	case VendorTypeCowFarm:
		q := d1.QJersey
		stage := s.QuestVar(q, 2) // QUEST_HIVE_TEASE1, _TEASE2, _ACTIVE: 1, 2, 3
		switch {
		case d1TakeItem(player, d1.IdiGreySuit):
			f.say(name, "TEXT_JERSEY7")
		case d1TakeItem(player, d1.IdiBrownSuit):
			f.dropD1Item(gen.SpawnUnique(d1.UItemBovine, s.D1Quests()[q].Level), beside)
			f.completeD1Quest(q)
			f.say(name, "TEXT_JERSEY8")
		case d1HasItem(player, d1.IdiRuneBomb) != nil:
			s.SetQuestVar(q, 1, 1)
			s.logQuest(q, 0)
			f.say(name, "TEXT_JERSEY5")
		case s.QuestActive(q) == d1.QuestDone:
			f.say(name, "TEXT_JERSEY1")
		case s.QuestActive(q) == d1.QuestActive:
			f.say(name, "TEXT_JERSEY5")
		case stage < 2:
			s.SetQuestVar(q, 2, stage+1)
			f.say(name, [2]string{"TEXT_JERSEY1", "TEXT_JERSEY2"}[stage])
		case stage == 2:
			s.SetQuestVar(q, 2, 3)
			f.say(name, "TEXT_JERSEY3")
		case !visited(9) && player.hero.Level < 15:
			f.say(name, fmt.Sprintf("TEXT_JERSEY%d", 9+d1Rng.Intn(4)))
		default:
			s.SetQuestVar(q, 1, 1)
			s.logQuest(q, 0)
			f.dropD1Item(gen.PlainItem(d1.IdiRuneBomb), beside)
			f.say(name, "TEXT_JERSEY4")
		}
		return true
	case VendorTypeGirl:
		q := d1.QGirl
		switch {
		case s.QuestActive(q) != d1.QuestDone && d1TakeItem(player, d1.IdiTheodore):
			f.dropD1Item(gen.TypeItem(13, d1.ITypeAmulet, "IMISC_AMULET", true), pos) // CreateAmulet(13)
			f.completeD1Quest(q)
			f.say(name, "TEXT_GIRL4")
			updateD1Town(s, s.GetMap())
		case s.QuestActive(q) == d1.QuestActive:
			f.say(name, "TEXT_GIRL3")
		case s.QuestActive(q) != d1.QuestDone:
			s.SetQuestVar(q, 1, 1)
			s.logQuest(q, 0)
			f.say(name, "TEXT_GIRL2")
		}
		return true
	}
	return false
}

// d1HellfireGates are the town entrances Hellfire's quests open (TownOpenHive, TownOpenGrave), with the quest.
var d1HellfireGates = []struct {
	loc  string
	dlvl int
	q    int
}{{"to_nest", 17, d1.QFarmer}, {"to_crypt", 21, d1.QGrave}}

// updateD1Town brings the town up to date with Hellfire's quests: the Nest and the Crypt stay shut until their
// quest opened them, and Lester, the Complete Nut and Celia are only there when InitTowners would create them.
func updateD1Town(s *State, m *gridmap.GridMap[*Actor, *Item, Object]) {
	if m == nil || m.GetName() != string(defs.LevelNameTown) || s.mapGen == nil { // the town is built before the quests
		return
	}
	for _, g := range d1HellfireGates {
		p := m.GetNamedLocation(g.loc)
		if m.GetNamedLocationByPos(p) != g.loc {
			continue
		}
		_, open := m.GetTransitionAt(p)
		if want := s.QuestActive(g.q) == d1.QuestDone; want && !open {
			m.AddTransitionAt(p, gridmap.Transition{TargetMap: defs.LevelNameFromDepth(g.dlvl), TargetLocation: "warp"})
		} else if !want && open {
			m.RemoveTransitionAt(p)
		}
	}
	flags := s.GetFlags()
	present := map[VendorType]bool{
		VendorTypeFarmer:  !flags.IsSet(FlagCowQuest) && s.QuestVar(d1.QFarmer, 2) != 1,
		VendorTypeCowFarm: flags.IsSet(FlagCowQuest),
		VendorTypeGirl:    flags.IsSet(FlagTheoQuest) && s.GetLastDungeonLevel() >= 17 && s.QuestActive(d1.QGirl) != d1.QuestDone,
	}
	if s.awayTowners == nil {
		s.awayTowners = map[VendorType]awayTowner{}
	}
	for _, a := range m.Actors() {
		if want, ok := present[a.GetVendorType()]; ok && !want {
			s.awayTowners[a.GetVendorType()] = awayTowner{a, a.MapPosition()}
			m.RemoveActor(a)
		}
	}
	for vt, away := range s.awayTowners {
		if present[vt] && m.CanPlaceActorHere(away.pos) {
			m.AddActor(away.actor, away.pos)
			delete(s.awayTowners, vt)
		}
	}
}

type awayTowner struct {
	actor *Actor
	pos   geometry.Point
}

// d1HellfireItemAction gives Hellfire's quest items their use: the Rune Bomb and the Cathedral Map open the Nest
// and the Crypt when used by their entrance (D1 drops them onto it), the Reconstructed Note is read.
func (g *Factory) d1HellfireItemAction(item *Item, it *d1.Item) {
	var effect func(user *Actor)
	switch it.BaseIdx {
	case d1.IdiRuneBomb, d1.IdiMapOfDoom:
		gate := d1HellfireGates[0]
		if it.BaseIdx == d1.IdiMapOfDoom {
			gate = d1HellfireGates[1]
		}
		effect = func(user *Actor) {
			m := g.State.GetMap()
			if m.GetName() != string(defs.LevelNameTown) || geometry.DistanceChebyshev(user.MapPosition(), m.GetNamedLocation(gate.loc)) > 2 {
				if it.BaseIdx == d1.IdiMapOfDoom { // UseItem's IMISC_MAPOFDOOM: the map is only looked at
					g.Msg("The map shows a crypt beneath the graveyard.")
				} else {
					g.Msg("Nothing here to blow open.")
				}
				return
			}
			user.GetInventory().Remove(item)
			g.State.SetQuestActive(gate.q, d1.QuestDone)
			if it.BaseIdx == d1.IdiRuneBomb { // AddOpenNest: four big explosions on the hive's mound
				g.Msg("The rune bomb blows the hive open!")
			} else {
				g.playD1Sound("IS_SARC")
				g.Msg("The crypt beneath the graveyard opens.")
			}
			updateD1Town(g.State, m)
		}
	case d1.IdiFullNote:
		effect = func(user *Actor) { g.say(it.Name, "TEXT_BOOK9") }
	default:
		return
	}
	action := NewUntargetedAction(it.Name, item.icon)
	action.SetEffect(&UntargetedEffect{
		Effect:      func(user *Actor, _ EffectSource) { effect(user) },
		Description: []util.LabelText{util.LabelTextFromString(it.Name)},
	})
	item.SetUntargetedAction(action)
}

// d1HellfireItemPickedUp ports CheckQuestItem's Hellfire part: the Cathedral Map starts Grave Matters, and the
// three torn notes become the Reconstructed Note (TryCombineNaKrulNotes).
func (f *Factory) d1HellfireItemPickedUp(it *d1.Item) {
	s, player := f.State, f.State.GetPlayer()
	switch it.BaseIdx {
	case d1.IdiMapOfDoom:
		if s.QuestActive(d1.QGrave) != d1.QuestDone {
			s.SetQuestActive(d1.QGrave, d1.QuestActive)
		}
		if s.QuestVar(d1.QGrave, 1) != 1 {
			f.playD1HeroSound(79)
			s.SetQuestVar(d1.QGrave, 1, 1)
		}
	case d1.IdiNote1, d1.IdiNote1 + 1, d1.IdiNote1 + 2:
		for n := d1.IdiNote1; n <= d1.IdiNote1+2; n++ {
			if d1HasItem(player, n) == nil {
				return
			}
		}
		for n := d1.IdiNote1; n <= d1.IdiNote1+2; n++ {
			d1TakeItem(player, n)
		}
		f.playD1HeroSound(46)
		player.GetInventory().Add(f.NewD1Item(f.Generator.PlainItem(d1.IdiFullNote)))
	}
}

// d1HellfireLoot ports SpawnLoot's Hellfire uniques: the Defiler leaves the Cathedral Map, the Hork Demon Theodore
// (Theoquest) or an amulet, Na-Krul three magic weapons and an Apocalypse book, and Hork Spawn nothing. It
// reports whether it replaced the monster's usual drop.
func (f *Factory) d1HellfireLoot(mon *D1Monster, pos geometry.Point, ilvl int) bool {
	s, gen := f.State, f.Generator
	drop := func(it *d1.Item) { s.AddItemToMapWithDistribution(f.NewD1Item(it), pos) }
	switch {
	case mon.Data.ID == "MT_SPAWN":
	case mon.Unique != nil && mon.Unique.Name == "The Defiler":
		drop(gen.PlainItem(d1.IdiMapOfDoom))
		f.completeD1Quest(d1.QDefiler)
	case mon.Unique != nil && mon.Unique.Name == "Hork Demon":
		if s.GetFlags().IsSet(FlagTheoQuest) {
			drop(gen.PlainItem(d1.IdiTheodore))
		} else {
			drop(gen.TypeItem(13, d1.ITypeAmulet, "IMISC_AMULET", true))
		}
	case mon.Data.ID == "MT_NKR":
		drop(gen.MagicItemWithCurs(ilvl, d1.ITypeSword, d1.ICursGreatSword))
		drop(gen.MagicItemWithCurs(ilvl, d1.ITypeStaff, d1.ICursWarStaff))
		drop(gen.MagicItemWithCurs(ilvl, d1.ITypeBow, d1.ICursLongWarBow))
		drop(gen.SpellBook(ilvl, d1.SplApocalypse))
	default:
		return false
	}
	return true
}

// D1MonsterGreeting ports ProcessMonsters' first-sight lines: the Butcher's "fresh meat", Na-Krul's taunt (which
// one depends on whether his door was opened by the books) and the Defiler's buzz.
func (f *Factory) D1MonsterGreeting(a *Actor) {
	switch mon := a.d1Monster; mon.Data.ID {
	case "MT_CLEAVER":
		f.playD1Sound("USFX_CLEAVER")
	case "MT_NKR":
		switch {
		case f.State.GetFlags().IsSet(FlagCowQuest):
			f.playD1Sound("USFX_NAKRUL6")
		case f.State.QuestVar(d1.QNakrul, 2) == 1:
			f.playD1Sound("USFX_NAKRUL4")
		default:
			f.playD1Sound("USFX_NAKRUL5")
		}
	case "MT_BUG":
		if mon.Unique != nil {
			f.playD1Sound("USFX_DEFILER8")
		}
	}
}

// d1HellfireArrival ports the level-entry parts: CheckQuests' activation of Grave Matters, the Defiler, Na-Krul
// and The Jersey's Jersey one level above theirs, and PlayDungMsgs' Defiler lines on 17 and 19 and the hero's
// "place of great power" on 21, each once.
func (f *Factory) d1HellfireArrival(m *gridmap.GridMap[*Actor, *Item, Object]) {
	s := f.State
	dlvl := LevelFromName(m.GetName())
	if d1SetLevelOf(m.GetName()) != nil || dlvl == 0 {
		return
	}
	for _, q := range []int{d1.QGrave, d1.QDefiler, d1.QNakrul, d1.QJersey} {
		if s.QuestActive(q) == d1.QuestInit && dlvl == s.D1Quests()[q].Level-1 {
			s.SetQuestActive(q, d1.QuestActive)
		}
	}
	once := func(key string) bool {
		key = "hellfire:dungmsg:" + key
		if s.GetFlags().IsSet(key) {
			return false
		}
		s.GetFlags().SetTrue(key)
		return true
	}
	switch {
	case dlvl == 17 && once("17"):
		s.logQuest(d1.QDefiler, 0)
		f.say("The Defiler", "TEXT_DEFILER1")
	case dlvl == 19 && once("19"):
		f.say("The Defiler", "TEXT_DEFILER3")
	case dlvl == 21 && once("21"):
		f.playD1HeroSound(92)
	}
	if dlvl == 21 {
		f.d1CornerstoneLoad(m)
	}
	if dlvl == 24 && (s.QuestActive(d1.QNakrul) == d1.QuestDone || s.QuestVar(d1.QNakrul, 2) == 1) {
		f.d1OpenNakrulRoom(m)
	}
}

// d1CryptJournal ports OperateStoryBook in the Crypt: a journal puts Na-Krul in the quest log with its text, and
// Na-Krul's three books, read in their order, open his door and weaken him (OperateNakrulBook, CMD_NAKRUL).
// It reports whether the book's text is still to be shown.
func (f *Factory) d1CryptJournal(obj *d1.Object) bool {
	s, m := f.State, f.State.GetMap()
	q := d1.QNakrul
	if obj.Var8 >= 6 && LevelFromName(m.GetName()) == 24 {
		if s.QuestActive(q) == d1.QuestDone || s.QuestVar(q, 2) == 1 {
			return true
		}
		seq := s.QuestVar(q, 1) // NaKrulTomeSequence
		switch {
		case obj.Var8 == 6:
			seq = 1
		case obj.Var8 == 7 && seq == 1:
			seq = 2
		case obj.Var8 == 8 && seq == 2:
			s.SetQuestVar(q, 2, 1) // IsUberRoomOpened
			f.completeD1Quest(q)
			f.d1OpenNakrulRoom(m)
			f.d1WeakenNakrul(m)
			return false
		default:
			seq = 0
		}
		s.SetQuestVar(q, 1, seq)
		return true
	}
	if obj.Var2 > 300 && LevelFromName(m.GetName()) >= 21 && s.QuestActive(q) != d1.QuestDone {
		s.logQuest(q, obj.Var2-313)
	}
	return true
}

// d1NakrulLever ports OperateLever on level 24: the lever opens Na-Krul's door, without weakening him.
func (f *Factory) d1NakrulLever(m *gridmap.GridMap[*Actor, *Item, Object]) bool {
	if LevelFromName(m.GetName()) != 24 {
		return false
	}
	f.completeD1Quest(d1.QNakrul)
	f.d1OpenNakrulRoom(m)
	return true
}

// d1OpenNakrulRoom ports SyncNakrulRoom on the built map.
func (f *Factory) d1OpenNakrulRoom(m *gridmap.GridMap[*Actor, *Item, Object]) {
	gen := f.State.mapGen
	lvl := gen.levels[m.GetName()]
	if lvl == nil || !lvl.OpenNakrulRoom() {
		return
	}
	f.playD1Sound("IS_CROPEN")
	p := toCell(lvl.Uber[0], lvl.Uber[1])
	gen.setD1Tiles(m, lvl, p.X-1, p.Y-3, p.X+1, p.Y+2, func(geometry.Point) bool { return false })
}

// d1WeakenNakrul ports WeakenNaKrul: -50 armor, no resistances and half his life.
func (f *Factory) d1WeakenNakrul(m *gridmap.GridMap[*Actor, *Item, Object]) {
	for _, a := range m.Actors() {
		if mon := a.d1Monster; mon != nil && mon.Data.ID == "MT_NKR" && a.IsAlive() {
			mon.AC -= 50
			mon.MagicRes = 0
			st := a.GetStats()
			half := st.GetAttribute(stats.MaxHealth) / 2
			st.SetAttribute(stats.MaxHealth, half)
			hp := st.GetResource(stats.Health)
			hp.SetToMax()
		}
	}
}

// d1CornerstoneLoad ports CornerstoneLoad: whatever lies on the Cornerstone of the World is replaced by the item
// last left there, in any game.
func (f *Factory) d1CornerstoneLoad(m *gridmap.GridMap[*Actor, *Item, Object]) {
	lvl := f.State.mapGen.levels[m.GetName()]
	if lvl == nil || lvl.Cornerstone == ([2]int{}) {
		return
	}
	p := toCell(lvl.Cornerstone[0], lvl.Cornerstone[1])
	if m.IsItemAt(p) {
		m.RemoveItem(m.ItemAt(p))
	}
	data, err := os.ReadFile(D1CornerstoneFile)
	var it d1.Item
	if err != nil || len(data) == 0 || json.Unmarshal(data, &it) != nil {
		return
	}
	m.AddItem(f.NewD1Item(&it), p)
}

// d1CornerstonePlace ports the item drop onto the Cornerstone: dropped beside it, the item goes onto it, is kept
// for later games (CornerstoneSave) and completes the quest. It reports whether it took the item.
func (f *Factory) d1CornerstonePlace(item *Item, from geometry.Point) bool {
	m := f.State.GetMap()
	lvl := f.State.mapGen.levels[m.GetName()]
	if item.d1 == nil || lvl == nil || lvl.Dlvl != 21 || lvl.Cornerstone == ([2]int{}) {
		return false
	}
	p := toCell(lvl.Cornerstone[0], lvl.Cornerstone[1])
	if geometry.DistanceChebyshev(from, p) > 1 || m.IsItemAt(p) {
		return false
	}
	m.AddItem(item, p)
	f.d1CornerstoneSave(item.d1)
	f.say("Cornerstone of the World", "TEXT_CORNSTN")
	f.completeD1Quest(d1.QCornstn)
	return true
}

// d1CornerstoneTaken empties the Cornerstone's memory when its item is picked up again.
func (f *Factory) d1CornerstoneTaken(p geometry.Point) {
	m := f.State.GetMap()
	if lvl := f.State.mapGen.levels[m.GetName()]; lvl != nil && lvl.Dlvl == 21 && lvl.Cornerstone != ([2]int{}) && toCell(lvl.Cornerstone[0], lvl.Cornerstone[1]) == p {
		f.d1CornerstoneSave(nil)
	}
}

func (f *Factory) d1CornerstoneSave(it *d1.Item) {
	var data []byte
	if it != nil {
		data, _ = json.Marshal(it)
	}
	if err := os.WriteFile(D1CornerstoneFile, data, 0o644); err != nil {
		f.Msg(fmt.Sprintf("The Cornerstone couldn't keep the item: %v", err))
	}
}
