package common

import (
	"fmt"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"rpg/src/d1"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/recfile"
)

// Named quest events. A trigger is only ever evaluated when one of these happens to a named subject: nothing
// here is polled, and there is no general "on tick".
const (
	EventKill         = "on_kill"          // subject: the dead monster's internal name, or a unique's name
	EventItemPickup   = "on_item_pickup"   // subject: the item's internal name
	EventEnterLevel   = "on_enter_level"   // subject: the id of the level the hero arrived on
	EventTalk         = "on_talk"          // subject: the NPC's internal name
	EventLevelCleared = "on_level_cleared" // subject: the map name; its last monster just died
	EventItemUse      = "on_item_use"      // subject: the item's internal name
	EventApproach     = "on_approach"      // subject: the NPC's internal name, when the hero walks up to it
)

// QuestEvents are the events a trigger may name, for the loader's check and for anyone adding one.
var QuestEvents = []string{EventKill, EventItemPickup, EventEnterLevel, EventTalk, EventLevelCleared, EventItemUse, EventApproach}

// questEnv is what a trigger condition may ask about: quests by key, raw flags, and where the hero is.
func (s *State) questEnv() defs.QuestEnv {
	state := func(key string) int {
		if q := s.QuestByKey(key); q >= 0 {
			return s.QuestActive(q)
		}
		return d1.QuestNotAvail
	}
	qvar := func(n int) func(string) int {
		return func(key string) int {
			if q := s.QuestByKey(key); q >= 0 {
				return s.QuestVar(q, n)
			}
			return 0
		}
	}
	return defs.QuestEnv{
		Done:   func(key string) bool { return state(key) == d1.QuestDone },
		Active: func(key string) bool { return state(key) == d1.QuestActive },
		Avail:  func(key string) bool { return state(key) != d1.QuestNotAvail },
		Logged: func(key string) bool { q := s.QuestByKey(key); return q >= 0 && s.QuestLog(q) },
		Flag:   s.GetFlags().IsSet,
		V1:     qvar(1),
		V2:     qvar(2),
		Depth:  s.CurrentLevel().Depth,
		Map:    s.CurrentLevel().Map,

		Deepest: s.Deepest(),
		Has: func(item string) bool {
			idx, ok := s.factory.D1().D.ItemIdx[item]
			return ok && d1HasItem(s.GetPlayer(), idx) != nil
		},
	}
}

// fireQuestTriggers runs the mod's triggers for one named event and subject, in file order: each one whose
// condition holds runs its outcomes. Conditions were compiled when the mod loaded; this only evaluates them.
// speaker is who a say() outcome speaks as; empty for events nobody speaks at. It reports whether any fired.
func (f *Factory) fireQuestTriggers(event, subject, speaker string) bool {
	fired := false
	s := f.State
	env := s.questEnv()
	for _, t := range s.factory.D1().D.QuestTriggers {
		if t.Event != event || t.Subject != subject {
			continue
		}
		if p := t.Cond(); p != nil {
			out, err := expr.Run(p, env)
			if err != nil {
				f.Msg(fmt.Sprintf("quest trigger %s/%s: %v", event, subject, err))
				continue
			}
			if ok, _ := out.(bool); !ok {
				continue
			}
		}
		fired = true
		for _, do := range t.Do {
			if !f.runQuestOutcome(do, speaker) {
				f.Msg(fmt.Sprintf("quest trigger %s/%s: no outcome named %q", event, subject, do.Name()))
			}
		}
		if (event == EventTalk || event == EventApproach) && !t.Pass {
			break // one conversation, one thing said
		}
		env = s.questEnv() // later triggers see what earlier ones did
	}
	return fired
}

// runQuestOutcome performs one named outcome; false if the engine has no such outcome. This list is the whole
// vocabulary a mod's triggers have -- outcomes are engine behaviour, never expressions.
func (f *Factory) runQuestOutcome(do recfile.StringPredicate, speaker string) bool {
	s := f.State
	quest := func(i int) int { return s.QuestByKey(do.GetString(i)) }
	switch do.Name() {
	case "complete_quest":
		if q := quest(0); q >= 0 {
			f.completeD1Quest(q)
		}
	case "activate_quest":
		if q := quest(0); q >= 0 {
			s.SetQuestActive(q, d1.QuestActive)
		}
	case "log_quest":
		if q := quest(0); q >= 0 {
			s.GetFlags().SetTrue(s.QuestFlag(q) + ".log")
		}
	case "set_quest_var":
		if q := quest(0); q >= 0 {
			s.SetQuestVar(q, do.GetInt(1), do.GetInt(2))
		}
	case "set_flag":
		s.GetFlags().SetTrue(do.GetString(0))
	case "hero_sound":
		f.PlayD1HeroSound(do.GetInt(0))
	case "sound":
		f.playD1Sound(do.GetString(0))
	case "refresh_level":
		s.QuestsChanged(s.GetMap())
	case "place_item": // place_item(item, location): on the level the hero is on; "anywhere" is a random open spot
		f.placeItem(do.GetString(0), do.GetString(1))
	case "say": // say(text) speaks as whoever the event is about; say(text, name) names the speaker
		if do.ParamCount() > 1 {
			speaker = do.GetString(1)
		}
		f.say(speaker, do.GetString(0))
	case "start_quest": // logged and active
		if q := quest(0); q >= 0 {
			s.startQuest(q)
		}
	case "start_town_quest": // logged; active with v1 1 only if it had not started
		if q := quest(0); q >= 0 {
			s.startTownQuest(q)
		}
	case "or_quest_var": // or_quest_var(quest, n, bits): sets bits in a quest variable
		if q := quest(0); q >= 0 {
			s.SetQuestVar(q, do.GetInt(1), s.QuestVar(q, do.GetInt(1))|do.GetInt(2))
		}
	case "take_item": // take_item(item): out of the hero's pack
		if idx, ok := f.D1().D.ItemIdx[do.GetString(0)]; ok {
			d1TakeItem(s.GetPlayer(), idx)
		}
	case "drop_item": // drop_item(item): a plain one at the speaker's feet
		if idx, ok := f.D1().D.ItemIdx[do.GetString(0)]; ok {
			f.dropD1Item(f.Generator.PlainItem(idx), f.talkAt)
		}
	case "drop_unique": // drop_unique(name): the unique item of that name at the speaker's feet
		for uid, u := range f.D1().D.UniqueItems {
			if u.Name == do.GetString(0) {
				f.dropD1Item(f.Generator.SpawnUnique(uid, 0), f.talkAt)
				break
			}
		}
	case "spawn_item": // spawn_item(item, map, location), or spawn_item(item, here): a plain item of the mod's
		location := ""
		if do.ParamCount() > 2 {
			location = do.GetString(2)
		}
		f.spawnItemAt(do.GetString(0), do.GetString(1), location)
	case "spawn_monster": // spawn_monster(monster, location): on the hero's level, at a named location or "anywhere"
		f.spawnMonster(do.GetString(0), do.GetString(1))
	case "add_npc": // add_npc(npc, map, location): a townsperson drawn somewhere in maps.ldtk, next to location
		f.addNPC(do.GetString(0), do.GetString(1), do.GetString(2))
	case "remove_npc": // remove_npc(npc, map)
		f.removeNPC(do.GetString(0), do.GetString(1))
	case "end_text": // end_text(text): the game is won
		f.endTextWindow(do.GetString(0))
	case "give_exp": // at the hero's own level, so exactly the amount named
		if p := s.GetPlayer(); p.hero != nil {
			f.grantExperience(p, p.hero.Level, do.GetInt(0))
			f.Msg(fmt.Sprintf("You gain %d experience.", do.GetInt(0)))
		}
	case "give_weapon":
		if it := f.Generator.WeaponItem(do.GetInt(0)); it != nil {
			item := f.NewD1Item(it)
			s.GetPlayer().GetInventory().Add(item)
			f.Msg(fmt.Sprintf("You receive %s.", item.Name()))
		}
	default:
		return false
	}
	return true
}

// QuestOutcomeExists reports whether the engine implements an outcome by that name, without running it. The
// loader's companion check: every outcome a mod names has to resolve.
func QuestOutcomeExists(name string) bool {
	switch name {
	case "complete_quest", "activate_quest", "log_quest", "set_quest_var", "set_flag", "hero_sound", "sound",
		"say", "refresh_level", "place_item", "end_text", "give_exp", "give_weapon", "spawn_item", "start_quest", "start_town_quest",
		"or_quest_var", "take_item", "drop_item", "drop_unique", "spawn_monster", "add_npc", "remove_npc":
		return true
	}
	return false
}

// spawnItemAt lays one plain item of the mod's at a named location of a map already built (the town always is).
func (f *Factory) spawnItemAt(item, mapName, location string) {
	idx, ok := f.Generator.D.ItemIdx[item]
	if !ok {
		f.Msg(fmt.Sprintf("spawn_item: this mod has no item %q", item))
		return
	}
	s := f.State
	if mapName == "here" { // where the hero stands
		m := s.GetMap()
		m.AddItem(f.NewD1Item(f.Generator.PlainItem(idx)), getDropPos(m, s.GetPlayer().MapPosition()))
		return
	}
	m := s.Map(mapName)
	if m == nil {
		f.Msg(fmt.Sprintf("spawn_item: map %q is not built", mapName))
		return
	}
	m.AddItem(f.NewD1Item(f.Generator.PlainItem(idx)), getDropPos(m, m.GetNamedLocation(location)))
}

// spawnMonster puts one monster of the mod's on the level the hero is on: next to a named location, or "anywhere"
// for a random open spot away from the stairs up, as place_item does.
func (f *Factory) spawnMonster(id, location string) {
	a := f.NewMonsterByID(id)
	if a == nil {
		f.Msg(fmt.Sprintf("spawn_monster: this mod has no monster %q", id))
		return
	}
	m := f.State.GetMap()
	var p geometry.Point
	if location == "anywhere" {
		spot, ok := randomQuestSpot(m, m.CanPlaceActorHere)
		if !ok {
			return
		}
		p = spot
	} else if cells := freeCellsNear(m, m.GetNamedLocation(location), 1, m.CanPlaceActorHere); len(cells) > 0 {
		p = cells[0]
	} else {
		return
	}
	a.d1Monster.SpawnIndex = -1 // saved in LevelSnapshot.Spawned
	a.SetDrawPosition(p.ToPointF())
	m.AddActor(a, p)
	f.State.OnActorSpawned(a)
}

// placeItem lays one plain item of the mod's on the level the hero is on: at a named location, or "anywhere"
// for a random open spot away from the stairs (ResyncQuests' SpawnQuestItem).
func (f *Factory) placeItem(item, location string) {
	idx, ok := f.Generator.D.ItemIdx[item]
	if !ok {
		f.Msg(fmt.Sprintf("place_item: this mod has no item %q", item))
		return
	}
	m := f.State.GetMap()
	if location == "anywhere" {
		f.spawnD1QuestItem(m, idx)
		return
	}
	m.AddItem(f.NewD1Item(f.Generator.PlainItem(idx)), getDropPos(m, m.GetNamedLocation(location)))
}

// ItemUsed fires on_item_use for an item the hero just used, named by its record.
func (s *State) ItemUsed(item *Item) {
	if item == nil || item.d1 == nil || s.factory == nil {
		return
	}
	if items := s.factory.D1().D.Items; item.d1.BaseIdx >= 0 && item.d1.BaseIdx < len(items) {
		s.factory.fireQuestTriggers(EventItemUse, items[item.d1.BaseIdx].InternalName, "")
	}
}

// questConditions caches compiled conditions that live outside quests.rec (levels.rec's open_when).
var questConditions = map[string]*vm.Program{}

// compileQuestCondition compiles one condition in the trigger language.
func compileQuestCondition(cond string) (*vm.Program, error) {
	if p, ok := questConditions[cond]; ok {
		return p, nil
	}
	p, err := expr.Compile(cond, expr.Env(defs.QuestEnv{}), expr.AsBool())
	if err != nil {
		return nil, err
	}
	questConditions[cond] = p
	return p, nil
}

// questCondition evaluates cond against the quests as they stand; a condition that fails to run is false.
func (f *Factory) questCondition(cond string) bool {
	p, err := compileQuestCondition(cond)
	if err != nil {
		return false
	}
	out, err := expr.Run(p, f.State.questEnv())
	ok, _ := out.(bool)
	return err == nil && ok
}
