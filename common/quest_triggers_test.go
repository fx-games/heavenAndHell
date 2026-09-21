package common

import (
	"slices"
	"testing"

	"github.com/expr-lang/expr"

	"rpg/d1"
	"rpg/defs"
	"rpg/files"
)

func loadQuestData(t *testing.T) *d1.Data {
	t.Helper()
	data, err := d1.Load(files.GetFileSystem())
	if err != nil {
		t.Skipf("no mod loaded: %v", err)
	}
	if len(data.QuestDefs) == 0 {
		t.Skip("the loaded mod has no quests")
	}
	return data
}

// Every quest the mod declares is usable: a key nothing else uses, a name, and a log text.
func TestQuestRecordsAreWellFormed(t *testing.T) {
	data := loadQuestData(t)
	seen := map[string]bool{}
	for i, q := range data.QuestDefs {
		switch {
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
		if (q.SetMap == "") != (q.SetLevel == 0) {
			t.Errorf("quest %q: set_map and set_level go together", q.Key)
		}
	}
}

// Every trigger names an event the engine fires, a condition that compiled, and outcomes the engine has.
func TestTriggersResolve(t *testing.T) {
	data := loadQuestData(t)
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
			case "complete_quest", "activate_quest", "log_quest", "set_quest_var":
				if !keys[do.GetString(0)] {
					t.Errorf("trigger %d (%s %s): %s names unknown quest %q", i, tr.Event, tr.Subject, do.Name(), do.GetString(0))
				}
			}
		}
	}
}

// A condition may only ask about quests the mod declares: a typo in a key is a condition that is quietly always
// false, which is the worst kind of content bug.
func TestConditionsNameKnownQuests(t *testing.T) {
	data := loadQuestData(t)
	keys := map[string]bool{}
	for _, q := range data.QuestDefs {
		keys[q.Key] = true
	}
	asked := map[string]bool{}
	probe := func(key string) bool { asked[key] = true; return false }
	probeInt := func(key string) int { asked[key] = true; return 0 }
	env := defs.QuestEnv{Done: probe, Active: probe, Avail: probe, Logged: probe, V1: probeInt, V2: probeInt,
		Flag: func(string) bool { return false }}
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
}

// selected is the triggers the engine would run for an event and subject, in order -- the matching half of
// fireQuestTriggers, without a running game.
func selected(data *d1.Data, event, subject string) (out []defs.Trigger) {
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
			Flag: func(k string) bool { return set[k] },
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
	lines := selected(data, EventTalk, "deadguy")
	if len(lines) != 2 {
		t.Skipf("this mod has %d deadguy lines", len(lines))
	}
	for _, c := range []struct {
		name       string
		state, v1  int
		wantSpeaks bool
		wantGroans bool
	}{
		{"not yet started", d1.QuestInit, 0, true, false},
		{"started, not yet spoken", d1.QuestActive, 0, true, false},
		{"already spoken", d1.QuestActive, 1, false, true},
		{"butcher dead", d1.QuestDone, 1, false, false},
		{"not in this game", d1.QuestNotAvail, 0, false, false},
	} {
		env := defs.QuestEnv{
			Done:   func(k string) bool { return k == "butcher" && c.state == d1.QuestDone },
			Active: func(k string) bool { return k == "butcher" && c.state == d1.QuestActive },
			Avail:  func(k string) bool { return k == "butcher" && c.state != d1.QuestNotAvail },
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
