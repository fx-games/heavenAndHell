package dungen

import (
	"slices"
	"testing"

	"rpg/src/geometry"
)

// A North connector on a room turned 270 degrees faces West. Before directions were wrapped it reported 540,
// whose Opposite is 360, which no East connector (0) ever matched: that attachment point was silently dead.
func TestRotatedConnectorStillMatches(t *testing.T) {
	room := NewDungeonRoomFromTemplate("t")
	room.SetFloorTiles([]geometry.Point{{X: 0, Y: 1}})
	room.SetConnections(map[geometry.Point]RoomConnection{{X: 0, Y: 0}: {OutwardDirection: geometry.North}})
	for count, want := range []geometry.CompassDirection{geometry.North, geometry.East, geometry.South, geometry.West} {
		room.SetRotationCount(count)
		for _, c := range room.GetFreeConnectionsWithRotatedDirection() {
			if c.OutwardDirection != want {
				t.Errorf("rotation %d: faces %v, want %v", count, c.OutwardDirection, want)
			}
		}
		if _, ok := room.HasMatchingConnection(RoomConnection{OutwardDirection: want.Opposite()}); !ok {
			t.Errorf("rotation %d: a connector facing %v found no match", count, want.Opposite())
		}
	}
}

// Connectors are ordered row by row whatever the template's width; ordering by y*20+x put (25,0) after (0,1).
func TestConnectorOrderIgnoresWidth(t *testing.T) {
	doors := map[geometry.Point]RoomConnection{{X: 0, Y: 1}: {}, {X: 25, Y: 0}: {}, {X: 3, Y: 0}: {}}
	got := getSortedKeys(doors)
	want := []geometry.Point{{X: 3, Y: 0}, {X: 25, Y: 0}, {X: 0, Y: 1}}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Every one of the eight orientations maps a room's tiles and back again, and turns a connector the same way
// it turns the floor behind it: the connector still sits just outside that floor, facing away from it.
func TestOrientationsKeepConnectorsOnTheirFloor(t *testing.T) {
	room := NewDungeonRoomFromTemplate("t")
	room.SetFloorTiles([]geometry.Point{{X: 1, Y: 1}, {X: 2, Y: 1}, {X: 1, Y: 2}})
	room.SetConnections(map[geometry.Point]RoomConnection{
		{X: 1, Y: 0}: {OutwardDirection: geometry.North}, {X: 3, Y: 1}: {OutwardDirection: geometry.East},
		{X: 1, Y: 3}: {OutwardDirection: geometry.South}, {X: 0, Y: 2}: {OutwardDirection: geometry.West}})
	for orientation := 0; orientation < 8; orientation++ {
		room.SetTransform(orientation%4, orientation >= 4)
		room.SetPositionOffset(geometry.Point{X: 10, Y: 10})
		for rel, c := range room.GetFreeConnectionsWithRotatedDirection() {
			abs := room.ToAbsolutePosition(rel)
			if back := room.ToRelativePosition(abs); back != rel {
				t.Errorf("orientation %d: %v maps back to %v", orientation, rel, back)
			}
			if inside := abs.Sub(c.OutwardDirection.ToPoint()); !room.Contains(inside) {
				t.Errorf("orientation %d: connector %v faces %v, away from no floor", orientation, rel, c.OutwardDirection)
			}
		}
	}
}
