package defs

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"rpg/src/recfile"
)

// Quest is one quest's content: its name, the text its log shows, the level it belongs to and what each town
// NPC says about it. The quest's *logic* stays in the engine; this record holds only what a mod authors.
//
// Key is the quest's stable id. Everything the engine remembers about the quest hangs off it in the Flags
// store: "quest:<key>" once done, ".active", ".log", ".v1"/".v2".
type Quest struct {
	Key     string
	Name    string
	LogText string // TEXT_* id the quest log shows

	// Level is the id (levels.rec) of the level the quest belongs to; its set level, if any, is a levels.rec
	// record of its own that names this quest.
	Level string

	// Gossip is town NPC internal name -> the TEXT_* id it says about this quest while the quest is running.
	Gossip map[string]string
}

// Trigger fires named outcomes when a named event happens to a named subject and its condition holds.
//
// Event is one of the engine's named events (on_kill, on_item_pickup, on_enter_level, on_talk, on_level_cleared, on_item_use) -- never a poll.
// Subject is what the event is about, matched verbatim: a monster's internal name or unique name, an item's
// internal name, a dungeon depth, an NPC. When is an expr-lang expression over the flag store, compiled once at
// load. Do is an ordered list of engine-implemented outcomes in predicate form, e.g. "complete_quest(blind)";
// the engine names them, a mod never writes behaviour.
type Trigger struct {
	Event   string
	Subject string
	When    string
	Do      []recfile.StringPredicate
	// Pass lets a conversation go on after this trigger spoke. A talk (on_talk, on_approach) is otherwise one
	// thing said: the first trigger that fires ends it, as TalkToTowner's return does.
	Pass bool

	program *vm.Program
}

// Cond is the compiled condition, nil when the trigger has none (it always fires).
func (t Trigger) Cond() *vm.Program { return t.program }

// QuestEnv is everything a trigger condition may ask about. Functions only: flag keys are not identifiers, and
// a mod should name quests, not flag spellings.
type QuestEnv struct {
	Done   func(string) bool `expr:"done"`   // the quest is finished
	Active func(string) bool `expr:"active"` // the quest is running
	Avail  func(string) bool `expr:"avail"`  // this game has the quest at all
	Logged func(string) bool `expr:"logged"` // the quest is in the quest log
	Flag   func(string) bool `expr:"flag"`   // a raw flag, for anything that is not a quest
	V1     func(string) int  `expr:"v1"`
	V2     func(string) int  `expr:"v2"`
	Depth  int               `expr:"depth"` // the dungeon level the hero is on
	Map    string            `expr:"map"`   // the levels.rec id of the level the hero is on
	// Deepest is the deepest dungeon level the hero has reached; Has whether the hero carries an item, by name.
	Deepest int               `expr:"deepest"`
	Has     func(string) bool `expr:"has"`
}

// ReadQuests loads the active mod's quests and triggers and compiles every trigger condition.
func ReadQuests(fsys fs.FS) ([]Quest, []Trigger, error) {
	f, err := fsys.Open(path.Join("assets", "data", "quests.rec"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil // a mod without quests is a mod without quests
	}
	if err != nil {
		return nil, nil, fmt.Errorf("quests.rec: %w", err)
	}
	defer f.Close()
	all := recfile.ReadMulti(f)

	var quests []Quest
	for i, rec := range all["quest"] {
		q := Quest{Gossip: map[string]string{}}
		for _, fld := range rec {
			switch fld.Name {
			case "key":
				q.Key = fld.Value
			case "name":
				q.Name = fld.Value
			case "log_text":
				q.LogText = fld.Value
			case "level":
				q.Level = fld.Value
			case "gossip":
				npc, id, ok := strings.Cut(fld.Value, " ")
				if !ok {
					return nil, nil, fmt.Errorf("quests.rec quest %d: gossip wants \"<npc> <text id>\", got %q", i+1, fld.Value)
				}
				q.Gossip[npc] = strings.TrimSpace(id)
			default:
				return nil, nil, fmt.Errorf("quests.rec quest %d: unknown field %q", i+1, fld.Name)
			}
		}
		if q.Key == "" {
			return nil, nil, fmt.Errorf("quests.rec quest %d: no key", i+1)
		}
		quests = append(quests, q)
	}

	var triggers []Trigger
	for i, rec := range all["trigger"] {
		var t Trigger
		for _, fld := range rec {
			switch fld.Name {
			case "event":
				t.Event = fld.Value
			case "subject":
				t.Subject = fld.Value
			case "when":
				t.When = fld.Value
			case "pass":
				t.Pass = fld.AsBool()
			case "do":
				p := recfile.StrPredicate(fld.Value)
				if p == nil {
					return nil, nil, fmt.Errorf("quests.rec trigger %d: do wants \"outcome(args)\", got %q", i+1, fld.Value)
				}
				t.Do = append(t.Do, p)
			default:
				return nil, nil, fmt.Errorf("quests.rec trigger %d: unknown field %q", i+1, fld.Name)
			}
		}
		if t.Event == "" || len(t.Do) == 0 {
			return nil, nil, fmt.Errorf("quests.rec trigger %d: needs an event and at least one do", i+1)
		}
		if t.When != "" {
			p, err := expr.Compile(t.When, expr.Env(QuestEnv{}), expr.AsBool())
			if err != nil {
				return nil, nil, fmt.Errorf("quests.rec trigger %d (%s %s): %w", i+1, t.Event, t.Subject, err)
			}
			t.program = p
		}
		triggers = append(triggers, t)
	}
	return quests, triggers, nil
}
