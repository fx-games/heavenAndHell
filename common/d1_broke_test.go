package common

import (
	"rpg/d1"
	"testing"
)

// Worn-out items are destroyed like in D1, except uniques: they stay equipped for Griswold to repair.
func TestD1BrokeDestroysAllButUniques(t *testing.T) {
	a := NewActor(0)
	noOwner := func() *Actor { return nil }
	helm := &Item{getPlayer: noOwner, d1: &d1.Item{Type: d1.ITypeHelm, Class: "ICLASS_ARMOR", MaxDur: 20}}
	body := &Item{getPlayer: noOwner, d1: &d1.Item{Type: d1.ITypeLArmor, Class: "ICLASS_ARMOR", MaxDur: 20, Magical: 2}}
	a.GetEquipment().Equip(helm)
	a.GetEquipment().Equip(body)
	d1Broke(func(string) {}, a, helm.d1)
	d1Broke(func(string) {}, a, body.d1)
	if a.GetEquipment().IsEquipped(helm) {
		t.Error("a broken helm stays equipped")
	}
	if !a.GetEquipment().IsEquipped(body) {
		t.Error("a broken unique was destroyed")
	}
}
