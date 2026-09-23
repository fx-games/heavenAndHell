package app

import (
	"testing"

	"rpg/src/rules"
)

// The sword/mace rule against animals and the undead is Diablo's, declared in its hero.rec; Heaven & Hell
// declares none, so no weapon there cares what it hits.
func TestWeaponBonusesAreTheMods(t *testing.T) {
	g, _ := createGameAndUIContext() // d1
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	data := g.GetWorldState().GetPlayer().Hero().Rules()
	sword := rules.Sheet{WeaponMask: 1 << rules.ITypeSword}
	mace := rules.Sheet{WeaponMask: 1 << rules.ITypeMace}
	both := rules.Sheet{WeaponMask: 1<<rules.ITypeSword | 1<<rules.ITypeMace}
	for _, c := range []struct {
		name  string
		s     rules.Sheet
		class string
		want  int
	}{
		{"sword vs animal", sword, "MC_ANIMAL", 50}, {"sword vs undead", sword, "MC_UNDEAD", -50},
		{"mace vs undead", mace, "MC_UNDEAD", 50}, {"mace vs animal", mace, "MC_ANIMAL", -50},
		{"sword and mace: the sword counts", both, "MC_UNDEAD", -50}, {"sword vs demon", sword, "MC_DEMON", 0},
	} {
		if got := data.WeaponVsMonsterClass(c.s, c.class); got != c.want {
			t.Errorf("D1 %s: %+d%%, want %+d%%", c.name, got, c.want)
		}
	}

	h := newHeavenAndHellGame(t, NewDummyUI())
	h.NewGame(h.GetGameData().ClassDefinitions[0], false, "")
	hh := h.GetWorldState().GetPlayer().Hero().Rules()
	if len(hh.WeaponBonuses) != 0 {
		t.Errorf("H&H applies %d weapon bonuses; its hero.rec declares none", len(hh.WeaponBonuses))
	}
	if got := hh.WeaponVsMonsterClass(mace, "MC_UNDEAD"); got != 0 {
		t.Errorf("H&H mace vs undead: %+d%%, want 0", got)
	}
}
