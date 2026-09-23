package common

import (
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"testing"
)

// A Shield soaks damage until its points run out, and goes when its caster dies.
func TestShieldAbsorbsAndDiesWithCaster(t *testing.T) {
	caster, v := NewActor(0), NewActor(0)
	v.AddStatusEffect(StatusShield, -shieldPoints(1))
	v.shieldedBy = caster.id
	if took := absorbShield(v, 4); took != 4 || !v.HasStatusEffect(StatusShield) {
		t.Fatalf("took %v, shield up %v; want 4, true", took, v.HasStatusEffect(StatusShield))
	}
	if took := absorbShield(v, 100); took != 6 || v.HasStatusEffect(StatusShield) {
		t.Fatalf("took %v, shield up %v; want 6, false", took, v.HasStatusEffect(StatusShield))
	}
	v.AddStatusEffect(StatusShield, -10)
	v.shieldedBy = caster.id
	dropShieldsOf([]*Actor{v}, caster)
	if v.HasStatusEffect(StatusShield) {
		t.Fatal("shield outlived its caster")
	}
}

// Ids stay unique past a loaded one, and a loaded Shield whose caster didn't come back is dropped.
func TestShieldCasterIDs(t *testing.T) {
	loaded := NewActor(0)
	loaded.SetID(nextActorID.Load() + 100)
	if fresh := NewActor(0); fresh.id <= loaded.id {
		t.Fatalf("new id %d not above loaded %d", fresh.id, loaded.id)
	}
	m := gridmap.NewEmptyMap[*Actor, *Item, Object](4, 4)
	hero, kept, orphan := NewActor(0), NewActor(0), NewActor(0)
	kept.AddStatusEffect(StatusShield, -7)
	kept.shieldedBy = hero.id
	orphan.AddStatusEffect(StatusShield, -7)
	orphan.shieldedBy = loaded.id // not on the map
	m.AddActor(kept, geometry.Point{X: 1, Y: 1})
	m.AddActor(orphan, geometry.Point{X: 2, Y: 2})
	DropOrphanShields(m, hero)
	if !kept.HasStatusEffect(StatusShield) || orphan.HasStatusEffect(StatusShield) {
		t.Fatal("wrong shields dropped")
	}
}
