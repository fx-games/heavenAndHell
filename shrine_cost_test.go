package main

import (
	"rpg/common"
	"rpg/d1"
	"rpg/defs"
	"rpg/gridmap"
	"rpg/stats"
	"testing"
)

// A shrine that D1 made take something for good - a stat, a spell level, base mana, experience - asks gold
// instead: touching one leaves every one of those at least where it was, and the purse lighter.
func TestShrinesAskGoldNotLoss(t *testing.T) {
	g := newModGame(t, "d1", NewDummyUI())
	g.NewGame(g.GetGameData().ClassDefinitions[2], false, "") // the sorcerer knows spells for Arcane to raise
	s := g.GetWorldState()
	player := s.GetPlayer()
	tested := 0
	for dlvl := 1; dlvl <= 16 && tested < 3; dlvl++ {
		s.MapTransition(gridmap.Transition{TargetMap: defs.LevelNameFromDepth(dlvl), TargetLocation: "ladder_up"})
		for _, obj := range s.GetMap().Objects() {
			o, ok := obj.(*common.D1Object)
			if !ok || (o.O.Kind != d1.ObjSHRINEL && o.O.Kind != d1.ObjSHRINER) || !o.O.Active || d1.ShrineCosts[o.O.Var1] == 0 {
				continue
			}
			player.GetStats().SetAttribute(stats.Gold, 100000)
			h := *player.Hero()
			o.ActorBumpedInto(player, nil)
			after := player.Hero()
			if after.BaseStr < h.BaseStr || after.BaseMag < h.BaseMag || after.BaseDex < h.BaseDex || after.BaseVit < h.BaseVit ||
				after.MaxManaBase < h.MaxManaBase || after.Exp < h.Exp {
				t.Errorf("%s took something for good", d1.ShrineNames[o.O.Var1])
			}
			for sp := range h.SpellLevels {
				if after.SpellLevels[sp] < h.SpellLevels[sp] {
					t.Errorf("%s took a spell level", d1.ShrineNames[o.O.Var1])
				}
			}
			if player.GetStats().GetAttribute(stats.Gold) >= 100000 {
				t.Errorf("%s asked no gold", d1.ShrineNames[o.O.Var1])
			}
			tested++
		}
	}
	if tested == 0 {
		t.Skip("no gold-asking shrine in sixteen levels of this seed")
	}
	t.Logf("%d shrines checked", tested)
}
