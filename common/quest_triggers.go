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
	EventKill       = "on_kill"        // subject: the dead monster's internal name, or a unique's name
	EventItemPickup = "on_item_pickup" // subject: the item's internal name
	EventEnterLevel = "on_enter_level" // subject: the dungeon depth the hero arrived on
	EventTalk       = "on_talk"        // subject: the NPC's internal name
)

// QuestEvents are the events a trigger may name, for the loader's check and for anyone adding one.
var QuestEvents = []string{EventKill, EventItemPickup, EventEnterLevel, EventTalk}

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
func (f *Factory) fireQuestTriggers(event, subject string) {
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
		for _, do := range t.Do {
			if !f.runQuestOutcome(do) {
				f.Msg(fmt.Sprintf("quest trigger %s/%s: no outcome named %q", event, subject, do.Name()))
			}
		}
		env = s.questEnv() // later triggers see what earlier ones did
	}
}

// runQuestOutcome performs one named outcome; false if the engine has no such outcome. This list is the whole
// vocabulary a mod's triggers have -- outcomes are engine behaviour, never expressions.
func (f *Factory) runQuestOutcome(do recfile.StringPredicate) bool {
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
	case "end_text":
		f.d1EndTextWindow()
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
		"refresh_level", "end_text":
		return true
	}
	return false
}
