package main

import (
	"rpg/src/recfile"
	"strings"
	"testing"
)

func TestSetFields(t *testing.T) {
	in := "# c\n\nname: a\ninternal_name: a\ndlvl: 1\nmax_dlvl: 3\n\ninternal_name: b\ndlvl: 2\n"
	got := setFields(in, "internal_name", "a", [][3]string{{"dlvl", "2", "0"}, {"max_dlvl", "0", "0"}, {"spawn_cost", "300", "0"}})
	want := "# c\n\nname: a\ninternal_name: a\ndlvl: 2\nspawn_cost: 300\n\ninternal_name: b\ndlvl: 2\n"
	if got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
	if setFields(want, "internal_name", "b", [][3]string{{"dlvl", "2", "0"}, {"random_spawn", "true", "true"}}) != want {
		t.Fatal("unchanged values rewrote the record")
	}
}

func TestSpan(t *testing.T) {
	e := &enemy{name: "x", min: 2, avail: 1}
	e.add(1, 8)
	if lo, hi, _ := e.span(8); lo != 1 || hi != 8 {
		t.Fatal(lo, hi)
	}
	if e.remove(8, 8); e.max != 7 {
		t.Fatal(e.max)
	}
	if e.remove(4, 8) == nil {
		t.Fatal("hole allowed")
	}
}

func TestSetList(t *testing.T) {
	in := "map: a\nmonster: x 1\ndepth: 1\nmonster: y 2\n\nmap: b\nmonster: x 1\n"
	got := setList(in, "map", "a", "monster", []string{"z 3"})
	if want := "map: a\ndepth: 1\nmonster: z 3\n\nmap: b\nmonster: x 1\n"; got != want {
		t.Fatalf("got %q", got)
	}
}

func TestMoveRoom(t *testing.T) {
	a := &level{id: "a", require: []string{"R", "S"}}
	b := &level{id: "b"}
	moveRoom([]*level{a, b}, "R", b)
	if len(a.require) != 1 || b.require[0] != "R" {
		t.Fatal(a, b)
	}
}

func TestQuestSpawns(t *testing.T) {
	in := "key: q\nlevel: x\n\nevent: on_enter_level\nsubject: x\nwhen: active(\"q\")\ndo: spawn_monster(a, anywhere)\ndo: spawn_monster(b, anywhere)\n"
	sp := questSpawns(recfile.Read(strings.NewReader(in)))
	if sp["a"] == nil || sp["a"] != sp["b"] || sp["a"].quests[0] != "q" {
		t.Fatal(sp)
	}
	out := setSubject(in, sp["a"].marker, "y")
	out = setFields(out, "key", "q", [][3]string{{"level", "y", ""}})
	if want := strings.ReplaceAll(in, ": x", ": y"); out != want {
		t.Fatalf("got %q", out)
	}
}
