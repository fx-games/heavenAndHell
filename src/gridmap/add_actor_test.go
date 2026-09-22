package gridmap

import (
	"rpg/src/geometry"
	"testing"
)

// An actor is either on the map and owns exactly one cell, or not on the map at all: AddActor must never leave
// it listed but placeless, which is how two monsters used to look like they shared a tile.
func TestAddActorIsAllOrNothing(t *testing.T) {
	m := NewEmptyMap[*stub, *stub, *stub](4, 4)
	m.Fill(MapCell[*stub, *stub, *stub]{TileType: Tile{IsWalkable: true, IsTransparent: true}})
	at := geometry.Point{X: 1, Y: 1}

	first := &stub{}
	if !m.AddActor(first, at) {
		t.Fatal("the first actor was refused an empty cell")
	}
	if m.AddActor(&stub{}, at) {
		t.Fatal("a second actor was placed on the same cell")
	}
	if m.AddActor(&stub{}, geometry.Point{X: 9, Y: 9}) {
		t.Fatal("an actor was placed off the map")
	}
	if got := len(m.Actors()); got != 1 {
		t.Fatalf("%d actors on the map, want 1", got)
	}
	if m.ActorAt(at) != first {
		t.Fatal("the cell does not hold the actor that was placed on it")
	}
}
