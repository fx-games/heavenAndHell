package main

import (
	"rpg/common"
	"rpg/stats"
	"testing"
)

// A potion is refused, not swallowed, while the resource it refills is already full - and drunk as usual once
// there is room again.
func TestFullResourceRefusesPotion(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, true, "")
	player := theGame.GetWorldState().GetPlayer()
	st := player.GetStats()
	st.GetResource(stats.Health).SetToMax()
	st.GetResource(stats.Mana).SetToMax()

	for _, miscID := range []string{"IMISC_HEAL", "IMISC_FULLHEAL", "IMISC_MANA", "IMISC_FULLMANA", "IMISC_REJUV"} {
		if msg, wasted := common.D1PotionWasted(player, miscID); !wasted {
			t.Fatalf("%s is not refused at full health and mana (%q)", miscID, msg)
		}
	}
	// an elixir always does something, so it is never refused
	if _, wasted := common.D1PotionWasted(player, "IMISC_ELIXSTR"); wasted {
		t.Fatal("an elixir was refused")
	}
	st.GetResource(stats.Health).Decrease(1)
	if _, wasted := common.D1PotionWasted(player, "IMISC_HEAL"); wasted {
		t.Fatal("a healing potion was refused with health missing")
	}
	if _, wasted := common.D1PotionWasted(player, "IMISC_MANA"); !wasted {
		t.Fatal("a mana potion was not refused with mana still full")
	}
}
