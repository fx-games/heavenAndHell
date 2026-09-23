package common

import (
	"rpg/src/rules"
	"testing"
)

// Worn-out items are destroyed like in D1, uniques too.
func TestBrokeDestroysUniquesToo(t *testing.T) {
	a := NewActor(0)
	noOwner := func() *Actor { return nil }
	helm := &Item{getPlayer: noOwner, roll: &rules.Item{Type: rules.ITypeHelm, Class: "ICLASS_ARMOR", MaxDur: 20}}
	body := &Item{getPlayer: noOwner, roll: &rules.Item{Type: rules.ITypeLArmor, Class: "ICLASS_ARMOR", MaxDur: 20, Magical: 2}}
	a.GetEquipment().Equip(helm)
	a.GetEquipment().Equip(body)
	itemBroke(nil, func(string) {}, a, helm.roll)
	itemBroke(nil, func(string) {}, a, body.roll)
	if a.GetEquipment().IsEquipped(helm) {
		t.Error("a broken helm stays equipped")
	}
	if a.GetEquipment().IsEquipped(body) {
		t.Error("a broken unique stays equipped")
	}
}
