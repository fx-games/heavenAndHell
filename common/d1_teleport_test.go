package common

import (
	"rpg/geometry"
	"rpg/gridmap"
	"testing"
)

// M_Teleport lands the Blink beside its enemy on a cell whose both coordinates differ from where it stood, and
// never in the row beyond the enemy (D1's k < 1).
func TestD1TeleportBesideTheEnemy(t *testing.T) {
	for seed := int64(1); seed <= 20; seed++ {
		SeedD1Rng(seed)
		m := gridmap.NewEmptyMap[*Actor, *Item, Object](20, 20)
		blink := NewActor(0)
		blink.d1Monster = &D1Monster{}
		from, enemy := geometry.Point{X: 3, Y: 3}, geometry.Point{X: 10, Y: 10}
		m.AddActor(blink, from)
		d1Teleport(m, blink, enemy)
		p := blink.MapPosition()
		if p == from || geometry.DistanceChebyshev(p, enemy) != 1 || p.X == from.X || p.Y == from.Y {
			t.Fatalf("seed %d: the Blink landed at %v, enemy at %v", seed, p, enemy)
		}
	}
}
