package d1

import (
	"os"
	"testing"
)

// The tables below moved out of Go source into mods/d1/data/*.rec. These are the values the literals had, spot
// checked at the ends and the awkward middles, so a mangled .rec fails here rather than in the game.
func TestRecTablesMatchTheOldLiterals(t *testing.T) {
	if _, err := Load(os.DirFS("../files")); err != nil {
		t.Fatal(err)
	}

	if len(oils) != 10 {
		t.Fatalf("oils = %d, want 10", len(oils))
	}
	for _, c := range []struct {
		i      int
		miscID string
		name   string
		lvl    int
		value  int
	}{
		{0, "IMISC_OILACC", "Oil of Accuracy", 1, 500},
		{5, "IMISC_OILBSMTH", "Blacksmith Oil", 1, 100},
		{7, "IMISC_OILPERM", "Oil of Permanence", 17, 15000},
		{9, "IMISC_OILIMP", "Oil of Imperviousness", 10, 2500},
	} {
		got := oils[c.i]
		if got.MiscID != c.miscID || got.Name != c.name || got.MinLvl != c.lvl || got.Value != c.value {
			t.Errorf("oils[%d] = %+v, want %s/%s/%d/%d", c.i, got, c.miscID, c.name, c.lvl, c.value)
		}
	}

	if ClassNames != [NumClasses]string{"Warrior", "Rogue", "Sorcerer", "Monk", "Bard", "Barbarian"} {
		t.Errorf("ClassNames = %v", ClassNames)
	}
	// The 1/64 fixed-point columns are the ones a .rec round trip could quietly round off, so check the
	// fractional ones and a whole one against what fp() produced from the literal.
	for _, c := range []struct {
		class int
		got   func(classAttributes) int
		want  int
		what  string
	}{
		{Warrior, func(a classAttributes) int { return a.AdjLife }, fp(18), "warrior adj_life"},
		{Warrior, func(a classAttributes) int { return a.AdjMana }, fp(-1), "warrior adj_mana"},
		{Rogue, func(a classAttributes) int { return a.AdjMana }, fp(5.5), "rogue adj_mana"},
		{Rogue, func(a classAttributes) int { return a.ManaCost }, fp(0.75), "rogue mana_cost"},
		{Bard, func(a classAttributes) int { return a.ItmMana }, fp(1.75), "bard itm_mana"},
		{Bard, func(a classAttributes) int { return a.ChrMana }, fp(1.5), "bard chr_mana"},
		{Barbarian, func(a classAttributes) int { return a.ItmLife }, fp(2.5), "barbarian itm_life"},
	} {
		if got := c.got(classAttr[c.class]); got != c.want {
			t.Errorf("%s = %d, want %d", c.what, got, c.want)
		}
	}
	if got := classAttr[Sorcerer].AttackFrames; got[0] != 20 || got[ITypeAxe] != 24 || got[ITypeStaff] != 16 || len(got) != 5 {
		t.Errorf("sorcerer AttackFrames = %v", got)
	}
	if !HasClassFlag(Barbarian, ClassIronSkin) || !HasClassFlag(Barbarian, ClassNaturalResistance) ||
		!HasClassFlag(Rogue, ClassTrapSense) || HasClassFlag(Sorcerer, ClassCriticalStrike) {
		t.Error("class flags did not survive hero.rec")
	}
	if MaxStats[Warrior] != [4]int{250, 50, 60, 100} || MaxStats[Barbarian] != [4]int{255, 0, 55, 150} {
		t.Errorf("MaxStats = %v", MaxStats)
	}
	if ExpLvls[0] != 0 || ExpLvls[1] != 2000 || ExpLvls[50] != 1583495809 {
		t.Errorf("ExpLvls ends = %d/%d/%d", ExpLvls[0], ExpLvls[1], ExpLvls[50])
	}

	if len(ShrineNames) != 34 || len(ShrineMessages) != 34 || len(shrineMax) != 34 || len(shrineAvail) != 34 {
		t.Fatalf("shrine tables = %d/%d/%d/%d, want 34 each",
			len(ShrineNames), len(ShrineMessages), len(shrineMax), len(shrineAvail))
	}
	// The generator rolls an index into these, so the level caps and the multiplayer-only markers decide which
	// shrines a dungeon level can hold.
	for _, c := range []struct {
		i          int
		name       string
		max, avail int
	}{
		{0, "Mysterious", 24, 0},
		{7, "Enchanted", 8, 0}, // the one shrine with a shallower cap
		{18, "Spooky", 24, 2},  // multiplayer only
		{25, "Tainted", 24, 2}, // multiplayer only
		{32, "Solar", 24, 1},
		{33, "Murphy's", 24, 0},
	} {
		if ShrineNames[c.i] != c.name || shrineMax[c.i] != c.max || shrineAvail[c.i] != c.avail {
			t.Errorf("shrine %d = %s/%d/%d, want %s/%d/%d",
				c.i, ShrineNames[c.i], shrineMax[c.i], shrineAvail[c.i], c.name, c.max, c.avail)
		}
	}
	if len(StoryBookNames) != 9 || StoryBookNames[0] != "The Great Conflict" || StoryBookNames[8] != "The Black King" {
		t.Errorf("StoryBookNames = %v", StoryBookNames)
	}

	if len(SfxFiles) != 746 {
		t.Errorf("SfxFiles = %d entries, want 746", len(SfxFiles))
	}
	for id, want := range map[string]string{
		"IS_SARC":      "items/sarc.ogg",
		"PS_MONK1":     "monk/monk01.ogg",
		"USFX_DIABLOD": "monsters/diablod.ogg",
	} {
		if SfxFiles[id] != want {
			t.Errorf("SfxFiles[%s] = %q, want %q", id, SfxFiles[id], want)
		}
	}

	if len(itemCAnimTbl) != 229 || len(itemDropSnds) != 43 || len(itemInvSnds) != 43 {
		t.Fatalf("item sound tables = %d/%d/%d, want 229/43/43", len(itemCAnimTbl), len(itemDropSnds), len(itemInvSnds))
	}
	for _, c := range []struct{ curs, anim int }{{0, 20}, {4, 4}, {41, 0}, {150, 14}, {228, 0}} {
		if itemCAnimTbl[c.curs] != c.anim {
			t.Errorf("itemCAnimTbl[%d] = %d, want %d", c.curs, itemCAnimTbl[c.curs], c.anim)
		}
	}
	if d, i := ItemSnds(4); d != "IS_GOLD" || i != "IS_GOLD" { // gold, anim 4
		t.Errorf("ItemSnds(4) = %q/%q, want IS_GOLD/IS_GOLD", d, i)
	}
	if d, i := ItemSnds(0); d != "IS_FPOT" || i != "IS_IPOT" { // anim 20
		t.Errorf("ItemSnds(0) = %q/%q, want IS_FPOT/IS_IPOT", d, i)
	}
	if d, i := ItemSnds(-1); d != "" || i != "" {
		t.Errorf("ItemSnds(-1) = %q/%q, want empty", d, i)
	}
	if d, i := ItemSnds(len(itemCAnimTbl)); d != "" || i != "" {
		t.Errorf("ItemSnds(out of range) = %q/%q, want empty", d, i)
	}
}
