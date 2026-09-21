package defs

import (
	"testing"

	"rpg/files"
)

// The whole point of the split: two mods, one loader. Selecting a mod must give you ITS monsters, through the
// same ReadMonsters, with no per-mod branch anywhere between here and the file. If this ever passes because
// both mods read the same records, the counts and the spot checks below stop matching.
func TestEachModLoadsItsOwnMonsters(t *testing.T) {
	for _, tc := range []struct {
		mod    string
		count  int
		want   string // a monster only this mod has
		absent string
	}{
		{"d1", 112, "MT_NZOMBIE", "rat"},
		{"heavenandhell", 43, "rat", "MT_NZOMBIE"},
	} {
		t.Run(tc.mod, func(t *testing.T) {
			files.SelectMod(tc.mod)
			monsters, err := ReadMonsters(files.GetFileSystem())
			if err != nil {
				t.Fatalf("ReadMonsters: %v", err)
			}
			if len(monsters) != tc.count {
				t.Errorf("loaded %d monsters, want %d", len(monsters), tc.count)
			}
			byID := map[string]Monster{}
			for _, m := range monsters {
				byID[m.ID] = m
			}
			m, ok := byID[tc.want]
			if !ok {
				t.Fatalf("%s is missing its own monster %q", tc.mod, tc.want)
			}
			if m.Name == "" || m.Health.Max() <= 0 {
				t.Errorf("%s loaded as an empty shell: %+v", tc.want, m)
			}
			if _, ok := byID[tc.absent]; ok {
				t.Errorf("%s mod served the other mod's monster %q", tc.mod, tc.absent)
			}
		})
	}
}

// D1 stores damage as a min..max range and H&H as dice; both arrive as util.Dice with their own bounds intact.
// This is the schema convergence, so it gets an assertion rather than a comment.
func TestBothSchemasSurviveTheSharedDiceField(t *testing.T) {
	files.SelectMod("d1")
	d1Monsters, err := ReadMonsters(files.GetFileSystem())
	if err != nil {
		t.Fatal(err)
	}
	zombie := d1Monsters[0] // MT_NZOMBIE: 4-7 hp, 2-5 damage, straight out of monstdat
	if zombie.MinHP() != 4 || zombie.MaxHP() != 7 || zombie.MinDam() != 2 || zombie.MaxDam() != 5 {
		t.Errorf("D1 range lost in the dice field: hp %d-%d dam %d-%d, want 4-7 and 2-5",
			zombie.MinHP(), zombie.MaxHP(), zombie.MinDam(), zombie.MaxDam())
	}

	files.SelectMod("heavenandhell")
	hh, err := ReadMonsters(files.GetFileSystem())
	if err != nil {
		t.Fatal(err)
	}
	rat := hh[0] // 1d8 hp, 1d3 damage
	if rat.ID != "rat" || rat.Health.ShortString() != "1d8" || rat.Damage.ShortString() != "1d3" {
		t.Errorf("H&H dice lost: %s hp=%s dam=%s, want rat 1d8 1d3", rat.ID, rat.Health.ShortString(), rat.Damage.ShortString())
	}
	if len(rat.UntargetedSkills) != 1 || rat.UntargetedSkills[0] != "replicate_with" {
		t.Errorf("H&H skills did not survive the port: %v", rat.UntargetedSkills)
	}
}
