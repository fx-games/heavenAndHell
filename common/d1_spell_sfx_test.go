package common

import (
	"io"
	"strings"
	"testing"

	"rpg/d1"
	"rpg/files"
	"rpg/geometry"
)

// Every D1 sound effect the game plays needs a sounds.rec event (sfx.<id>), or it is silently skipped and can't be
// remapped with ./remap_sounds.sh.
func TestD1SfxHaveSoundEvents(t *testing.T) {
	f, err := files.Open("assets/data/sounds.rec")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, _ := io.ReadAll(f)
	var ids []string
	for _, s := range d1.SpellSfx {
		ids = append(ids, s[:]...)
	}
	for _, s := range d1MissileSfx {
		ids = append(ids, s[:]...)
	}
	for _, id := range d1ObjectSfx {
		ids = append(ids, id)
	}
	for _, kind := range []int{d1.ObjBARREL, d1.ObjBARRELEX} {
		for _, dlvl := range []int{1, 17, 21} {
			ids = append(ids, d1BarrelSfx(kind, dlvl))
		}
	}
	for curs := 0; curs < 200; curs++ {
		drop, inv := d1.ItemSnds(curs)
		ids = append(ids, drop, inv)
	}
	for _, v := range []string{"WARR", "ROGUE", "MAGE", "MONK"} {
		ids = append(ids, "PS_"+v+"69", "PS_"+v+"71")
	}
	ids = append(ids, "PS_DEAD", "PS_SWING", "PS_BFIRE", "PS_WALK1", "IS_ISWORD", "IS_TRAP", "IS_DOOROPEN", "IS_CROPEN",
		"IS_RBOOK", "IS_QUESTDN", "IS_SARC", "IS_CAST8", "IS_CHEST", "IS_ISCROL", "IS_LEVER", "LS_FOUNTAIN", "LS_PUDDLE", "LS_BLODSTAR")
	for _, id := range ids {
		if cue := d1SfxCue(id); cue != "" && !strings.Contains(string(b), "internal_name: "+cue+"\n") {
			t.Errorf("sounds.rec has no %s", cue)
		}
	}
}

func TestD1SoundPosition(t *testing.T) {
	for _, c := range []struct {
		d        geometry.Point
		att, pan float64
		heard    bool
	}{
		{geometry.Point{}, 0, 0, true},
		{geometry.Point{X: 3, Y: -1}, 1.92, 7.68, true},
		{geometry.Point{X: -2, Y: 5}, 3.2, -5.12, true},
		{geometry.Point{Y: 40}, 16, 0, true}, // snd_play_snd's floor
		{geometry.Point{X: 26}, 0, 0, false}, // too far to the side
		{geometry.Point{Y: 100}, 0, 0, false},
	} {
		att, pan, heard := d1SoundPosition(geometry.Point{X: 10, Y: 10}, geometry.Point{X: 10, Y: 10}.Add(c.d))
		if heard != c.heard || heard && (att != c.att || pan != c.pan) {
			t.Errorf("%v: got %v dB, pan %v, heard %v", c.d, att, pan, heard)
		}
	}
}
