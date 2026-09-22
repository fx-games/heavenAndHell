package main

import (
	"strings"
	"testing"

	"rpg/common"
	"rpg/d1"
	"rpg/gridmap"
)

// Every lever, book and skull switch D1 places on a level opens part of it (ObjChangeMap). Since the map changes
// became levels.rec patches, a level whose record forgets one leaves that part shut: this walks every D1 level
// over several seeds and checks each such object's kind is named in one of its level's patches.
func TestEveryMapChangingObjectHasAPatch(t *testing.T) {
	changers := []int{d1.ObjLEVER, d1.ObjCRUX1, d1.ObjCRUX2, d1.ObjCRUX3, d1.ObjBOOK2L, d1.ObjSWITCHSKL, d1.ObjBOOK2R,
		d1.ObjBLINDBOOK, d1.ObjSTEELTOME}
	// The Chamber of Bone's book teaches Guardian; it opens nothing (OperateBook outside Lazarus' lair).
	opensNothing := map[string]int{"bone_chamber": d1.ObjBOOK2L}
	for seed := int64(1); seed <= 4; seed++ {
		g, _ := createGameAndUIContext()
		g.GetAudio().SetDisablePlayback(true)
		g.SeedRandom(seed)
		common.SeedD1Rng(seed)
		g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
		s := g.GetWorldState()
		for q := range s.D1Quests() { // every quest's set piece, not just the ones this seed rolled
			s.D1Quests()[q].Active = d1.QuestInit
		}
		names := g.GetFactory().D1().D.Objects
		for _, l := range s.Levels() {
			if l.Town {
				continue
			}
			s.MapTransition(gridmap.Transition{TargetMap: l.Map, TargetLocation: "ladder_up"})
			for _, obj := range s.GetMap().Objects() {
				o, ok := obj.(*common.D1Object)
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
