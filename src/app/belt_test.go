package app

import (
	"rpg/src/common"
	"rpg/src/rules"
	"rpg/src/stats"
	"testing"
)

// The belt holds the pack's live stack and a spell's action: the stack counts down as it is drunk and clears when
// empty, the spell survives a refresh, a thing sits on the belt once, and the belt comes back from a save.
func TestBelt(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "")
	player := theGame.GetWorldState().GetPlayer()
	inv := player.GetInventory()
	health := player.GetStats().GetResource(stats.Health)

	var potion *common.Item
	for _, it := range inv.Items() {
		if it.Roll() != nil && it.Roll().MiscID == "IMISC_HEAL" {
			potion = it
		}
	}
	if potion == nil {
		t.Fatal("the warrior starts without a healing potion")
	}
	stack := inv.StackOf(potion)
	potions := len(*stack)
	if potions < 2 {
		t.Fatalf("want a stack of healing potions, got %d", potions)
	}
	common.SetBeltEntry(player, 0, stack)

	player.Hero().SpellLevels[rules.SplFirebolt] = 1
	theGame.GetFactory().RefreshSpells(player)
	firebolt := player.GetTargetedSkills()[0]
	if firebolt.GetInternalName() == "" {
		t.Fatal("Firebolt has no internal name")
	}
	common.SetBeltEntry(player, 1, firebolt)
	theGame.GetFactory().RefreshSpells(player)
	if common.BeltEntry(player, 1) != firebolt {
		t.Fatal("a refresh replaced the belted spell")
	}
	common.SetBeltEntry(player, 2, firebolt)
	if common.BeltEntry(player, 1) != nil || common.BeltEntry(player, 2) != firebolt {
		t.Fatal("a spell dragged to another slot is on the belt twice")
	}
	common.SwapBeltEntries(player, 1, 2)

	items := inv.Items()
	saved := common.SaveBelt(player, items)
	if saved[0].Item == 0 || saved[1].Skill != firebolt.GetInternalName() {
		t.Fatalf("saved belt %+v", saved)
	}

	for n := potions; n > 0; n-- {
		health.Decrease(health.Value() - 1)
		common.BeltEntry(player, 0).Use(theGame.GetActions())
		if left := common.BeltEntry(player, 0); n > 1 && (left == nil || left.Counter() != n-1) {
			t.Fatalf("after a drink the belt slot holds %v, want %d potions", left, n-1)
		}
	}
	if common.BeltEntry(player, 0) != nil {
		t.Fatal("the emptied potion stack is still on the belt")
	}

	common.LoadBelt(player, saved, items)
	if common.BeltEntry(player, 0) != nil || common.BeltEntry(player, 1) != firebolt {
		t.Fatalf("loaded belt: slot 1 %v, slot 2 %v", common.BeltEntry(player, 0), common.BeltEntry(player, 1))
	}
}
