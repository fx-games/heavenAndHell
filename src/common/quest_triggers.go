package common

import (
	"fmt"
	"slices"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/recfile"
	"rpg/src/rules"
	"rpg/src/stats"
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
	// A quest boss who talks before it fights (monsters.rec's talker AIs); subject: its AI. It is set up when its
	// level is built (spawn), the hero walks into it (talk, before it speaks), it has spoken (said), and every
	// turn it thinks (turn).
	EventTalkerSpawn = "on_talker_spawn"
	EventTalkerTalk  = "on_talker_talk"
	EventTalkerSaid  = "on_talker_said"
	EventTalkerTurn  = "on_talker_turn"
	// EventTalkerDisturbed: the hero reads a bookcase on its level while it is aware of the hero.
	EventTalkerDisturbed = "on_talker_disturbed"
)

// QuestEvents are the events a trigger may name, for the loader's check and for anyone adding one.
var QuestEvents = []string{EventKill, EventItemPickup, EventEnterLevel, EventTalk, EventLevelCleared, EventItemUse, EventApproach,
	EventTalkerSpawn, EventTalkerTalk, EventTalkerSaid, EventTalkerTurn, EventTalkerDisturbed}

// questEnv is what a trigger condition may ask about: quests by key, raw flags, and where the hero is.
func (s *State) questEnv() defs.QuestEnv {
	state := func(key string) int {
		if q := s.QuestByKey(key); q >= 0 {
			return s.QuestActive(q)
		}
		return rules.QuestNotAvail
	}
	qvar := func(n int) func(string) int {
		return func(key string) int {
			if q := s.QuestByKey(key); q >= 0 {
				return s.QuestVar(q, n)
			}
			return 0
		}
	}
	env := defs.QuestEnv{
		Done:   func(key string) bool { return state(key) == rules.QuestDone },
		Active: func(key string) bool { return state(key) == rules.QuestActive },
		Avail:  func(key string) bool { return state(key) != rules.QuestNotAvail },
		Logged: func(key string) bool { q := s.QuestByKey(key); return q >= 0 && s.QuestLog(q) },
		Flag:   s.GetFlags().IsSet,
		V1:     qvar(1),
		V2:     qvar(2),
		Depth:  s.CurrentLevel().Depth,
		Map:    s.CurrentLevel().Map,

		Deepest: s.Deepest(),
		Has: func(item string) bool {
			return hasItem(s.GetPlayer(), item) != nil
		},
		At: func(name string) bool {
			p, ok := namedLocation(s.GetMap(), name)
			return ok && s.GetPlayer().MapPosition() == p
		},
		Here: func(key string) bool { q := s.QuestByKey(key); return q >= 0 && s.questHere(q) },
	}
	if a := s.factory.talker; a != nil {
		env.Talk, env.Goal, env.Visible = a.monster.TalkMsg, goalNames[a.monster.Goal], s.factory.talkerVisible
		if env.Goal == "" {
			env.Goal = "other"
		}
	}
	return env
}

// fireQuestTriggers runs the mod's triggers for one named event and subject. It works in rounds, so the order
// triggers sit in the file never matters: each round checks every trigger that has not fired yet against the same
// state, then runs the outcomes of all that hold; the next round sees what they did (a hand-in can make the next
// step true). Each trigger fires at most once per event. A talk (conversationEvent) says one thing: it ends with
// the round its line fired in, and two lines holding in one round are a content error, reported. Its pass
// triggers (hand-ins, log entries) run in rounds of their own before the line is picked. speaker is who
// a say() outcome speaks as; empty for events nobody speaks at. It reports whether any fired.
func (f *Factory) fireQuestTriggers(event, subject, speaker string) bool {
	triggers := f.Gen().D.QuestTriggers
	done := make([]bool, len(triggers))
	firedAny := false
	for {
		env := f.State.questEnv()
		var round []int
		lines := 0
		for i, t := range triggers {
			if done[i] || t.Event != event || t.Subject != subject || !f.triggerHolds(t, env) {
				continue
			}
			round = append(round, i)
			if !t.Pass {
				lines++
			}
		}
		if len(round) == 0 {
			return firedAny
		}
		if conversationEvent(event) && lines < len(round) {
			// hand-ins first: the line is picked in a later round, from what they left
			round = slices.DeleteFunc(round, func(i int) bool { return !triggers[i].Pass })
			lines = 0
		}
		if conversationEvent(event) && lines > 1 {
			f.Msg(fmt.Sprintf("quest trigger %s/%s: %d lines hold at once", event, subject, lines))
		}
		// triggers holding together run in the order of their own text, so not even that comes from the file
		slices.SortFunc(round, func(a, b int) int { return strings.Compare(triggerText(triggers[a]), triggerText(triggers[b])) })
		for _, i := range round {
			done[i] = true
			for _, do := range triggers[i].Do {
				if !f.runQuestOutcome(do, speaker) {
					f.Msg(fmt.Sprintf("quest trigger %s/%s: no outcome named %q", event, subject, do.Name()))
				}
			}
		}
		firedAny = true
		if conversationEvent(event) && lines > 0 {
			return true // one conversation, one thing said; a talker's state moves one step
		}
	}
}

func triggerText(t defs.Trigger) string {
	var b strings.Builder
	b.WriteString(t.When)
	for _, do := range t.Do {
		b.WriteString("\n" + do.Name())
		for i := range do.ParamCount() {
			b.WriteString(" " + do.GetString(i))
		}
	}
	return b.String()
}

// triggerHolds evaluates a trigger's condition; one that fails to run is reported and doesn't hold.
func (f *Factory) triggerHolds(t defs.Trigger, env defs.QuestEnv) bool {
	p := t.Cond()
	if p == nil {
		return true
	}
	out, err := expr.Run(p, env)
	if err != nil {
		f.Msg(fmt.Sprintf("quest trigger %s/%s: %v", t.Event, t.Subject, err))
		return false
	}
	ok, _ := out.(bool)
	return ok
}

// conversationEvent: the events where the hero is told one thing (a talk, walking up to someone, a talker's
// talk state moving one step).
func conversationEvent(event string) bool {
	return event == EventTalk || event == EventApproach || talkerEvent(event)
}

func talkerEvent(event string) bool {
	return event == EventTalkerSpawn || event == EventTalkerTalk || event == EventTalkerSaid || event == EventTalkerTurn ||
		event == EventTalkerDisturbed
}

// runQuestOutcome performs one named outcome; false if the engine has no such outcome. This list is the whole
// vocabulary a mod's triggers have -- outcomes are engine behaviour, never expressions.
func (f *Factory) runQuestOutcome(do recfile.StringPredicate, speaker string) bool {
	s := f.State
	quest := func(i int) int { return s.QuestByKey(do.GetString(i)) }
	switch do.Name() {
	case "complete_quest":
		if q := quest(0); q >= 0 {
			f.completeQuest(q)
		}
	case "activate_quest":
		if q := quest(0); q >= 0 {
			s.SetQuestActive(q, rules.QuestActive)
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
		f.PlayHeroSound(do.GetInt(0))
	case "sound":
		f.playSound(do.GetString(0))
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
		takeItem(s.GetPlayer(), do.GetString(0))
	case "drop_item": // drop_item(item): a plain one at the speaker's feet
		if b, ok := f.Gen().D.ItemsByName[do.GetString(0)]; ok {
			f.dropItem(f.Generator.PlainItem(b), f.talkAt)
		}
	case "drop_unique": // drop_unique(id): the unique item with that id at the speaker's feet
		if uid := f.Gen().D.UniqueItemIdx(do.GetString(0)); uid >= 0 {
			f.dropItem(f.Generator.SpawnUnique(uid, s.CurrentLevel().Depth), f.talkAt)
		}
	case "spawn_item": // spawn_item(item, map, location), or spawn_item(item, here): a plain item of the mod's
		location := ""
		if do.ParamCount() > 2 {
			location = do.GetString(2)
		}
		f.spawnItemAt(do.GetString(0), do.GetString(1), location)
	case "spawn_monster": // spawn_monster(monster, location): on the hero's level, at a named location or "anywhere"
		f.spawnMonster(do.GetString(0), do.GetString(1))
	case "add_npc": // add_npc(npc, map, location): a townsperson drawn on some map, next to location
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
			item := f.NewRolledItem(it)
			s.GetPlayer().GetInventory().Add(item)
			f.Msg(fmt.Sprintf("You receive %s.", item.Name()))
		}
	case "talk", "silence", "quiet", "stop_talking", "set_goal", "speak", "drop_monster_item",
		"drop_misc_item", "kill_talker":
		return f.talkerOutcome(do)
	default:
		return false
	}
	return true
}

// talkerOutcome performs the outcomes only an on_talker_* event has: they change the quest boss' talk state.
func (f *Factory) talkerOutcome(do recfile.StringPredicate) bool {
	a := f.talker
	if a == nil {
		return false
	}
	mon := a.monster
	switch do.Name() {
	case "talk": // talk(text): it has that to say, and waits for the hero
		mon.TalkMsg, mon.Goal = do.GetString(0), goalInquiring
	case "silence": // nothing left to say, goal unchanged
		mon.TalkMsg = ""
	case "quiet": // done talking: it fights, already aware of the hero
		mon.Squelch, mon.TalkMsg, mon.Goal = 255, "", goalNormal
	case "stop_talking":
		mon.TalkMsg, mon.Goal = "", goalNormal
	case "set_goal": // set_goal(inquiring)
		mon.Goal = goalInquiring
	case "speak": // it says its line now (M_DoTalk)
		f.doTalk(a)
	case "drop_monster_item": // what the monster itself would drop (SpawnItem)
		dlvl := f.State.CurrentLevel().Depth
		if it := f.Generator.MonsterDrop(mon.Data, mon.Level, true, dlvl, false); it != nil {
			f.dropItem(it, a.MapPosition())
		}
	case "drop_misc_item": // drop_misc_item(IMISC_x): a random misc item of that kind (SpawnTypeItem)
		f.dropItem(f.Generator.TypeItem(f.State.CurrentLevel().Depth, rules.ITypeMisc, do.GetString(0), false), a.MapPosition())
	case "kill_talker": // M_StartKill(i, -1)
		f.ActionBroker.Execute(NewChangeResourceAction(AbstractCause(ActionName(a.Name()+"'s rest")), a, stats.Health, -1e6))
	default:
		return false
	}
	return true
}

// fireTalker runs a talker event for a quest boss.
func (f *Factory) fireTalker(event string, a *Actor, visible bool) {
	prev, prevVisible, prevAt := f.talker, f.talkerVisible, f.talkAt
	f.talker, f.talkerVisible, f.talkAt = a, visible, a.MapPosition()
	f.fireQuestTriggers(event, a.monster.AI, a.Name())
	f.talker, f.talkerVisible, f.talkAt = prev, prevVisible, prevAt
}

// QuestOutcomeExists reports whether the engine implements an outcome by that name, without running it. The
// loader's companion check: every outcome a mod names has to resolve.
func QuestOutcomeExists(name string) bool {
	switch name {
	case "complete_quest", "activate_quest", "log_quest", "set_quest_var", "set_flag", "hero_sound", "sound",
		"say", "refresh_level", "place_item", "end_text", "give_exp", "give_weapon", "spawn_item", "start_quest", "start_town_quest",
		"or_quest_var", "take_item", "drop_item", "drop_unique", "spawn_monster", "add_npc", "remove_npc",
		"talk", "silence", "quiet", "stop_talking", "set_goal", "speak", "drop_monster_item", "drop_misc_item", "kill_talker":
		return true
	}
	return false
}

// spawnItemAt lays one plain item of the mod's at a named location of a map already built (the town always is).
func (f *Factory) spawnItemAt(item, mapName, location string) {
	b, ok := f.Generator.D.ItemsByName[item]
	if !ok {
		f.Msg(fmt.Sprintf("spawn_item: this mod has no item %q", item))
		return
	}
	s := f.State
	if mapName == "here" { // where the hero stands
		m := s.GetMap()
		m.AddItem(f.NewRolledItem(f.Generator.PlainItem(b)), getDropPos(m, s.GetPlayer().MapPosition()))
		return
	}
	m := s.Map(mapName)
	if m == nil {
		f.Msg(fmt.Sprintf("spawn_item: map %q is not built", mapName))
		return
	}
	m.AddItem(f.NewRolledItem(f.Generator.PlainItem(b)), getDropPos(m, m.GetNamedLocation(location)))
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
	a.monster.SpawnIndex = -1 // saved in LevelSnapshot.Spawned
	a.SetDrawPosition(p.ToPointF())
	m.AddActor(a, p)
	f.State.OnActorSpawned(a)
	// its group_spawn escort comes along, as it does for a monster the level was built with
	data, _ := f.monsterByID(id)
	companions := f.groupSpawnOf(data, rng)
	for i, spot := range freeCellsNear(m, p, len(companions), m.CanPlaceActorHere) {
		if c := f.NewMonsterByID(companions[i].ID); c != nil {
			c.monster.SpawnIndex = -1
			c.SetDrawPosition(spot.ToPointF())
			m.AddActor(c, spot)
			f.State.OnActorSpawned(c)
		}
	}
}

// placeItem lays one plain item of the mod's on the level the hero is on: at a named location, or "anywhere"
// for a random open spot away from the stairs (ResyncQuests' SpawnQuestItem).
func (f *Factory) placeItem(item, location string) {
	b, ok := f.Generator.D.ItemsByName[item]
	if !ok {
		f.Msg(fmt.Sprintf("place_item: this mod has no item %q", item))
		return
	}
	m := f.State.GetMap()
	if location == "anywhere" {
		f.spawnQuestItem(m, b)
		return
	}
	m.AddItem(f.NewRolledItem(f.Generator.PlainItem(b)), getDropPos(m, m.GetNamedLocation(location)))
}

// ItemUsed fires on_item_use for an item the hero just used, named by its record.
func (s *State) ItemUsed(item *Item) {
	if item == nil || item.roll == nil || s.factory == nil {
		return
	}
	if item.roll.Base != nil {
		s.factory.fireQuestTriggers(EventItemUse, item.roll.Base.InternalName, "")
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
