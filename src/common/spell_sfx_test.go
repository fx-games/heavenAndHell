package common

import (
	"io"
	"strings"
	"testing"

	"rpg/files"
	"rpg/src/geometry"
	"rpg/src/rules"
)

// Every D1 sound effect the game plays needs a sounds.rec event (sfx.<id>), or it is silently skipped and can't be
// remapped with scripts/remap_sounds.sh.
func TestSfxHaveSoundEvents(t *testing.T) {
	f, err := files.Open("assets/data/sounds.rec")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, _ := io.ReadAll(f)
	var ids []string
	for _, s := range test.Spells {
		ids = append(ids, s.Sfx[:]...)
	}
	for _, m := range test.Missiles {
		ids = append(ids, m.LaunchSfx, m.ImpactSfx)
	}
	for _, d := range test.ObjectDisplays {
		ids = append(ids, d.Sfx)
	}
	for _, kind := range []int{rules.ObjBARREL, rules.ObjBARRELEX} {
		ids = append(ids, barrelSfx(kind))
	}
	for curs := 0; curs < 200; curs++ {
		drop, inv := test.ItemSnds(curs)
		ids = append(ids, drop, inv)
	}
	for _, v := range []string{"WARR", "ROGUE", "MAGE"} { // the D1 mod's classes; the Monk's voice is Hellfire's
		ids = append(ids, "PS_"+v+"69", "PS_"+v+"71")
	}
	ids = append(ids, "PS_DEAD", "PS_SWING", "PS_BFIRE", "PS_WALK1", "IS_ISWORD", "IS_TRAP", "IS_DOOROPEN",
		"IS_RBOOK", "IS_QUESTDN", "IS_SARC", "IS_CAST8", "IS_CHEST", "IS_ISCROL", "IS_LEVER", "LS_FOUNTAIN", "LS_PUDDLE", "LS_BLODSTAR")
	for _, id := range ids {
		if cue := sfxCue(id); cue != "" && !strings.Contains(string(b), "internal_name: "+cue+"\n") {
			t.Errorf("sounds.rec has no %s", cue)
		}
	}
}

// Every speech id D1 has needs a voices.rec record, or it can't be remapped with scripts/remap_voices.sh.
func TestSpeechHaveVoiceEvents(t *testing.T) {
	f, err := files.Open("assets/data/voices.rec")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, _ := io.ReadAll(f)
	if len(test.SfxFiles) == 0 {
		t.Fatal("SfxFiles is empty") // TestMain loads it; an empty map would check nothing
	}
	for id := range test.SfxFiles {
		if !strings.Contains(string(b), "internal_name: "+id+"\n") {
			t.Errorf("voices.rec has no %s", id)
		}
	}
}

func TestSoundPosition(t *testing.T) {
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
		att, pan, heard := soundPosition(geometry.Point{X: 10, Y: 10}, geometry.Point{X: 10, Y: 10}.Add(c.d))
		if heard != c.heard || heard && (att != c.att || pan != c.pan) {
			t.Errorf("%v: got %v dB, pan %v, heard %v", c.d, att, pan, heard)
		}
	}
}
