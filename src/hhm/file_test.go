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
