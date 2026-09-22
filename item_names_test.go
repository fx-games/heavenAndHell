package main

import (
	"testing"

	"rpg/common"
)

// How items read is the mod's (item_names.rec): Diablo shows the damage range and armour class, Heaven & Hell
// main's Angband style, dice and bonus in (), armour class and bonus in [].
func TestItemNamesFollowTheModsTemplate(t *testing.T) {
	name := func(g interface {
		GetFactory() *common.Factory
	}, internal string, count int) string {
		f := g.GetFactory()
		idx, ok := f.Generator.D.ItemIdx[internal]
		if !ok {
			t.Fatalf("no item %q", internal)
		}
		stack := common.InventoryStack{}
		for i := 0; i < count; i++ {
			stack = append(stack, f.NewD1Item(f.Generator.PlainItem(idx)))
		}
		return stack.InventoryName()
	}
	g, _ := createGameAndUIContext() // d1
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	for internal, want := range map[string]string{"short_sword": "Short Sword (2-6)", "rags": "Rags [2]"} {
		if got := name(g, internal, 1); got != want {
			t.Errorf("D1 %s: %q, want %q", internal, got, want)
		}
	}
	h := newHeavenAndHellGame(t, NewDummyUI())
	h.NewGame(h.GetGameData().ClassDefinitions[0], false, "")
	if got, want := name(h, "dagger", 1), "Dagger (1d4, +0)"; got != want {
		t.Errorf("H&H dagger: %q, want %q", got, want)
	}
	if got, want := name(h, "dagger", 3), "Dagger (1d4, +0) (x3)"; got != want {
		t.Errorf("H&H daggers: %q, want %q", got, want)
	}
	if got, want := name(h, "soft_leather_boots", 1), "Pair of Soft Leather Boots [1, +0]"; got != want {
		t.Errorf("H&H boots: %q, want %q", got, want)
	}
}
