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
