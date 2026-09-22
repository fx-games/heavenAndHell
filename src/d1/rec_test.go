package d1

import (
	"slices"
	"testing"
)

// The tables below moved out of Go source into mods/d1/data/*.rec. These are the values the literals had, spot
// checked at the ends and the awkward middles, so a mangled .rec fails here rather than in the game.
// TestRecTablesMatchTheOldLiterals reads the tables with Hellfire's rows (test fixtures: the D1 mod has none), so
// every table the engine supports is checked, not only the ones 1.09 fills.
func TestRecTablesMatchTheOldLiterals(t *testing.T) {
	d := loadHellfire(t)

	if len(d.oils) != 10 {
		t.Fatalf("oils = %d, want 10", len(d.oils))
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
		got := d.oils[c.i]
		if got.MiscID != c.miscID || got.Name != c.name || got.MinLvl != c.lvl || got.Value != c.value {
			t.Errorf("oils[%d] = %+v, want %s/%s/%d/%d", c.i, got, c.miscID, c.name, c.lvl, c.value)
		}
	}

	if len(spellNames) != NumSpells || len(SpellDescriptions) != NumSpells {
		t.Fatalf("spell slots = %d names / %d descriptions, want %d each",
			len(spellNames), len(SpellDescriptions), NumSpells)
	}
	// Slot 0 is D1's "no spell" and must stay blank, or every unset spell would name itself.
	if spellNames[0] != "" || SpellDescriptions[0] != "" {
		t.Errorf("slot 0 = %q/%q, want blank", spellNames[0], SpellDescriptions[0])
	}
	if spellNames[SplFirebolt] != "Firebolt" || spellNames[SplRage] != "Rage" ||
		spellNames[NumSpells-1] != "Stone Trap" || spellNames[16] != "Doom Serpents" {
		t.Errorf("spellNames did not survive spells.rec")
	}
	if SpellDescriptions[SplFirebolt] != "Hurls a bolt of fire at the target." {
		t.Errorf("SpellDescriptions[Firebolt] = %q", SpellDescriptions[SplFirebolt])
	}

	if len(d.RuneTraps) != 5 || d.RuneTraps["IMISC_RUNEF"] != (RuneTrap{SplRuneFire, "a fire explosion"}) ||
		d.RuneTraps["IMISC_GR_RUNEL"] != (RuneTrap{SplRuneNova, "a lightning nova"}) {
		t.Errorf("RuneTraps did not survive spells.rec: %v", d.RuneTraps)
	}

	if len(d.MonsterAIDescriptions) != 35 || d.MonsterAIDescriptions["AI_ZOMBIE"] != "Shambles slowly toward you." ||
		d.MonsterAIDescriptions["AI_LACHDAN"] != "Never fights." {
		t.Errorf("MonsterAIDescriptions did not survive monsters.rec (%d entries)", len(d.MonsterAIDescriptions))
	}

	if len(d.ObjectDisplays) != 89 {
		t.Errorf("ObjectDisplays = %d, want 89", len(d.ObjectDisplays))
	}
	for _, c := range []struct {
		kind int
		want ObjectDisplay
	}{
		{ObjCHEST1, ObjectDisplay{Name: "Small Chest", Container: true, Sfx: "IS_CHEST"}},
		{ObjL1LIGHT, ObjectDisplay{WallLight: true}},
		{ObjSKFIRE, ObjectDisplay{Name: "Skull Fire", Light: 5}},
		{ObjGOATSHRINE, ObjectDisplay{Name: "Goat Shrine", Sfx: "LS_GSHRINE"}},
		{ObjBCROSS, ObjectDisplay{Name: "Burning Cross", Light: 10}},
	} {
		if got := d.ObjectDisplays[c.kind]; got != c.want {
			t.Errorf("ObjectDisplays[%d] = %+v, want %+v", c.kind, got, c.want)
		}
	}

	if len(d.propertyDefs) != 41 {
		t.Errorf("propertyDefs = %d, want 41", len(d.propertyDefs))
	}
	// One of each formatter kind, including the light radius, the only property that scales its value.
	for _, c := range []struct {
		p    Property
		want string
	}{
		{Property{"str", 5, 5}, "+5 to strength"},
		{Property{KeyDamage, 6, 9}, "Damage: 6-9"},
		{Property{"knockback", 1, 1}, "knocks target back"},
		{Property{"attackspeed", 3, 3}, "faster attack"},
		{Property{"light", 2, 2}, "+20% light radius"},
	} {
		if got := d.PropertyText(c.p); got != c.want {
			t.Errorf("%s text = %q, want %q", c.p.Key, got, c.want)
		}
	}
	if d.propertyDefs["gethit"].better != -1 || d.propertyDefs["rndarrowvel"].better != 0 || d.propertyDefs["str"].better != 1 {
		t.Error("property better did not survive properties.rec")
	}

	shuns := d.Vendors["boy"].Shuns
	if len(shuns) != 6 || len(shuns["Rogue"]) != 5 ||
		shuns["Warrior"][0] != "bow" || shuns["Warrior"][1] != "staff" || shuns["Monk"][1] != "marmor" {
		t.Errorf("Wirt's shuns did not survive stores.rec: %v", shuns)
	}

	if len(SpellResist) != 25 || len(SpellTargeted) != 20 || len(SpellSfx) != 45 {
		t.Errorf("spell tables = %d/%d/%d, want 25/20/45", len(SpellResist), len(SpellTargeted), len(SpellSfx))
	}
	if SpellResist[SplFirebolt] != ResistKindFire || SpellResist[SplChain] != ResistKindLightning ||
		SpellResist[SplBoneSpirit] != ResistKindMagic || SpellResist[SplRuneStone] != ResistKindMagic {
		t.Error("SpellResist did not survive spells.rec")
	}
	if !SpellTargeted[SplFirebolt] || !SpellTargeted[SplBerserk] || SpellTargeted[SplHeal] {
		t.Error("SpellTargeted did not survive spells.rec")
	}
	if SpellSfx[SplFirebolt] != [3]string{"IS_CAST2", "LS_FBOLT1", "LS_FIRIMP2"} ||
		SpellSfx[SplHeal] != [3]string{"IS_CAST8", "", ""} ||
		SpellSfx[SplHolyBolt] != [3]string{"IS_CAST2", "LS_HOLYBOLT", "LS_ELECIMP1"} {
		t.Errorf("SpellSfx did not survive spells.rec: %v", SpellSfx[SplFirebolt])
	}
	// SpellFrames and MaxStats are derived from the class table; both used to be computed at package init, which
	// would now run before it is loaded.
	// Six is D1's OWN roster, not the engine's: the tables are sized by how many classes this mod declares.
	if !slices.Equal(d.SpellFrames, []int{20, 16, 12, 18, 16, 20}) {
		t.Errorf("SpellFrames = %v", d.SpellFrames)
	}

	if !slices.Equal(d.ClassNames, []string{"Warrior", "Rogue", "Sorcerer", "Monk", "Bard", "Barbarian"}) {
		t.Errorf("ClassNames = %v", d.ClassNames)
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
		if got := c.got(d.classAttr[c.class]); got != c.want {
			t.Errorf("%s = %d, want %d", c.what, got, c.want)
		}
	}
	if got := d.classAttr[Sorcerer].AttackFrames; got[0] != 20 || got[ITypeAxe] != 24 || got[ITypeStaff] != 16 || len(got) != 5 {
		t.Errorf("sorcerer AttackFrames = %v", got)
	}
	if !d.HasClassFlag(Barbarian, ClassIronSkin) || !d.HasClassFlag(Barbarian, ClassNaturalResistance) ||
		!d.HasClassFlag(Rogue, ClassTrapSense) || d.HasClassFlag(Sorcerer, ClassCriticalStrike) {
		t.Error("class flags did not survive hero.rec")
	}
	if d.MaxStats[Warrior] != [4]int{250, 50, 60, 100} || d.MaxStats[Barbarian] != [4]int{255, 0, 55, 150} {
		t.Errorf("MaxStats = %v", d.MaxStats)
	}
	if d.ExpLvls[0] != 0 || d.ExpLvls[1] != 2000 || d.ExpLvls[50] != 1583495809 {
		t.Errorf("ExpLvls ends = %d/%d/%d", d.ExpLvls[0], d.ExpLvls[1], d.ExpLvls[50])
	}

	if len(d.ShrineNames) != 34 || len(d.ShrineMessages) != 34 || len(d.shrineMax) != 34 || len(d.shrineAvail) != 34 {
		t.Fatalf("shrine tables = %d/%d/%d/%d, want 34 each",
			len(d.ShrineNames), len(d.ShrineMessages), len(d.shrineMax), len(d.shrineAvail))
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
		if d.ShrineNames[c.i] != c.name || d.shrineMax[c.i] != c.max || d.shrineAvail[c.i] != c.avail {
			t.Errorf("shrine %d = %s/%d/%d, want %s/%d/%d",
				c.i, d.ShrineNames[c.i], d.shrineMax[c.i], d.shrineAvail[c.i], c.name, c.max, c.avail)
		}
	}
	if len(d.StoryBookNames) != 9 || d.StoryBookNames[0] != "The Great Conflict" || d.StoryBookNames[8] != "The Black King" {
		t.Errorf("StoryBookNames = %v", d.StoryBookNames)
	}

	if len(d.SfxFiles) != 746 {
		t.Errorf("SfxFiles = %d entries, want 746", len(d.SfxFiles))
	}
	for id, want := range map[string]string{
		"IS_SARC":      "items/sarc.ogg",
		"PS_MONK1":     "monk/monk01.ogg",
		"USFX_DIABLOD": "monsters/diablod.ogg",
	} {
		if d.SfxFiles[id] != want {
			t.Errorf("SfxFiles[%s] = %q, want %q", id, d.SfxFiles[id], want)
		}
	}

	if len(d.itemCAnimTbl) != 229 || len(d.itemDropSnds) != 43 || len(d.itemInvSnds) != 43 {
		t.Fatalf("item sound tables = %d/%d/%d, want 229/43/43", len(d.itemCAnimTbl), len(d.itemDropSnds), len(d.itemInvSnds))
	}
	for _, c := range []struct{ curs, anim int }{{0, 20}, {4, 4}, {41, 0}, {150, 14}, {228, 0}} {
		if d.itemCAnimTbl[c.curs] != c.anim {
			t.Errorf("itemCAnimTbl[%d] = %d, want %d", c.curs, d.itemCAnimTbl[c.curs], c.anim)
		}
	}
	if d, i := d.ItemSnds(4); d != "IS_GOLD" || i != "IS_GOLD" { // gold, anim 4
		t.Errorf("ItemSnds(4) = %q/%q, want IS_GOLD/IS_GOLD", d, i)
	}
	if d, i := d.ItemSnds(0); d != "IS_FPOT" || i != "IS_IPOT" { // anim 20
		t.Errorf("ItemSnds(0) = %q/%q, want IS_FPOT/IS_IPOT", d, i)
	}
	if d, i := d.ItemSnds(-1); d != "" || i != "" {
		t.Errorf("ItemSnds(-1) = %q/%q, want empty", d, i)
	}
	if d, i := d.ItemSnds(len(d.itemCAnimTbl)); d != "" || i != "" {
		t.Errorf("ItemSnds(out of range) = %q/%q, want empty", d, i)
	}
}
