package gridmap

import (
	"rpg/src/geometry"
	"testing"
)

type stub struct{ pos geometry.Point }

func (s *stub) MapPosition() geometry.Point     { return s.pos }
func (s *stub) SetMapPosition(p geometry.Point) { s.pos = p }
func (s *stub) Icon(tick uint64) int32          { return 0 }
func (s *stub) IsWalkable(person *stub) bool    { return true }
func (s *stub) IsTransparent() bool             { return true }
func (s *stub) IsPassableForProjectile() bool   { return true }

// Corridor map, 11x3. Row 1 is open floor, rows 0 and 2 are wall except for a
// gap at x=10 leading into a dead-end pocket at (10,2) -> (9,2):
//
//	###########
//	.........@#   player at x=9
//	#########..
//
// A candidate at (8,2) is unreachable and must be ignored; a candidate at
// (0,1) is 9 steps away, one at (9,2) is 1 step away.
func corridorMap(t *testing.T) *GridMap[*stub, *stub, *stub] {
	m := NewEmptyMap[*stub, *stub, *stub](11, 3)
	wall := Tile{IsWalkable: false}
	floor := Tile{IsWalkable: true, IsTransparent: true, IsMissilePassable: true}
	for x := 0; x < 11; x++ {
		for y := 0; y < 3; y++ {
			p := geometry.Point{X: x, Y: y}
			open := y == 1 || (y == 2 && x >= 9)
			m.SetTile(p, map[bool]Tile{true: floor, false: wall}[open])
			if open {
				m.SetExplored(p)
			}
		}
	}
	return m
}

func TestNearestByPathCostUsesPathNotStraightLine(t *testing.T) {
	m := corridorMap(t)
	player := geometry.Point{X: 9, Y: 1}

	// (8,2) is a wall right next to the player: closest by straight line,
	// not a candidate at all. (0,1) is the only reachable one.
	got := m.NearestByPathCost(player, func(p geometry.Point) bool {
		return p == geometry.Point{X: 8, Y: 2} || p == geometry.Point{X: 0, Y: 1}
	})
	if want := (geometry.Point{X: 0, Y: 1}); got != want {
		t.Errorf("unreachable candidate picked: got %v want %v", got, want)
	}

	// Both reachable: the one fewer steps away wins.
	got = m.NearestByPathCost(player, func(p geometry.Point) bool {
		return p == geometry.Point{X: 10, Y: 2} || p == geometry.Point{X: 0, Y: 1}
	})
	if want := (geometry.Point{X: 10, Y: 2}); got != want {
		t.Errorf("farther candidate picked: got %v want %v", got, want)
	}

	// No candidate -> sentinel is the position itself.
	if got := m.NearestByPathCost(player, func(geometry.Point) bool { return false }); got != player {
		t.Errorf("empty case: got %v want %v", got, player)
	}
}

func TestNearestUnexploredIsAFrontierTile(t *testing.T) {
	m := corridorMap(t)
	// Forget the west end of the corridor.
	for x := 0; x < 3; x++ {
		m.cells[1*11+x].IsExplored = false
	}
	got := m.GetNearestUnexploredWalkablePosition(geometry.Point{X: 9, Y: 1})
	// The frontier is (3,1): explored, and neighbouring the unknown (2,1).
	if want := (geometry.Point{X: 3, Y: 1}); got != want {
		t.Errorf("got %v want %v", got, want)
	}
	if !m.IsExplored(got) {
		t.Errorf("explore target %v is not on known ground", got)
	}
}

func TestNearestIsDeterministicOnTies(t *testing.T) {
	answers := map[geometry.Point]bool{}
	for i := 0; i < 40; i++ {
		m := NewEmptyMap[*stub, *stub, *stub](40, 40)
		floor := Tile{IsWalkable: true, IsTransparent: true, IsMissilePassable: true}
		for x := 0; x < 40; x++ {
			for y := 0; y < 40; y++ {
				m.SetTile(geometry.Point{X: x, Y: y}, floor)
			}
		}
		for x := 15; x < 25; x++ {
			for y := 15; y < 25; y++ {
				m.SetExplored(geometry.Point{X: x, Y: y})
			}
		}
		answers[m.GetNearestUnexploredWalkablePosition(geometry.Point{X: 20, Y: 20})] = true
	}
	if len(answers) != 1 {
		t.Errorf("symmetric room gave %d different targets: %v", len(answers), answers)
	}
}

func TestSafeForActorAvoidsTransitions(t *testing.T) {
	m := corridorMap(t)
	portal := geometry.Point{X: 5, Y: 1}
	m.AddTransitionAt(portal, Transition{TargetMap: "town", TargetLocation: "portal"})

	safe := m.CurrentlyPassableAndSafeForActor(&stub{})
	if safe(portal) {
		t.Errorf("player path would walk onto the transition at %v", portal)
	}
	if next := (geometry.Point{X: 4, Y: 1}); !safe(next) {
		t.Errorf("plain floor at %v became unwalkable", next)
	}
}
