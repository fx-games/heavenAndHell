package app

import (
	"strings"
	"testing"

	"rpg/src/common"
	"rpg/src/gridmap"
	"rpg/src/rules"
)

// Every lever, book and skull switch D1 places on a level opens part of it (ObjChangeMap). Since the map changes
// became levels.rec patches, a level whose record forgets one leaves that part shut: this walks every D1 level
// over several seeds and checks each such object's kind is named in one of its level's patches.
func TestEveryMapChangingObjectHasAPatch(t *testing.T) {
	changers := []int{rules.ObjLEVER, rules.ObjCRUX1, rules.ObjCRUX2, rules.ObjCRUX3, rules.ObjBOOK2L, rules.ObjSWITCHSKL, rules.ObjBOOK2R,
		rules.ObjBLINDBOOK, rules.ObjSTEELTOME}
	// The Chamber of Bone's book teaches Guardian; it opens nothing (OperateBook outside Lazarus' lair).
	opensNothing := map[string]int{"bone_chamber": rules.ObjBOOK2L}
	for seed := int64(1); seed <= 4; seed++ {
		g, _ := createGameAndUIContext()
		g.GetAudio().SetDisablePlayback(true)
		g.SeedRandom(seed)
		common.SeedRandom(seed)
		g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
		s := g.GetWorldState()
		for q := range s.Quests() { // every quest's set piece, not just the ones this seed rolled
			s.Quests()[q].Active = rules.QuestInit
		}
		names := g.GetFactory().Gen().D.Objects
		for _, l := range s.Levels() {
			if l.Town {
				continue
			}
			s.MapTransition(gridmap.Transition{TargetMap: l.Map, TargetLocation: "ladder_up"})
			for _, obj := range s.GetMap().Objects() {
				o, ok := obj.(*common.Prop)
				if !ok || !contains(changers, o.O.Kind) || opensNothing[l.Map] == o.O.Kind {
					continue
				}
				declared := false
				for _, p := range l.Patches {
					declared = declared || strings.Contains(" "+p.Region+" ", " "+names[o.O.Kind].Name+" ")
				}
				if !declared {
					t.Errorf("seed %d, %s: %s at %v has no patch", seed, l.Map, names[o.O.Kind].Name, o.MapPosition())
				}
			}
		}
	}
}

func contains(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
