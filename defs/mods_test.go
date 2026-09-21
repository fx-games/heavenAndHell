package defs

import (
	"strings"
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

// Same again for items: one ReadItems, and each mod gets its own.
func TestEachModLoadsItsOwnItems(t *testing.T) {
	for _, tc := range []struct {
		mod    string
		count  int
		want   string
		absent string
	}{
		{"d1", 168, "The Undead Crown", "Potion of Berserker Rage"},
		{"heavenandhell", 113, "Potion of Berserker Rage", "The Undead Crown"},
	} {
		t.Run(tc.mod, func(t *testing.T) {
			files.SelectMod(tc.mod)
			items, err := ReadItems(files.GetFileSystem())
			if err != nil {
				t.Fatalf("ReadItems: %v", err)
			}
			if len(items) != tc.count {
				t.Errorf("loaded %d items, want %d", len(items), tc.count)
			}
			byName := map[string]Item{}
			for _, it := range items {
				byName[it.Name] = it
			}
			if it, ok := byName[tc.want]; !ok {
				t.Errorf("%s is missing its own item %q", tc.mod, tc.want)
			} else if it.Icon == "" {
				t.Errorf("%q has no icon ID", tc.want)
			}
			if _, ok := byName[tc.absent]; ok {
				t.Errorf("%s mod served the other mod's item %q", tc.mod, tc.absent)
			}
		})
	}
}

// The D1 mod supplies every icon the engine names; that is the state a finished mod is in.
func TestD1ModSuppliesEveryIcon(t *testing.T) {
	files.SelectMod("d1")
	if err := LoadIcons(); err != nil {
		t.Fatal(err)
	}
}

// The H&H mod does not yet, and this is the exact list, so finishing it is a shrinking list rather than a
// guess and so a name quietly appearing or vanishing shows up here. Six are classes H&H does not have (monk,
// bard and barbarian are Hellfire's) and the rest are dungeon furniture and a missile the engine names because
// D1 draws them -- both are the engine's fixed Icon* list still being shaped like one game's content.
func TestHeavenAndHellModIconGapIsExactlyThis(t *testing.T) {
	want := []string{
		"class.monk_male", "class.monk_female", "class.bard_male", "class.bard_female",
		"class.barbarian_male", "class.barbarian_female",
		"tile.wooden_fence", "tile.arch", "tile.grate", "tile.pillar", "tile.lamp", "tile.lava",
		"tile.wall_torch", "tile.brazier", "effect.arrow",
	}
	files.SelectMod("heavenandhell")
	err := LoadIcons()
	if err == nil {
		t.Fatal("the mod now supplies every icon: delete this test and its list")
	}
	got := err.Error()
	for _, name := range want {
		if !strings.Contains(got, name) {
			t.Errorf("%q is supplied now; remove it from the list", name)
		}
	}
	if n := strings.Count(got, ", ") + 1; n != len(want) {
		t.Errorf("%d icons missing, the list expects %d: %v", n, len(want), err)
	}
	files.SelectMod("d1") // leave the icon vars pointing at a complete mod
	if err := LoadIcons(); err != nil {
		t.Fatal(err)
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
