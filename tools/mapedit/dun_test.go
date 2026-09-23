package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestSetPieceRoundTrip is the check this format needs: every set piece the mod ships reads back from what the
// editor would write, with the same tiles, the same overlays and the same sizes.
func TestSetPieceRoundTrip(t *testing.T) {
	dir, err := modDir("d1")
	if err != nil {
		t.Skip(err)
	}
	var found int
	for _, lt := range []string{"l1", "l2", "l3", "l4"} {
		entries, err := os.ReadDir(filepath.Join(dir, "data", "levels", lt))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) != ".txt" {
				continue
			}
			path := filepath.Join(dir, "data", "levels", lt, e.Name())
			p, err := readSetPiece(path, lt)
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			found++
			tmp := filepath.Join(t.TempDir(), e.Name())
			if err := os.WriteFile(tmp, p.write(), 0o644); err != nil {
				t.Fatal(err)
			}
			back, err := readSetPiece(tmp, lt)
			if err != nil {
				t.Fatalf("%s rewritten: %v", path, err)
			}
			if back.w != p.w || back.h != p.h || !reflect.DeepEqual(back.tiles, p.tiles) {
				t.Errorf("%s: tiles changed", e.Name())
			}
			if !reflect.DeepEqual(back.trans, p.trans) {
				t.Errorf("%s: transparency changed", e.Name())
			}
			if !reflect.DeepEqual(back.layers, p.layers) {
				t.Errorf("%s: sparse layers changed", e.Name())
			}
		}
	}
	if found == 0 {
		t.Skip("this mod ships no set pieces")
	}
}

// TestMegaTables checks the til/sol tables parse and line up: every megatile's pieces must be ones sol knows.
func TestMegaTables(t *testing.T) {
	dir, err := modDir("d1")
	if err != nil {
		t.Skip(err)
	}
	for _, lt := range []string{"l1", "l2", "l3", "l4"} {
		tbl, err := readMegaTable(filepath.Join(dir, "data", "levels", lt))
		if err != nil {
			t.Skipf("%s: %v", lt, err)
		}
		if tbl.count() == 0 {
			t.Errorf("%s: no megatiles", lt)
		}
		for i := 1; i <= tbl.count(); i++ {
			for _, piece := range tbl.quarters(i) {
				if piece >= len(tbl.sol) {
					t.Fatalf("%s: megatile %d uses piece %d, but sol has %d", lt, i, piece, len(tbl.sol))
				}
			}
		}
	}
}

// TestPartTiles checks every D1 megatile resolves to a map part the game draws.
func TestPartTiles(t *testing.T) {
	dir := filepath.Join("..", "..", "files", "mods", "d1")
	m, err := newMapEditor(dir)
	if err != nil {
		t.Skip("no d1 maps:", err)
	}
	p := newPieceEditor(dir)
	for lt, n := range map[string]int{"l1": 206, "l2": 160, "l3": 156, "l4": 137} {
		for mega := 1; mega <= n; mega++ {
			if q, ok := p.partTiles(m, lt, mega); !ok || q == [4]int{-1, -1, -1, -1} {
				t.Fatalf("%s megatile %d: no map part (%v)", lt, mega, q)
			}
		}
	}
}
