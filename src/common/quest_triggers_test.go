package common

import (
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/expr-lang/expr"

	"rpg/files"
	"rpg/src/defs"
	"rpg/src/rules"
)

func loadQuestData(t *testing.T) *rules.Data {
	t.Helper()
	data, err := rules.Load(files.GetFileSystem())
	if err != nil {
		t.Skipf("no mod loaded: %v", err)
	}
	if len(data.QuestDefs) == 0 {
		t.Skip("the loaded mod has no quests")
	}
	return data
}

// forEachMod runs a content check against every mod's quests, not just the one TestMain selected.
func forEachMod(t *testing.T, check func(t *testing.T, data *rules.Data)) {
	for _, mod := range []string{"d1", "heavenandhell"} {
		t.Run(mod, func(t *testing.T) {
			withMod(t, mod)
			check(t, loadQuestData(t))
		})
	}
}

// Every quest the mod declares is usable: a key nothing else uses, a name, and a log text.
func TestQuestRecordsAreWellFormed(t *testing.T) {
	forEachMod(t, func(t *testing.T, data *rules.Data) {
		seen := map[string]bool{}
		for i, q := range data.QuestDefs {
			switch {
			case q.Key == "" && i < rules.NumQuests: // an engine quest this mod does not declare
				continue
			case q.Key == "":
				t.Errorf("quest %d: no key", i)
			case seen[q.Key]:
				t.Errorf("quest %d: duplicate key %q", i, q.Key)
			case q.Name == "":
				t.Errorf("quest %q: no name", q.Key)
			case q.LogText == "":
				t.Errorf("quest %q: no log_text", q.Key)
			}
			seen[q.Key] = true
		}
	})
}

// Every trigger names an event the engine fires, a condition that compiled, and outcomes the engine has.
func TestTriggersResolve(t *testing.T) {
	forEachMod(t, func(t *testing.T, data *rules.Data) {
		keys := map[string]bool{}
		for _, q := range data.QuestDefs {
			keys[q.Key] = true
		}
		for i, tr := range data.QuestTriggers {
			if !slices.Contains(QuestEvents, tr.Event) {
				t.Errorf("trigger %d: no such event %q", i, tr.Event)
			}
			if tr.Subject == "" {
				t.Errorf("trigger %d (%s): no subject", i, tr.Event)
			}
			if tr.When != "" && tr.Cond() == nil {
				t.Errorf("trigger %d (%s %s): condition did not compile", i, tr.Event, tr.Subject)
			}
			for _, do := range tr.Do {
				if !QuestOutcomeExists(do.Name()) {
					t.Errorf("trigger %d (%s %s): no outcome named %q", i, tr.Event, tr.Subject, do.Name())
				}
				// the outcomes that name a quest must name one that exists
				switch do.Name() {
				case "take_item", "drop_item", "spawn_item":
					if _, ok := data.ItemsByName[do.GetString(0)]; !ok {
						t.Errorf("trigger %d (%s %s): %s names unknown item %q", i, tr.Event, tr.Subject, do.Name(), do.GetString(0))
					}
				case "drop_unique":
					if !slices.ContainsFunc(data.UniqueItems, func(u rules.UniqueItemData) bool { return u.ID == do.GetString(0) }) {
						t.Errorf("trigger %d (%s %s): no unique item %q", i, tr.Event, tr.Subject, do.GetString(0))
					}
				case "say":
					if data.Texts[do.GetString(0)] == "" {
						t.Errorf("trigger %d (%s %s): says %q, which texts.rec does not have", i, tr.Event, tr.Subject, do.GetString(0))
					}
				case "complete_quest", "activate_quest", "log_quest", "set_quest_var":
					if !keys[do.GetString(0)] {
						t.Errorf("trigger %d (%s %s): %s names unknown quest %q", i, tr.Event, tr.Subject, do.Name(), do.GetString(0))
					}
				}
			}
		}
	})
}

// A condition may only ask about quests the mod declares: a typo in a key is a condition that is quietly always
// false, which is the worst kind of content bug.
func TestConditionsNameKnownQuests(t *testing.T) {
	forEachMod(t, func(t *testing.T, data *rules.Data) {
		keys := map[string]bool{}
		for _, q := range data.QuestDefs {
			keys[q.Key] = true
		}
		asked := map[string]bool{}
		probe := func(key string) bool { asked[key] = true; return false }
		probeInt := func(key string) int { asked[key] = true; return 0 }
		env := defs.QuestEnv{Done: probe, Active: probe, Avail: probe, Logged: probe, V1: probeInt, V2: probeInt,
			Flag: func(string) bool { return false }, Has: no, Here: probe, At: no}
		for i, tr := range data.QuestTriggers {
			if tr.Cond() == nil {
				continue
			}
			clear(asked)
			if _, err := expr.Run(tr.Cond(), env); err != nil {
				t.Errorf("trigger %d (%s %s): %v", i, tr.Event, tr.Subject, err)
				continue
			}
			for key := range asked {
				if !keys[key] {
					t.Errorf("trigger %d (%s %s): condition asks about unknown quest %q", i, tr.Event, tr.Subject, key)
				}
			}
		}
	})
}

// selected is the triggers the engine would run for an event and subject, in order -- the matching half of
// fireQuestTriggers, without a running game.
func selected(data *rules.Data, event, subject string) (out []defs.Trigger) {
	for _, tr := range data.QuestTriggers {
		if tr.Event == event && tr.Subject == subject {
			out = append(out, tr)
		}
	}
	return
}

// A trigger answers to its own event and subject and to nothing else.
func TestTriggersMatchTheirEventOnly(t *testing.T) {
	data := loadQuestData(t)
	for i, tr := range data.QuestTriggers {
		if len(selected(data, tr.Event, tr.Subject)) == 0 {
			t.Fatalf("trigger %d: %s/%s selects nothing", i, tr.Event, tr.Subject)
		}
		if n := len(selected(data, tr.Event, tr.Subject+"!")); n != 0 {
			t.Errorf("trigger %d: %s answers to a subject it does not name", i, tr.Event)
		}
		for _, other := range QuestEvents {
			if other == tr.Event {
				continue
			}
			for _, also := range selected(data, other, tr.Subject) {
				if also.When == tr.When && slices.Equal(names(also), names(tr)) {
					t.Errorf("trigger %d: %s/%s is also reachable as %s", i, tr.Event, tr.Subject, other)
				}
			}
		}
	}
}

func names(tr defs.Trigger) (out []string) {
	for _, do := range tr.Do {
		out = append(out, do.Name())
	}
	return
}

// A trigger that remembers itself with a flag stops firing once it has: true before, false after. That is the
// "each once" contract of the hero's lines on first reaching a new part of the dungeon, checked on the real
// conditions rather than on a copy of them.
func TestOnceTriggersStopFiring(t *testing.T) {
	data := loadQuestData(t)
	run := func(tr defs.Trigger, set map[string]bool) bool {
		t.Helper()
		env := defs.QuestEnv{
			Done: no, Active: no, Avail: no, Logged: no, V1: zero, V2: zero,
			Flag: func(k string) bool { return set[k] }, Has: no,
		}
		out, err := expr.Run(tr.Cond(), env)
		if err != nil {
			t.Fatalf("%s/%s: %v", tr.Event, tr.Subject, err)
		}
		return out.(bool)
	}
	checked := 0
	for _, tr := range data.QuestTriggers {
		for _, do := range tr.Do {
			if do.Name() != "set_flag" || tr.Cond() == nil {
				continue
			}
			key := do.GetString(0)
			if !run(tr, nil) {
				t.Errorf("%s/%s: does not fire with %q unset", tr.Event, tr.Subject, key)
			}
			if run(tr, map[string]bool{key: true}) {
				t.Errorf("%s/%s: still fires with %q set", tr.Event, tr.Subject, key)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Skip("no once-only triggers in this mod")
	}
}

// The dying townsman's two dialogue records are mutually exclusive in every quest state he can be met in: he
// either says his piece or groans, never both.
func TestDeadGuyDialogueIsExclusive(t *testing.T) {
	data := loadQuestData(t)
	lines := selected(data, EventApproach, "deadguy")
	if len(lines) != 2 {
		t.Skipf("this mod has %d deadguy lines", len(lines))
	}
	for _, c := range []struct {
		name       string
		state, v1  int
		wantSpeaks bool
		wantGroans bool
	}{
		{"not yet started", rules.QuestInit, 0, true, false},
		{"started, not yet spoken", rules.QuestActive, 0, true, false},
		{"already spoken", rules.QuestActive, 1, false, true},
		{"butcher dead", rules.QuestDone, 1, false, false},
		{"not in this game", rules.QuestNotAvail, 0, false, false},
	} {
		env := defs.QuestEnv{
			Done:   func(k string) bool { return k == "butcher" && c.state == rules.QuestDone },
			Active: func(k string) bool { return k == "butcher" && c.state == rules.QuestActive },
			Avail:  func(k string) bool { return k == "butcher" && c.state != rules.QuestNotAvail },
			Has:    no,
			Logged: no,
			V1:     func(string) int { return c.v1 },
			V2:     zero,
			Flag:   func(string) bool { return false },
		}
		var speaks, groans bool
		for _, tr := range lines {
			out, err := expr.Run(tr.Cond(), env)
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			if !out.(bool) {
				continue
			}
			if slices.Contains(names(tr), "say") {
				speaks = true
			} else {
				groans = true
			}
		}
		if speaks != c.wantSpeaks || groans != c.wantGroans {
			t.Errorf("%s: speaks=%v groans=%v, want %v/%v", c.name, speaks, groans, c.wantSpeaks, c.wantGroans)
		}
	}
}

func no(string) bool  { return false }
func zero(string) int { return 0 }

// Every mod says when its game is won: some trigger ends it with end_text(text), and the text exists.
func TestEveryModEnds(t *testing.T) {
	forEachMod(t, func(t *testing.T, data *rules.Data) {
		for _, tr := range data.QuestTriggers {
			for _, do := range tr.Do {
				if do.Name() == "end_text" {
					if data.Texts[do.GetString(0)] == "" {
						t.Errorf("end_text(%s): no such text", do.GetString(0))
					}
					return
				}
			}
		}
		t.Error("no trigger ends the game with end_text(text)")
	})
}

// Every mod brings its own UI files: a default font from its fonts/ folder, the terminal UI's settings and the
// hero name lists, so nothing is read from outside the mod.
func TestEveryModHasItsUIFiles(t *testing.T) {
	forEachMod(t, func(t *testing.T, _ *rules.Data) {
		name, ok := ModSetting("Font")
		if !ok || !slices.Contains(ModFonts(), name) {
			t.Errorf("mod.rec's Font %q is not in the mod's fonts/ (%v)", name, ModFonts())
		}
		for _, f := range []string{"data/ascii/ui.rec", "data/ascii/palette.rec", "data/ascii/keymap.txt",
			"data/names-male.txt", "data/names-female.txt"} {
			if _, err := fs.Stat(files.GetFileSystem(), path.Join("mods", files.ActiveMod(), f)); err != nil {
				t.Error(err)
			}
		}
	})
}

// A conversation says one thing, so the lines of one (event, subject) must rule each other out whatever order the
// file lists them in. For each conversation this tries every combination of what its conditions ask about: each
// quest's state (not available, available, active, done) and log, each flag, item and place as true or false, and
// each number and name as every literal the conditions compare it with, one either side of each number, and
// 0 or "". Conditions only compare against literals, so these values reach every case a condition can tell apart.
func TestTalkTriggersAreExclusive(t *testing.T) {
	forEachMod(t, func(t *testing.T, data *rules.Data) {
		groups := map[string][]int{}
		for i, tr := range data.QuestTriggers {
			if !tr.Pass && conversationEvent(tr.Event) {
				groups[tr.Event+" "+tr.Subject] = append(groups[tr.Event+" "+tr.Subject], i)
			}
		}
		for g, lines := range groups {
			for x, a := range lines {
				for _, b := range lines[x+1:] {
					checkExclusive(t, g, data.QuestTriggers[a], data.QuestTriggers[b])
				}
			}
		}
	})
}

var (
	condCall   = regexp.MustCompile(`\b(done|active|avail|logged|v1|v2|flag|has|at|here)\("([^"]*)"\)`)
	condBare   = regexp.MustCompile(`\b(depth|deepest|map|talk|goal|visible)\b`)
	condNumber = regexp.MustCompile(`\b\d+\b`)
	condQuoted = regexp.MustCompile(`"([^"]*)"`)
)

// checkExclusive tries every combination of what two lines' conditions ask about and fails if both hold in one.
func checkExclusive(t *testing.T, g string, a, b defs.Trigger) {
	t.Helper()
	ints, strs := []int{0}, []string{""}
	var vars []string
	for _, when := range []string{a.When, b.When} {
		for _, m := range condNumber.FindAllString(when, -1) {
			n, _ := strconv.Atoi(m)
			ints = append(ints, n-1, n, n+1)
		}
		for _, m := range condQuoted.FindAllStringSubmatch(when, -1) {
			strs = append(strs, m[1])
		}
		for _, m := range condCall.FindAllStringSubmatch(when, -1) {
			switch m[1] {
			case "done", "active", "avail":
				vars = append(vars, "state "+m[2])
			default:
				vars = append(vars, m[1]+" "+m[2])
			}
		}
		vars = append(vars, condBare.FindAllString(when, -1)...)
	}
	slices.Sort(vars)
	vars = slices.Compact(vars)
	domain := func(v string) int {
		switch kind, _, _ := strings.Cut(v, " "); kind {
		case "state":
			return 4
		case "v1", "v2", "depth", "deepest":
			return len(ints)
		case "map", "talk":
			return len(strs)
		case "goal":
			return 5
		}
		return 2
	}
	pick := make([]int, len(vars))
	at := func(v string) int { // an input neither condition reads keeps its first value
		if i, ok := slices.BinarySearch(vars, v); ok {
			return pick[i]
		}
		return 0
	}
	state := func(k string) int { return at("state " + k) } // 0 not available, 1 available, 2 active, 3 done
	coin := func(kind string) func(string) bool { return func(k string) bool { return at(kind+" "+k) == 1 } }
	num := func(kind string) func(string) int { return func(k string) int { return ints[at(kind+" "+k)] } }
	holds := func(tr defs.Trigger, env defs.QuestEnv) bool {
		p := tr.Cond()
		if p == nil {
			return true
		}
		out, err := expr.Run(p, env)
		return err == nil && out == true
	}
	for {
		env := defs.QuestEnv{
			Done: func(k string) bool { return state(k) == 3 }, Active: func(k string) bool { return state(k) == 2 },
			Avail: func(k string) bool { return state(k) != 0 }, Logged: coin("logged"), Flag: coin("flag"),
			V1: num("v1"), V2: num("v2"), Has: coin("has"), At: coin("at"), Here: coin("here"),
			Depth: ints[at("depth")], Deepest: ints[at("deepest")], Map: strs[at("map")], Talk: strs[at("talk")],
			Goal: []string{"normal", "attack2", "inquiring", "talking", "other"}[at("goal")], Visible: at("visible") == 1,
		}
		if holds(a, env) && holds(b, env) {
			t.Errorf("%s: %q and %q hold together", g, a.When, b.When)
			return
		}
		k := 0 // next combination (mixed radix)
		for ; k < len(vars); k++ {
			if pick[k]++; pick[k] < domain(vars[k]) {
				break
			}
			pick[k] = 0
		}
		if k == len(vars) {
			return
		}
	}
}
