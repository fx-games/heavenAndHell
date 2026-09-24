package app

import (
	"testing"

	"rpg/src/common"
	"rpg/src/stats"
)

// A ration makes the hero Satiated; each turn rested then heals 10% of max health, five times a meal.
func TestFoodRest(t *testing.T) {
	g := newHeavenAndHellGame(t, NewDummyUI())
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	f := g.GetFactory()
	player := g.GetWorldState().GetPlayer()
	food := f.NewRolledItem(f.Generator.PlainItem(f.Generator.D.ItemsByName["food"]))
	player.GetInventory().Add(food)
	health := player.GetStats().GetResource(stats.Health)
	max := health.GetMax()
	health.Decrease(health.Value() - 1)

	g.GetActions().RestUntilHealed()
	if health.Value() != 1 {
		t.Fatal("rested without eating")
	}
	food.Use(g.GetActions())
	if !player.HasStatusEffect(common.StatusSatiated) {
		t.Fatal("eating did not satiate")
	}
	for range 6 {
		g.GetActions().RestUntilHealed()
	}
	if want := 1 + 5*max/10; health.Value() < want-1 || health.Value() > want+1 {
		t.Fatalf("health %d after resting, want %d", health.Value(), want)
	}
}
