package main

import "testing"

func TestSetFields(t *testing.T) {
	in := "# c\n\nname: a\ninternal_name: a\ndlvl: 1\nmax_dlvl: 3\n\ninternal_name: b\ndlvl: 2\n"
	got := setFields(in, "internal_name", "a", [][3]string{{"dlvl", "2", "0"}, {"max_dlvl", "0", "0"}, {"spawn_weight", "3", "1"}})
	want := "# c\n\nname: a\ninternal_name: a\ndlvl: 2\nspawn_weight: 3\n\ninternal_name: b\ndlvl: 2\n"
	if got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
	if setFields(want, "internal_name", "b", [][3]string{{"dlvl", "2", "0"}, {"available", "0", "0"}}) != want {
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
	d1Rules = true
	defer func() { d1Rules = false }()
	z := &enemy{min: 1, max: 3, avail: 1} // D1's zombie: levels 1-2
	if lo, hi, _ := z.span(16); lo != 1 || hi != 2 {
		t.Fatal(lo, hi)
	}
	z.add(4, 16)
	z.remove(1, 16)
	if lo, hi, _ := z.span(16); lo != 2 || hi != 4 {
		t.Fatal(lo, hi)
	}
}

func TestSetList(t *testing.T) {
	in := "map: a\nmonster: x 1\ndepth: 1\nmonster: y 2\n\nmap: b\nmonster: x 1\n"
	got := setList(in, "map", "a", "monster", []string{"z 3"})
	if want := "map: a\ndepth: 1\nmonster: z 3\n\nmap: b\nmonster: x 1\n"; got != want {
		t.Fatalf("got %q", got)
	}
}
