package app

import (
	"rpg/src/common"
	"rpg/src/defs"
	"rpg/src/rules"
	"rpg/src/stats"
	"testing"
)

// Spell menu keys follow the skill bar: 1-0, then a-z. Dragging a skill to another slot moves its key with it.
func TestSpellMenuHotkeys(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "")
	player := theGame.GetWorldState().GetPlayer()
	player.Hero().SpellLevels[rules.SplFirebolt] = 1
	player.Hero().SpellLevels[rules.SplHeal] = 1
	theGame.GetFactory().RefreshSpells(player)
	staff := theGame.GetFactory().NewRolledItem(&rules.Item{Name: "Staff of Infravision", Type: rules.ITypeStaff, Spell: rules.SplInfra, Charges: 5, MaxCharges: 5})
	player.GetInventory().Add(staff)

	menu := func() (labels []string) {
		for _, it := range common.SpellMenuItems(theGame.GetWorldState(), theGame.GetActions()) {
			labels = append(labels, it.MainText)
		}
		return labels
	}
	check := func(want ...string) {
		t.Helper()
		got := menu()
		if len(got) != len(want) {
			t.Fatalf("menu %q, want %q", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("menu %q, want %q", got, want)
			}
		}
	}
	check("1) Firebolt", "2) Item Repair", "3) Healing", "4) Vision (Staff, 5 charges)")
	common.MoveSkill(player, 3, 0) // drag the staff to the front
	check("1) Vision (Staff, 5 charges)", "2) Firebolt", "3) Item Repair", "4) Healing")
	player.Hero().SpellLevels[rules.SplLightning] = 1 // a newly learned spell goes last, the others keep their keys
	theGame.GetFactory().RefreshSpells(player)
	check("1) Vision (Staff, 5 charges)", "2) Firebolt", "3) Item Repair", "4) Healing", "5) Lightning")

	color := func(i int) any {
		return common.SpellMenuItems(theGame.GetWorldState(), theGame.GetActions())[i].MainTextColor
	}
	if color(1) != defs.White || color(0) == defs.Red {
		t.Fatalf("castable lines: skill %v, staff %v", color(1), color(0))
	}
	player.GetStats().GetResource(stats.Mana).Decrease(player.GetStats().GetResource(stats.Mana).Value())
	staff.SetCharges(0)
	if color(1) != defs.Red || color(0) != defs.Red || color(2) != defs.White { // Item Repair costs nothing
		t.Fatalf("uncastable lines not red: staff %v, firebolt %v, repair %v", color(0), color(1), color(2))
	}
}
