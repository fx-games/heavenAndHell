package hhm

import (
	"os"
	"reflect"
	"testing"
)

// TestRoundTrip is the one check the format needs: whatever an editor builds, a reader reads back unchanged,
// tabs and newlines in a sign's text included.
func TestRoundTrip(t *testing.T) {
	l := NewLevel("Town", 4, 3)
	l.SetTile(0, 0, 12)
	l.SetTile(3, 2, 7)
	l.Props = Props{DisplayName: "Tristram", Weight: 3, IsDecoration: true, UsesDecoration: "Deco_", SlotID: 2, PlugsIntoSlotID: 1, CanBeLit: true}
	sign := NewEntity("Sign", 1, 1)
	sign.F["Text"] = "first\nsecond\twith a tab\\and a backslash"
	npc := NewEntity("NPC", 2, 0)
	npc.F["Name"], npc.F["InternalName"], npc.F["Tile"], npc.F["Type"], npc.F["ASCII"] = "Griswold", "blacksmith", "79", "Store_weapons", "2"
	l.Entities = append(l.Entities, sign, npc, NewEntity("NorthConnector", 0, 2))

	back, err := ReadLevel(WriteLevel(l))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(l.Props, back.Props) {
		t.Errorf("props: %+v, want %+v", back.Props, l.Props)
	}
	if !reflect.DeepEqual(l.Tiles, back.Tiles) || back.Width != 4 || back.Height != 3 {
		t.Errorf("tiles: %v %dx%d", back.Tiles, back.Width, back.Height)
	}
	if len(back.Entities) != 3 {
		t.Fatalf("got %d entities, want 3", len(back.Entities))
	}
	for _, want := range l.Entities {
		var found *Entity
		for _, e := range back.Entities {
			if e.Kind == want.Kind && e.X == want.X && e.Y == want.Y {
				found = e
			}
		}
		if found == nil {
			t.Fatalf("%s at %d,%d is gone", want.Kind, want.X, want.Y)
		}
		if !reflect.DeepEqual(want.F, found.F) {
			t.Errorf("%s fields: %v, want %v", want.Kind, found.F, want.F)
		}
	}
}

// TestLoadsTheMods is the check that the shipped maps still parse, and that every entity on them is one the
// schema knows.
func TestLoadsTheMods(t *testing.T) {
	for _, mod := range []string{"heavenandhell", "d1"} {
		p, err := Load(os.DirFS("../../files/mods/"+mod), "maps")
		if err != nil {
			t.Fatalf("%s: %v", mod, err)
		}
		if len(p.Levels) == 0 || p.Tileset.Count() == 0 {
			t.Fatalf("%s: %d levels, %d tiles", mod, len(p.Levels), p.Tileset.Count())
		}
		for _, l := range p.Levels {
			for _, e := range l.Entities {
				if _, ok := KindByName(e.Kind); !ok {
					t.Errorf("%s/%s: unknown entity %q", mod, l.Name, e.Kind)
				}
			}
		}
	}
}

// TestTilesetRoundTrip: the editor rewrites tileset.hht on every save, so every marker and every tile's record
// has to survive it.
func TestTilesetRoundTrip(t *testing.T) {
	ts := &Tileset{Name: "World", Image: "atlas.png", GridSize: 8, Width: 16, Height: 16,
		Flags:   map[int]Flags{3: {BlocksMovement: true, BlocksMissiles: true, HoldsTrap: true}, 4: {Damaging: true}},
		Records: map[int]string{3: "tile.cathedral_trap_wall", 7: "tile.arch"}}
	back, err := ReadTileset(WriteTileset(ts))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ts.Flags, back.Flags) || !reflect.DeepEqual(ts.Records, back.Records) {
		t.Errorf("flags %v records %v, want %v %v", back.Flags, back.Records, ts.Flags, ts.Records)
	}
}

// TestTileRecords: tags become markers, same_as lends its icon until the remapper gives a record its own, and a
// drawn tile takes its record's name, icon and tags.
func TestTileRecords(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/data/icons", 0o755); err != nil {
		t.Fatal(err)
	}
	rec := "internal_name: tile.wall\nname: Wall\nicon: 56\ntags: blocks_movement, blocks_view\n\n" +
		"internal_name: tile.trap_wall\nname: Trap Wall\nsame_as: tile.wall\ntags: blocks_movement, trap\n\n" +
		"internal_name: tile.remapped\nname: Remapped\nsame_as: tile.wall\nicon: 9\n"
	if err := os.WriteFile(dir+"/data/icons/tiles.rec", []byte(rec), 0o644); err != nil {
		t.Fatal(err)
	}
	recs, err := ReadTileRecords(os.DirFS(dir), "data")
	if err != nil {
		t.Fatal(err)
	}
	ts := &Tileset{Flags: map[int]Flags{5: {Water: true}}, Records: map[int]string{183: "tile.trap_wall"}}
	if got := DrawnTile(ts, recs, 183); got.Name != "Wall" || got.Icon != 56 || got.Flags != (Flags{BlocksMovement: true, HoldsTrap: true}) {
		t.Errorf("tile 183 = %+v", got)
	}
	if got := DrawnTile(ts, recs, 5); got.Icon != 5 || !got.Flags.Water || got.Name != "" {
		t.Errorf("bare tile 5 = %+v", got)
	}
	if recs["tile.remapped"].Icon != 9 {
		t.Errorf("a record's own icon lost to same_as: %d", recs["tile.remapped"].Icon)
	}
	if _, err := ReadTileRecords(os.DirFS(dir), "nowhere"); err == nil {
		t.Error("a missing tiles.rec read")
	}
}
