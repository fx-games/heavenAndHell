package common

import (
	"fmt"

	"github.com/expr-lang/expr"

	"rpg/d1"
	"rpg/defs"
	"rpg/recfile"
)

// Named quest events. A trigger is only ever evaluated when one of these happens to a named subject: nothing
// here is polled, and there is no general "on tick".
const (
	EventKill         = "on_kill"          // subject: the dead monster's internal name, or a unique's name
	EventItemPickup   = "on_item_pickup"   // subject: the item's internal name
	EventEnterLevel   = "on_enter_level"   // subject: the dungeon depth the hero arrived on
	EventTalk         = "on_talk"          // subject: the NPC's internal name
	EventLevelCleared = "on_level_cleared" // subject: the map name; its last monster just died
)

// QuestEvents are the events a trigger may name, for the loader's check and for anyone adding one.
var QuestEvents = []string{EventKill, EventItemPickup, EventEnterLevel, EventTalk, EventLevelCleared}

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
		Depth:  d1DungeonLevel(s),
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
		UpdateD1Quests(s, s.GetMap())
	case "say": // say(text) speaks as whoever the event is about; say(text, name) names the speaker
		if do.ParamCount() > 1 {
			speaker = do.GetString(1)
		}
		f.say(speaker, do.GetString(0))
	case "spawn_item": // spawn_item(item, map, location): a plain item of the mod's, lying at a named spot
		f.spawnItemAt(do.GetString(0), do.GetString(1), do.GetString(2))
	case "end_text":
		f.d1EndTextWindow()
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
		"say", "refresh_level", "end_text", "give_exp", "give_weapon", "spawn_item":
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
	m := s.GetTownMap(mapName)
	if m == nil {
		m = s.GetDungeonMap(mapName)
	}
	if m == nil {
		f.Msg(fmt.Sprintf("spawn_item: map %q is not built", mapName))
		return
	}
	m.AddItem(f.NewD1Item(f.Generator.PlainItem(idx)), getDropPos(m, m.GetNamedLocation(location)))
}
