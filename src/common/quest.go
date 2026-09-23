package common

import (
	"fmt"
	"rpg/src/defs"
	"rpg/src/dungen"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/rules"
	"strings"
)

// D1's quests (quests.cpp). Which quests a game has comes from its seed (InitQuests, State.Quests, not saved);
// what happened since lives in the saved flags under the quest's key: "quest:<key>" once done, ".active" once
// active, ".log" once in the quest log, ".v1"/".v2" its _qvar1/_qvar2. The keys, and everything else a quest
// says or places, come from the mod's quests.rec.

// QuestDefs are the mod's quests, in quest-id order.
func (s *State) QuestDefs() []defs.Quest { return s.factory.Gen().D.QuestDefs }

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
		return rules.QuestNotAvail // a quest this mod does not have is never active
	}
	switch f := s.GetFlags(); {
	case f.IsSet(s.QuestFlag(q)):
		return rules.QuestDone
	case f.IsSet(s.QuestFlag(q) + ".active"):
		return rules.QuestActive
	}
	return s.mapGen.quest(q).Active
}

func (s *State) SetQuestActive(q, v int) {
	switch v {
	case rules.QuestDone:
		s.GetFlags().SetTrue(s.QuestFlag(q))
	case rules.QuestActive:
		s.GetFlags().SetTrue(s.QuestFlag(q) + ".active")
	}
}

func (s *State) QuestLog(q int) bool { return s.GetFlags().IsSet(s.QuestFlag(q) + ".log") }

// startQuest makes quest q active and puts it in the quest log.
func (s *State) startQuest(q int) {
	if s.QuestActive(q) != rules.QuestDone {
		s.SetQuestActive(q, rules.QuestActive)
	}
	s.GetFlags().SetTrue(s.QuestFlag(q) + ".log")
}

// startTownQuest is TalkToTowner's usual start: logged, and only a quest that was still QUEST_INIT becomes active
// with _qvar1 1.
func (s *State) startTownQuest(q int) {
	if s.QuestActive(q) == rules.QuestInit {
		s.SetQuestActive(q, rules.QuestActive)
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
		return s.mapGen.quest(q).Var1
	}
	return s.mapGen.quest(q).Var2
}

func (s *State) SetQuestVar(q, n, v int) {
	s.GetFlags().SetToValue(fmt.Sprintf("%s.v%d", s.QuestFlag(q), n), v)
}

// questHere ports QuestStatus: quest q is available and the player is on its (not set) level.
func (s *State) questHere(q int) bool { return s.questOn(q, s.GetMap()) }

func (s *State) questOn(q int, m *gridmap.GridMap[*Actor, *Item, Object]) bool {
	qu := s.mapGen.quest(q)
	return qu.Active != rules.QuestNotAvail && q < len(s.QuestDefs()) && m.GetName() == s.QuestDefs()[q].Level
}

// completeQuest marks quest q done and says so.
func (f *Factory) completeQuest(q int) {
	if f.State.QuestActive(q) == rules.QuestDone {
		return
	}
	f.State.SetQuestActive(q, rules.QuestDone)
	f.Msg(fmt.Sprintf("Quest completed: %s", f.State.QuestDefs()[q].Name))
}

// setQuest is the quest whose set level (a d1_set record) level id is, -1 for any other level.
func (s *State) setQuest(id string) int {
	if cfg, ok := s.Level(id).Config.(dungen.SetLevelParams); ok {
		return s.QuestByKey(cfg.Quest)
	}
	return -1
}

// onSetLevelOf reports whether m is the set level of quest q.
func (s *State) onSetLevelOf(m *gridmap.GridMap[*Actor, *Item, Object], q int) bool {
	return q >= 0 && s.setQuest(m.GetName()) == q
}

// questTopic is what npc says about quest q, if S_TalkEnter lists it: the quest is active and logged. What
// each townsperson says comes from the quest's gossip lines; which of them is offered is the logic below.
// Black Mushroom's topic moves from Adria to Pepin once she has the mushroom, and Pepin's ends once he has the
// brain -- the one case the table cannot say.
func (s *State) questTopic(npc VendorType, q int) string {
	if s.QuestActive(q) != rules.QuestActive || !s.QuestLog(q) {
		return ""
	}
	id := s.QuestDefs()[q].Gossip[string(npc)]
	if v := s.QuestVar(rules.QMushroom, 1); q == rules.QMushroom && v >= rules.QSMushGiven {
		switch npc {
		case VendorTypeWands:
			id = ""
		case VendorTypeGeneral:
			if v < rules.QSBrainGiven {
				id = "TEXT_MUSH3"
			}
		}
	}
	return id
}

// QuestLogEntries ports the quest log: the active quests in it, each with its text.
func (s *State) QuestLogEntries() []string {
	lines := []string{"Quest Log", ""}
	for q, def := range s.QuestDefs() {
		if s.QuestActive(q) == rules.QuestActive && s.QuestLog(q) {
			lines = append(lines, def.Name, "  "+s.text(classText(s, def.LogText)), "")
		}
	}
	if len(lines) == 2 {
		lines = append(lines, "No active quests.")
	}
	return lines
}

func (s *State) text(id string) string { return s.factory.Gen().D.Texts[id] }

// classText picks the hero's class version of a book's text (TEXT_BONER, TEXT_RBONER, TEXT_MBONER).
func classText(s *State, id string) string {
	if !strings.HasSuffix(id, "BONER") && !strings.HasSuffix(id, "BLOODY") && !strings.HasSuffix(id, "BLINDING") && !strings.HasSuffix(id, "BLOODWAR") {
		return id
	}
	return "TEXT_" + s.factory.Generator.D.Class(s.GetPlayer().Hero().Class).TextPrefix + strings.TrimPrefix(id, "TEXT_")
}

// say ports InitQTextMsg: a quest text in a text window, and its speech.
func (f *Factory) say(speaker, id string) {
	id = classText(f.State, id)
	f.playSound(f.Gen().D.TextSfx[id])
	text := f.State.text(id)
	if f.Show != nil && text != "" {
		f.Show(ShowText{append([]string{speaker, ""}, strings.Split(text, "\n\n")...)})
	}
}

// hasItem ports PlrHasItem: the first item with that internal_name in the player's pack.
func hasItem(player *Actor, name string) *Item {
	for _, it := range player.GetInventory().Items() {
		if it.roll != nil && it.roll.BaseName() == name {
			return it
		}
	}
	return nil
}

// takeItem removes an item with that internal_name from the player's pack (RemoveInvItem); false if there is none.
func takeItem(player *Actor, name string) bool {
	it := hasItem(player, name)
	if it != nil {
		player.GetEquipment().UnEquip(it)
		player.GetInventory().Remove(it)
	}
	return it != nil
}

func (f *Factory) dropItem(it *rules.Item, pos geometry.Point) {
	f.State.AddItemToMapWithDistribution(f.NewRolledItem(it), pos)
}

// Monster goals (monster_goal) the talking bosses use.
const (
	goalNormal    = 1
	goalAttack2   = 5
	goalInquiring = 6
	goalTalking   = 7
)

// goalNames are the goals a talker trigger's condition sees.
var goalNames = map[int]string{goalNormal: "normal", goalAttack2: "attack2", goalInquiring: "inquiring", goalTalking: "talking"}

// initTalk sets a quest boss' talk state from the quest's progress (on_talker_spawn): D1 keeps it with the
// monster, here it is rebuilt whenever its level is. Other monsters have nothing to say.
func (f *Factory) initTalk(a *Actor) {
	a.monster.TalkMsg = ""
	if a.monster.IsTalker(f.Generator.D) {
		f.fireTalker(EventTalkerSpawn, a, false)
	}
}

// TalkToMonster ports TalktoMonster: the hero's hand-ins (on_talker_talk), then the talk.
func (f *Factory) TalkToMonster(a *Actor) {
	f.fireTalker(EventTalkerTalk, a, false)
	f.doTalk(a)
}

// doTalk ports M_DoTalk: the monster says its line, and what that sets in motion (on_talker_said).
func (f *Factory) doTalk(a *Actor) {
	mon := a.monster
	mon.Goal = goalTalking
	f.say(a.Name(), mon.TalkMsg)
	f.fireTalker(EventTalkerSaid, a, false)
}

// TalkerTurn ports the talk states of the MAI_* talkers for one turn (on_talker_turn); visible is D1's
// BFLAG_VISIBLE (the hero sees the monster). It reports whether the monster fights this turn: monsters.rec's
// fights_when, by default once it is neither waiting to speak nor speaking.
func (f *Factory) TalkerTurn(a *Actor, visible bool) bool {
	f.fireTalker(EventTalkerTurn, a, visible)
	cond := f.Generator.D.MonsterAIs[a.monster.AI].FightsWhen
	if cond == "" {
		cond = `goal != "inquiring" and goal != "talking"`
	}
	prev, prevVisible := f.talker, f.talkerVisible
	f.talker, f.talkerVisible = a, visible
	defer func() { f.talker, f.talkerVisible = prev, prevVisible }()
	return f.questCondition(cond)
}

// stairsDown is a func, not a var: package vars are set before defs.LoadIcons binds the icon IDs.
func stairsDown() gridmap.Tile {
	return gridmap.Tile{DefinedIcon: defs.IconStairsDown, DefinedDescription: "stairs leading down", IsWalkable: true, IsTransparent: true, IsMissilePassable: true, Special: gridmap.SpecialTileStairsDown}
}

var redPortal = gridmap.Tile{DefinedIcon: 72, DefinedDescription: "a red portal", IsWalkable: true, IsTransparent: true, IsMissilePassable: true}

// Lazarus' lair names its places in levels.rec ("location:"): vile_book_N, the magic circle in front of each
// Book of Vileness, and vile_book_N_to, where it teleports the hero; vile_center, the central circle, and
// vile_altar, the one in front of Lazarus it leads to.

// namedLocation is the cell the map's level calls name, if it has one.
func namedLocation(m *gridmap.GridMap[*Actor, *Item, Object], name string) (geometry.Point, bool) {
	p := m.GetNamedLocation(name)
	return p, m.GetNamedLocationByPos(p) == name
}

// operateVileBook ports OperateBook in Lazarus' lair: read while standing on the magic circle in front of it, a
// Book of Vileness teleports the hero into the sealed room behind it and opens that room.
func (f *Factory) operateVileBook(o *Prop, player *Actor) {
	m := f.State.GetMap()
	to, onCircle := geometry.Point{}, false
	for i := 1; ; i++ {
		circle, ok := namedLocation(m, fmt.Sprintf("vile_book_%d", i))
		if !ok {
			break
		}
		if player.MapPosition() == circle {
			to, onCircle = namedLocation(m, fmt.Sprintf("vile_book_%d_to", i))
		}
	}
	if !o.O.Active || !onCircle {
		return
	}
	o.O.Active = false
	f.vileTeleport(player, to)
	f.State.QuestsChanged(f.State.GetMap())
}

// vileTeleport ports MIS_RNDTELEPORT onto a lair circle: the hero lands on it, or next to it when taken
// (GetVileMissPos).
func (f *Factory) vileTeleport(player *Actor, p geometry.Point) {
	m := f.State.GetMap()
	if !m.CanPlaceActorHere(p) {
		if near := freeCellsNear(m, p, 1, m.CanPlaceActorHere); len(near) > 0 {
			p = near[0]
		}
	}
	f.movePlayer(player, p, "Teleport")
}

// magicCircle ports Obj_Circle: once both Books of Vileness are read, the central circle opens the way to
// Lazarus and teleports the hero in front of him.
func (f *Factory) magicCircle(o *Prop, player *Actor) {
	m, s := f.State.GetMap(), f.State
	center, _ := namedLocation(m, "vile_center")
	altar, hasAltar := namedLocation(m, "vile_altar")
	if !s.onSetLevelOf(m, rules.QBetrayer) || o.pos != center || !hasAltar || vileBooksRead(m) < 2 {
		return
	}
	// 1.09 sets _qvar1 4 on every step onto the circle, even after Lazarus spoke (its BUGFIX note); not done again here
	if s.QuestActive(rules.QBetrayer) == rules.QuestActive && s.QuestVar(rules.QBetrayer, 1) < 4 {
		s.SetQuestVar(rules.QBetrayer, 1, 4)
	}
	s.QuestsChanged(m)
	f.vileTeleport(player, altar)
}

// vileBooksRead counts the lair's used Books of Vileness (the central circle's _oVar5 minus 1).
func vileBooksRead(m *gridmap.GridMap[*Actor, *Item, Object]) (n int) {
	for _, obj := range m.Objects() {
		if o, ok := obj.(*Prop); ok && o.O.Kind == rules.ObjBOOK2L && !o.O.Active {
			n++
		}
	}
	return
}

// objectGroupUsed ports OperateLever's and BreakCrux's check: every object of the kind in group v is used.
func objectGroupUsed(m *gridmap.GridMap[*Actor, *Item, Object], kind func(int) bool, v int) bool {
	for _, obj := range m.Objects() {
		if o, ok := obj.(*Prop); ok && kind(o.O.Kind) && o.O.Var8 == v && o.O.Active {
			return false
		}
	}
	return true
}

// spawnQuestItem ports SpawnQuestItem at a random spot (randarea 5).
func (f *Factory) spawnQuestItem(m *gridmap.GridMap[*Actor, *Item, Object], b *rules.ItemData) {
	if p, ok := randomQuestSpot(m, nil); ok {
		m.AddItem(f.NewRolledItem(f.Generator.PlainItem(b)), p)
	}
}

// randomQuestSpot picks a random open cell (all 8 neighbours passable) that is reachable from the stairs up and
// away from them.
func randomQuestSpot(m *gridmap.GridMap[*Actor, *Item, Object], keep func(geometry.Point) bool) (geometry.Point, bool) {
	up := m.GetNamedLocation("ladder_up")
	cells := freeCellsNear(m, up, dungen.LevelSize*dungen.LevelSize, func(p geometry.Point) bool {
		_, isTransition := m.GetTransitionAt(p)
		return m.CanPlaceItemHere(p) && !m.IsObjectAt(p) && !isTransition &&
			geometry.DistanceChebyshev(p, up) > 10 && len(m.GetFilteredNeighbors(p, m.IsWalkable)) == 8 && (keep == nil || keep(p))
	})
	if len(cells) == 0 {
		return geometry.Point{}, false
	}
	return cells[rng.Intn(len(cells))], true
}

// questKill ports Poisoned Water's check of CheckQuests -- the set level is clean once its last monster dies,
// which is a whole-map question no condition can ask -- and then hands the death to the mod's on_kill triggers,
// which carry CheckQuestKill and M_DiabloDeath's quest.
func questKill(ctx ActionContext, mon *Monster) {
	state := ctx.GameState
	f := state.factory
	clean := mapClearedBy(state.GetMap(), mon)
	if state.onSetLevelOf(state.GetMap(), rules.QPWater) && state.QuestActive(rules.QPWater) != rules.QuestInit {
		if clean && state.QuestActive(rules.QPWater) != rules.QuestDone {
			f.completeQuest(rules.QPWater)
			f.PlaySfx("IS_QUESTDN", state.GetPlayer().MapPosition())
			state.mapGen.cleanWater(state.GetMap())
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
func mapClearedBy(m *gridmap.GridMap[*Actor, *Item, Object], mon *Monster) bool {
	for _, a := range m.Actors() {
		if a.monster != nil && a.monster != mon && !a.monster.Golem && a.IsAlive() {
			return false
		}
	}
	return true
}

// endTextWindow is the end_text(text) outcome: the mod's game is won, and texts.rec's text says so.
func (f *Factory) endTextWindow(id string) {
	PlayUICue("you_win")
	if f.Show != nil {
		f.Show(ShowText{strings.Split(f.State.text(id), "\n")})
	}
}

// questItemPickedUp ports CheckQuestItem: the mod's on_item_pickup triggers, named by the item picked up.
func (f *Factory) questItemPickedUp(it *rules.Item) {
	if it.Base != nil {
		f.fireQuestTriggers(EventItemPickup, it.Base.InternalName, "")
	}
}

// questDrop ports SpawnItem's Black Mushroom branch: once Adria has the mushroom, the next ordinary monster
// killed drops a brain instead of its loot.
func (f *Factory) questDrop(mon *Monster) *rules.Item {
	if q := rules.QMushroom; mon.Unique == nil && f.State.QuestActive(q) == rules.QuestActive && f.State.QuestVar(q, 1) == rules.QSMushGiven {
		f.State.SetQuestVar(q, 1, rules.QSBrainSpawned)
		return f.Generator.PlainItem(f.Generator.D.Item(rules.ItemBrain))
	}
	return nil
}

// operateQuestObject ports OperateObject for the quest objects; false for any other object.
func (f *Factory) operateQuestObject(o *Prop, player *Actor) bool {
	obj, s, gen, m := o.O, f.State, f.Generator, f.State.GetMap()
	sp := s.mapGen.setPiece(m)
	cell := func(px, py int) geometry.Point {
		return geometry.Point{X: px - dungen.CellOrigin, Y: py - dungen.CellOrigin}
	}
	switch obj.Kind {
	case rules.ObjMUSHPATCH: // OperateMushPatch
		if q := rules.QMushroom; s.QuestActive(q) != rules.QuestActive || s.QuestVar(q, 1) < rules.QSTomeGiven {
			f.PlayHeroSound(13)
			f.Msg("I can't open this yet.")
		} else if obj.Active {
			obj.Active = false
			f.PlaySfx("IS_CHEST", o.pos)
			f.dropItem(gen.PlainItem(gen.D.Item(rules.ItemMushroom)), o.pos)
			s.SetQuestVar(q, 1, rules.QSMushSpawned)
		}
	case rules.ObjSIGNCHEST: // OperateInnSignChest
		if s.QuestVar(rules.QLtBanner, 1) != 2 {
			f.PlayHeroSound(24)
			f.Msg("I can't open this yet.")
		} else if obj.Active {
			obj.Active = false
			f.PlaySfx("IS_CHEST", o.pos)
			f.dropItem(gen.PlainItem(gen.D.Item(rules.ItemBanner)), o.pos)
		}
	case rules.ObjBLINDBOOK, rules.ObjBLOODBOOK, rules.ObjSTEELTOME: // OperateBookLever
		q := map[int]int{rules.ObjBLINDBOOK: rules.QBlind, rules.ObjBLOODBOOK: rules.QBlood, rules.ObjSTEELTOME: rules.QWarlord}[obj.Kind]
		if obj.Active {
			obj.Active = false
			if s.QuestVar(q, 1) == 0 {
				s.startQuest(q)
				s.SetQuestVar(q, 1, 1)
				if q == rules.QBlood {
					for _, c := range [3][2]int{{19, 26}, {31, 26}, {25, 33}} {
						m.AddItem(f.NewRolledItem(gen.PlainItem(gen.D.Item(rules.ItemBldStone))), cell(2*sp[0]+c[0], 2*sp[1]+c[1]))
					}
				}
			}
			s.QuestsChanged(m)
			if q == rules.QBlind {
				f.dropItem(gen.SpawnUnique(gen.D.UniqueItemIdx(rules.UItemOptAmulet), s.CurrentLevel().Depth), cell(2*sp[0]+16+5, 2*sp[1]+16+5))
			}
		}
		f.say(o.Name(), s.QuestDefs()[q].LogText)
	case rules.ObjPEDISTAL: // OperatePedistal
		q := rules.QBlood
		stones := s.QuestVar(q, 2)
		switch {
		case stones >= 3:
			f.Msg("The pedestal is full. Its three blood stones are set.")
		case !takeItem(player, rules.ItemBldStone):
			f.Msg(fmt.Sprintf("Three hollows are cut into the pedestal; %d still wait for a blood stone.", 3-stones))
		default:
			s.SetQuestVar(q, 2, stones+1)
			f.PlaySfx(map[bool]string{false: "LS_PUDDLE", true: "LS_BLODSTAR"}[stones+1 == 3], o.pos)
			s.QuestsChanged(m)
			if stones+1 < 3 {
				f.Msg(fmt.Sprintf("You set a blood stone into the pedestal. %d to go.", 3-(stones+1)))
				break
			}
			obj.Active = false
			f.Msg("The last blood stone sinks home. The pedestal drinks, and the hall below lights up.")
			f.dropItem(gen.SpawnUnique(gen.D.UniqueItemIdx(rules.UItemArmOfVal), s.CurrentLevel().Depth), cell(2*sp[0]+25, 2*sp[1]+19))
		}
	case rules.ObjBOOK2R: // OperateSChambBk
		if q := rules.QSchamb; obj.Active {
			obj.Active = false
			if s.QuestActive(q) == rules.QuestInit {
				s.startQuest(q)
			}
			s.QuestsChanged(m)
		}
		f.say(o.Name(), s.QuestDefs()[rules.QSchamb].LogText)
	case rules.ObjBOOK2L: // OperateBook
		if s.onSetLevelOf(m, rules.QBetrayer) {
			f.operateVileBook(o, player)
		} else if obj.Active && s.onSetLevelOf(m, rules.QSchamb) {
			obj.Active = false
			h := player.hero
			h.SpellLevels[rules.SplGuardian] = min(h.SpellLevels[rules.SplGuardian]+1, 15)
			f.RefreshSpells(player)
			f.completeQuest(rules.QSchamb)
			f.PlaySfx("IS_QUESTDN", o.pos)
			f.Msg("Arcane knowledge gained!")
			f.ActionBroker.Execute(NewSpellAction(player, rules.SplGuardian, h.SpellLevels[rules.SplGuardian], o.pos.Add(geometry.Point{X: -2, Y: -4})))
		}
	case rules.ObjBOOKCASEL, rules.ObjBOOKCASER: // OperateBookCase: a talker already aware of the hero may have had enough
		if obj.Active {
			for _, a := range m.Actors() {
				if mon := a.monster; mon != nil && mon.IsTalker(f.Generator.D) && a.IsAlive() && mon.Squelch == 255 {
					f.fireTalker(EventTalkerDisturbed, a, false)
				}
			}
		}
		return false
	default:
		return false
	}
	return true
}

// GeneratedLevel is the generated D1 level behind a built map, nil for any other.
func (s *State) GeneratedLevel(m *gridmap.GridMap[*Actor, *Item, Object]) *dungen.Level {
	return s.mapGen.generated[m.GetName()]
}

// MonsterGreeting ports ProcessMonsters' first-sight lines (the Butcher's "fresh meat"): enemies.rec's sight_sfx.
func (f *Factory) MonsterGreeting(a *Actor) {
	if id := a.monster.Data.SightSfx; id != "" {
		f.playSound(id)
	}
}
