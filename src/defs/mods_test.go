package defs

import (
	"slices"
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
		{"heavenandhell", 86, "rat", "MT_NZOMBIE"},
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
		{"d1", 143, "The Undead Crown", "Potion of Berserker Rage"}, // 1.09's table without its empty slots
		{"heavenandhell", 109, "Potion of Berserker Rage", "The Undead Crown"},
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
func TestModSuppliesEveryIcon(t *testing.T) {
	files.SelectMod("d1")
	if err := LoadIcons(); err != nil {
		t.Fatal(err)
	}
}

// Every icon the engine requires of a mod, H&H supplies from its own sheet.
func TestHeavenAndHellModSuppliesEveryIcon(t *testing.T) {
	files.SelectMod("heavenandhell")
	err := LoadIcons()
	files.SelectMod("d1") // leave the icon vars pointing at a complete mod
	if err != nil {
		t.Error(err)
	}
	if err := LoadIcons(); err != nil {
		t.Fatal(err)
	}
}

// LoadIcons no longer demands D1's dungeon furniture of every mod, so D1 has to be held to it here.
// Ranges ("4-7") and dice ("1d8") share the field; both arrive as util.Dice with their own bounds intact.
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
	rat := hh[0] // 1-2 hp, 1-3 damage: H&H's stats follow D1's curve, as ranges too
	if rat.ID != "rat" || rat.MinHP() != 1 || rat.MaxHP() != 2 || rat.MinDam() != 1 || rat.MaxDam() != 3 {
		t.Errorf("H&H range lost: %s hp %d-%d dam %d-%d, want rat 1-2 and 1-3", rat.ID, rat.MinHP(), rat.MaxHP(), rat.MinDam(), rat.MaxDam())
	}
	if len(rat.UntargetedSkills) != 1 || rat.UntargetedSkills[0] != "replicate_with" {
		t.Errorf("H&H skills did not survive the port: %v", rat.UntargetedSkills)
	}
}

// Stores were the last hardcoded content boundary: D1 picked its stock with a Go predicate over the item table
// and H&H listed it in a .rec. One ReadVendors serves both now, so both halves of that record get checked here --
// and, the point of the split, each mod's vendors must reach its OWN items and none of the other mod's.
func TestEachModStocksItsOwnItems(t *testing.T) {
	for _, tc := range []struct {
		mod       string
		vendors   int
		namesFrom string // a vendor that names its stock outright
		want      string // an item only this mod has, which some vendor of it can stock
		absent    string // the other mod's item, which none of its vendors may reach
	}{
		{"d1", 6, "witch", "Potion of Full Mana", "Potion of Berserker Rage"},
		{"heavenandhell", 4, "store_general", "a torch", "Potion of Full Mana"},
	} {
		t.Run(tc.mod, func(t *testing.T) {
			files.SelectMod(tc.mod)
			vendors, err := ReadVendors(files.GetFileSystem())
			if err != nil {
				t.Fatalf("ReadVendors: %v", err)
			}
			if len(vendors) != tc.vendors {
				t.Errorf("loaded %d vendors, want %d", len(vendors), tc.vendors)
			}
			items, err := ReadItems(files.GetFileSystem())
			if err != nil {
				t.Fatal(err)
			}
			byName := map[string]Item{}
			for _, it := range items {
				byName[it.InternalName] = it
			}
			// The list half: every item a vendor names has to be one of this mod's, or the shop sells nothing.
			named := 0
			for _, v := range vendors {
				for _, name := range slices.Concat(v.Always, v.Sometimes, v.Items) {
					if _, ok := byName[name]; !ok {
						t.Errorf("%s stocks %q, which this mod's items.rec does not have", v.InternalName, name)
					}
					named++
				}
			}
			if named == 0 {
				t.Error("no vendor names a single item")
			}
			if !slices.ContainsFunc(vendors, func(v Vendor) bool { return v.InternalName == tc.namesFrom }) {
				t.Errorf("%s has no vendor %q", tc.mod, tc.namesFrom)
			}
			// The filter half, and the split itself: what the criteria reach is drawn from this mod's table and
			// nowhere else, so the other mod's items are out of reach however the criteria are written.
			reach := map[string]bool{}
			for _, v := range vendors {
				for _, name := range slices.Concat(v.Always, v.Sometimes, v.Items) {
					reach[byName[name].Name] = true
				}
				if len(v.Types) == 0 && len(v.ExcludeTypes) == 0 {
					continue // a pure list vendor, with no criteria to run
				}
				for _, it := range items {
					if it.Drop != "IDROP_NEVER" && v.Accepts(it) {
						reach[it.Name] = true
					}
				}
			}
			if !reach[tc.want] {
				t.Errorf("%s vendors cannot reach their own %q", tc.mod, tc.want)
			}
			if reach[tc.absent] {
				t.Errorf("%s vendors reach the other mod's %q", tc.mod, tc.absent)
			}
		})
	}
}
