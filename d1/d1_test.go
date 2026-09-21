package d1

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

func TestLoadAndGenerate(t *testing.T) {
	d, err := Load(testFS())
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Prefixes) != 86 || len(d.Suffixes) != 98 || len(d.UniqueItems) != 111 || len(d.UniqueMonsters) != 97 {
		t.Fatalf("sizes: pre=%d suf=%d uitems=%d umon=%d", len(d.Prefixes), len(d.Suffixes), len(d.UniqueItems), len(d.UniqueMonsters))
	}
	if len(d.Items) != 166 {
		t.Fatalf("hellfire table %d", len(d.Items))
	}
	// Hellfire's AllItemsList order (1.09's itemdat.cpp under HELLFIRE)
	for idx, name := range map[int]string{5: "Short Staff of Mana", 22: "Map of the Stars", 32: "Bovine Plate", 83: "Blacksmith Oil",
		86: "Oil", 87: "Elixir of Strength", 91: "Scroll of Healing", 92: "Scroll of Vision", 114: "Book of ", 118: "Dagger",
		160: "Amulet", 161: "Fire Trap", 165: "Stone Trap"} {
		if d.Items[idx].Name != name {
			t.Errorf("hellfire item %d is %q, want %q", idx, d.Items[idx].Name, name)
		}
	}
	if m := d.Monsters[0]; m.Name != "Zombie" || m.Family != "Zombie" || d.Monsters[4].Family != "FalSpear" || m.AI != "AI_ZOMBIE" || m.Frames[1] != 24 || m.MagicRes != 72 {
		t.Fatalf("zombie row: %+v", m)
	}
	if blind := d.Texts["TEXT_BLIND1"]; !strings.HasPrefix(blind, "You recite") || strings.Count(blind, "\n\n") != 1 ||
		d.Texts["TEXT_BONER"] == "" || !strings.HasPrefix(d.Texts["TEXT_GARBUD1"], "Pleeeease") || len(d.Gossip["ogden"]) == 0 {
		t.Fatalf("texts: blind=%q boner=%q garbud=%q ogden=%d", blind, d.Texts["TEXT_BONER"], d.Texts["TEXT_GARBUD1"], len(d.Gossip["ogden"]))
	}

	g := NewGenerator(d, 1)
	var magic, uniques, gold, books int
	for dlvl := 1; dlvl <= 16; dlvl++ {
		types := g.LevelMonsterTypes(dlvl)
		if dlvl < 16 && len(types) == 0 {
			t.Fatalf("no monster types on dlvl %d", dlvl)
		}
		for _, mt := range types {
			m := &d.Monsters[mt]
			for n := 0; n < 200; n++ {
				for _, it := range []*Item{g.MonsterDrop(m, m.Level, n%20 == 0, dlvl, dlvl > 12), g.FloorItem(dlvl, n%7 == 0, false)} {
					if it == nil {
						continue
					}
					if it.Name == "" || it.IName == "" || it.IValue < 1 && it.Type != ITypeGold {
						t.Fatalf("bad item %+v", it)
					}
					switch {
					case it.Magical == 2:
						uniques++
						if it.Uid < 0 || it.Identified {
							t.Fatalf("bad unique %+v", it)
						}
					case it.Magical == 1:
						magic++
						if it.IName == it.Name && it.Type != ITypeStaff {
							t.Fatalf("magic item without affix name %+v", it)
						}
					case it.Type == ITypeGold:
						gold++
						if it.Value < 5*dlvl || it.Value > 5000 {
							t.Fatalf("gold %d on dlvl %d", it.Value, dlvl)
						}
					}
					if it.MiscID == "IMISC_BOOK" {
						books++
						if it.Spell == 32 || it.Spell == 34 || !strings.HasPrefix(it.Name, "Book of ") {
							t.Fatalf("bad book %+v", it)
						}
					}
				}
			}
		}
	}
	if magic == 0 || uniques == 0 || gold == 0 || books == 0 {
		t.Fatalf("distribution magic=%d uniques=%d gold=%d books=%d", magic, uniques, gold, books)
	}
	t.Logf("magic=%d uniques=%d gold=%d books=%d", magic, uniques, gold, books)
}

func TestHero(t *testing.T) {
	w := NewHero(Warrior)
	if w.MaxHPBase>>6 != 70 || w.MaxManaBase>>6 != 10 {
		t.Fatalf("warrior start hp=%d mana=%d", w.MaxHPBase>>6, w.MaxManaBase>>6)
	}
	sorc := NewHero(Sorcerer)
	if sorc.MaxHPBase>>6 != 30 || sorc.MaxManaBase>>6 != 70 {
		t.Fatalf("sorcerer start hp=%d mana=%d", sorc.MaxHPBase>>6, sorc.MaxManaBase>>6)
	}

	sword := &Item{Type: ITypeSword, Class: "ICLASS_WEAPON", MinDam: 2, MaxDam: 6, MinStr: 18}
	heavy := &Item{Type: ITypeHArmor, Class: "ICLASS_ARMOR", AC: 40, MinStr: 90}
	ring := &Item{Type: ITypeRing, Class: "ICLASS_MISC", PLStr: 60, Magical: 1}
	s := w.Sheet([]*Item{sword, heavy})
	if !s.Active[0] || s.Active[1] || s.AC != 0 || s.MinDam != 2 || s.WeaponType != ITypeSword {
		t.Fatalf("requirements not applied: %+v", s)
	}
	if s = w.Sheet([]*Item{sword, heavy, ring}); s.Active[1] || s.Str != 30 {
		t.Fatalf("an unidentified ring must not count: %+v", s)
	}
	ring.Identified = true
	s = w.Sheet([]*Item{sword, heavy, ring})
	if !s.Active[1] || s.AC != 40 || s.Str != 90 {
		t.Fatalf("ring should enable heavy armor: %+v", s)
	}
	if hc := w.MeleeHitChance(s, 0); hc != 81 { // 10 (dex/2) + 1 (level) + 50 + 20 (warrior)
		t.Fatalf("warrior hit chance vs AC0 = %d", hc)
	}

	gained := w.AddExperience(1, 2000)
	if gained != 1 || w.Level != 2 || w.StatPts != 5 || w.MaxHPBase != 70<<6+129 {
		t.Fatalf("level up: gained=%d %+v", gained, w)
	}
	if !w.SpendStatPoint(AttrVit) || w.MaxHPBase != 70<<6+129+128 || w.BaseVit != 26 {
		t.Fatalf("vit point: %+v", w)
	}
}

func TestStores(t *testing.T) {
	d, err := Load(testFS())
	if err != nil {
		t.Fatal(err)
	}
	{
		g := NewGenerator(d, 3)
		lim := g.limits()
		for lvl := 6; lvl <= 16; lvl += 5 {
			smith := g.SmithStock(lvl)
			for _, it := range smith {
				if it.Type == ITypeMisc || it.Type == ITypeStaff && it.Spell != 0 || it.Type == ITypeRing || it.Magical != 0 || !it.Identified {
					t.Fatalf("smith sells %+v", it)
				}
			}
			if len(smith) > lim.smithItems-1 {
				t.Fatalf("smith has %d items", len(smith))
			}
			witch := g.WitchStock(lvl, [NumSpells]int{})
			if witch[0].MiscID != "IMISC_MANA" || witch[1].MiscID != "IMISC_FULLMANA" || witch[2].Spell != 7 || len(witch) < 10 || len(witch) >= lim.witchItems {
				t.Fatalf("witch stock starts wrong: %s %s %s (%d items)", witch[0].Name, witch[1].Name, witch[2].Name, len(witch))
			}
			for _, it := range witch {
				if IsOil(it.MiscID) {
					t.Fatalf("witch sells %s", it.Name)
				}
			}
			book := &Item{MiscID: "IMISC_BOOK", Spell: SplFirebolt}
			var levels [NumSpells]int
			levels[SplFirebolt] = 2
			g.witchBookLevel(book, levels)
			if want := d.Spells[SplFirebolt].MinInt; book.MinMag != want+want*20/100+(want+want*20/100)*20/100 {
				t.Fatalf("witch book with spell level 2 needs %d magic (base %d)", book.MinMag, want)
			}
			maxed := &Shopper{Class: Warrior, BaseStr: 250} // Hellfire: no Elixir of Strength for a maxed Warrior
			healer := g.HealerStock(lvl, maxed)
			if healer[0].MiscID != "IMISC_HEAL" || healer[1].MiscID != "IMISC_FULLHEAL" {
				t.Fatalf("healer stock starts wrong: %s %s", healer[0].Name, healer[1].Name)
			}
			for _, it := range healer[2:] {
				if it.MiscID == "IMISC_HEAL" || it.MiscID == "IMISC_MANA" || it.MiscID == "IMISC_ELIXSTR" {
					t.Fatalf("healer random stock has %s", it.Name)
				}
			}
			rogue := &Shopper{Class: Rogue, Str: 20, Mag: 15, Dex: 30}
			boy := g.BoyItem(lvl, rogue)
			if boy.Magical == 0 || boy.IValue > lim.boyMaxVal || d.WirtPrice(boy) >= BuyPrice(boy) {
				t.Fatalf("wirt item %+v at %d", boy, d.WirtPrice(boy))
			}
		}

		p := &Premium{}
		g.RefillPremium(p, 1, nil)
		shift := 3
		moved := p.Items[shift+1]
		g.RefillPremium(p, 2, nil)
		if p.Level != 2 || p.Items[1] != moved || p.Items[lim.premiumItems-1] == nil || lim.premiumItems < premiumSlots && p.Items[lim.premiumItems] != nil {
			t.Fatalf("premium rotation: level %d", p.Level)
		}
		p.Items[4] = nil
		g.RefillPremium(p, 2, nil)
		if p.Items[4] == nil {
			t.Fatal("premium slot not refilled")
		}

		sword := &Item{Type: ITypeSword, Class: "ICLASS_WEAPON", Value: 400, IValue: 400, MaxDur: 20, Durability: 1}
		if !d.SmithBuys(sword) || d.WitchBuys(sword) {
			t.Fatal("sell filters")
		}
		oil := &Item{Type: ITypeMisc, MiscID: "IMISC_OILOF", Class: "ICLASS_MISC"}
		if !d.SmithBuys(oil) || d.WitchBuys(oil) {
			t.Fatal("oil sell filters")
		}
	}

	sword := &Item{Type: ITypeSword, Class: "ICLASS_WEAPON", Value: 400, IValue: 400, MaxDur: 20, Durability: 1}
	if SellPrice(sword) != 100 || RepairCost(sword) != 190 {
		t.Fatalf("prices: sell %d repair %d", SellPrice(sword), RepairCost(sword))
	}
	if broke := WeaponDur(rand.New(rand.NewSource(1)), []*Item{sword}, 1); broke != sword || sword.Durability != 0 {
		t.Fatalf("weapon should break, durability %d", sword.Durability)
	}
	if s := NewHero(Warrior).Sheet([]*Item{sword}); s.Active[0] || s.MinDam != 1 {
		t.Fatalf("broken weapon still counts: %+v", s)
	}
}

func TestSpells(t *testing.T) {
	d, err := Load(testFS())
	if err != nil {
		t.Fatal(err)
	}
	sorc, rogue, warrior := NewHero(Sorcerer), NewHero(Rogue), NewHero(Warrior)
	if sorc.SpellLevels[SplFirebolt] != 2 {
		t.Fatalf("sorcerer starts with firebolt level %d", sorc.SpellLevels[SplFirebolt])
	}
	rogue.SpellLevels[SplFirebolt] = 2
	for _, c := range []struct {
		h           *Hero
		spell, mana int
	}{{sorc, SplFirebolt, 6}, {rogue, SplFirebolt, 4}, {warrior, SplHeal, 7}} {
		if got := d.ManaCost(c.h, Sheet{}, c.spell) >> 6; got != c.mana {
			t.Errorf("class %d spell %d costs %d mana, want %d", c.h.Class, c.spell, got, c.mana)
		}
	}
	if len(NovaRays) != 36 {
		t.Errorf("nova sends %d rays, want 36", len(NovaRays))
	}
	if g := GolemStats(1, 1, 0); g.HP != 10 || g.Hit != 47 || g.MinDam != 10 || g.MaxDam != 18 {
		t.Errorf("golem %+v", g)
	}
	if StoneTurns(1) != 14 || FireWallTurns(0, 0) != 20 {
		t.Errorf("stone %d turns, fire wall %d turns", StoneTurns(1), FireWallTurns(0, 0))
	}
}

func TestPotionRangeMatchesRolls(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, class := range []int{Warrior, Rogue, Sorcerer} {
		h := &Hero{Class: class}
		maxV := 80 << 6
		hlo, hhi := h.PotionHealRange(maxV)
		mlo, mhi := h.PotionManaRange(maxV)
		for range 500 {
			if v := h.PotionHeal(rng, maxV); v < hlo || v > hhi {
				t.Fatalf("class %d heal %d outside %d-%d", class, v, hlo, hhi)
			}
			if v := h.PotionMana(rng, maxV); v < mlo || v > mhi {
				t.Fatalf("class %d mana %d outside %d-%d", class, v, mlo, mhi)
			}
		}
	}
}

func TestSpellDescriptionsMatchNames(t *testing.T) {
	if len(SpellDescriptions) != len(spellNames) {
		t.Fatalf("%d descriptions for %d spells", len(SpellDescriptions), len(spellNames))
	}
}

func TestQuestItemIndices(t *testing.T) {
	d, err := Load(testFS())
	if err != nil {
		t.Fatal(err)
	}
	for idx, name := range map[int]string{IdiRock: "Magic Rock", IdiOptAmulet: "Optic Amulet", IdiBanner: "Tavern Sign",
		IdiGldnElix: "Golden Elixir", IdiAnvil: "Anvil of Fury", IdiMushroom: "Black Mushroom", IdiBrain: "Brain",
		IdiFungalTm: "Fungal Tome", IdiSpecElix: "Spectral Elixir", IdiBldStone: "Blood Stone", IdiArmOfVal: "Arkaine's Valor",
		IdiLazStaff: "Staff of Lazarus"} {
		if d.Items[idx].Name != name {
			t.Errorf("item %d is %q, want %q", idx, d.Items[idx].Name, name)
		}
	}
	for uid, name := range map[int]string{UItemInfraRing: "Empyrean Band", UItemOptAmulet: "Optic Amulet", UItemTRing: "Ring of Truth",
		UItemHarCrest: "Harlequin Crest", UItemSteelVeil: "Veil of Steel", UItemArmOfVal: "Arkaine's Valor", UItemGriswold: "Griswold's Edge"} {
		if d.UniqueItems[uid].Name != name {
			t.Errorf("unique %d is %q, want %q", uid, d.UniqueItems[uid].Name, name)
		}
	}
}

// TestClassSkills checks Item Repair and Staff Recharge: they refill at the cost of the maximum, and wear it out.
func TestClassSkills(t *testing.T) {
	zero := func(int) int { return 0 }
	it := &Item{Durability: 10, MaxDur: 40}
	if RepairItem(it, 10, zero) || it.MaxDur != 35 || it.Durability != 35 {
		t.Errorf("repair: %d/%d", it.Durability, it.MaxDur)
	}
	if worn := (&Item{Durability: 0, MaxDur: 1}); !RepairItem(worn, 1, zero) {
		t.Errorf("repair didn't wear out a 0/1 item")
	}
	d, err := Load(testFS())
	if err != nil {
		t.Fatal(err)
	}
	staff := &Item{Type: ITypeStaff, Spell: SplFirebolt, Charges: 0, MaxCharges: 10}
	d.RechargeItem(staff, 1, zero) // one charge per round: 1 of 9, ... until the charges reach the shrinking maximum
	if staff.MaxCharges != 5 || staff.Charges != 5 {
		t.Errorf("recharge: %d/%d", staff.Charges, staff.MaxCharges)
	}
}

// TestTextSpeech checks that every text's speech and every hero line the quests play is in SfxFiles.
func TestTextSpeech(t *testing.T) {
	d, err := Load(testFS())
	if err != nil {
		t.Fatal(err)
	}
	for id, sfx := range d.TextSfx {
		if _, ok := SfxFiles[sfx]; !ok {
			t.Errorf("%s: speech %q has no file", id, sfx)
		}
	}
	for _, n := range []int{8, 9, 13, 14, 15, 16, 24, 27, 35, 61, 69, 71, 62, 80, 82, 83, 87, 88, 89, 91, 94, 95, 97, 98, 99} {
		for _, class := range []string{"WARR", "ROGUE", "MAGE"} {
			if id := "PS_" + class + fmt.Sprint(n); SfxFiles[id] == "" {
				t.Errorf("hero line %s has no file", id)
			}
		}
	}
	if len(d.Gossip["farnham"]) < 2 || len(d.Gossip["gillian"]) < 2 {
		t.Errorf("gossip: farnham %d, gillian %d", len(d.Gossip["farnham"]), len(d.Gossip["gillian"]))
	}
}

func TestHealRangeBoundsRolls(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, class := range []int{Warrior, Rogue, Sorcerer} {
		lo, hi := HealRange64(class, 7, 3)
		seenLo, seenHi := 1<<30, 0
		for range 20000 {
			v := HealAmount64(rng, class, 7, 3)
			seenLo, seenHi = min(seenLo, v), max(seenHi, v)
		}
		if seenLo < lo || seenHi > hi {
			t.Fatalf("class %d: rolls %d-%d outside %d-%d", class, seenLo, seenHi, lo, hi)
		}
	}
}

// TestHellfireClasses pins the three Hellfire classes' starting pools and the Barbarian's Rage state machine,
// which is the only class skill with a lasting effect.
func TestHellfireClasses(t *testing.T) {
	for _, c := range []struct{ class, hp, mana int }{
		{Warrior, 70, 10}, {Rogue, 45, 22}, {Sorcerer, 30, 70},
		{Monk, 45, 22}, {Bard, 45, 35}, {Barbarian, 70, 0},
	} {
		h := NewHero(c.class)
		if h.MaxHPBase>>6 != c.hp || h.MaxManaBase>>6 != c.mana {
			t.Errorf("%s starts with hp=%d mana=%d, want %d/%d", ClassNames[c.class], h.MaxHPBase>>6, h.MaxManaBase>>6, c.hp, c.mana)
		}
	}

	h := NewHero(Barbarian)
	h.Level = 10
	calm := h.Sheet(nil)
	if calm.FR != 10 || calm.BonusAC != 2 { // natural resistance and iron skin scale with the level
		t.Errorf("calm barbarian: resist %d, bonus AC %d, want 10 and 2", calm.FR, calm.BonusAC)
	}
	h.Rage = RageTurns(h.Level)
	raging := h.Sheet(nil)
	if raging.Str != calm.Str+20 || raging.Dex != calm.Dex+15 || raging.Vit != calm.Vit+20 || raging.MaxHP <= calm.MaxHP {
		t.Errorf("raging barbarian %d/%d/%d hp %d, calm %d/%d/%d hp %d",
			raging.Str, raging.Dex, raging.Vit, raging.MaxHP, calm.Str, calm.Dex, calm.Vit, calm.MaxHP)
	}
	h.Rage = -RageTurns(h.Level)
	hung := h.Sheet(nil)
	if hung.Str != calm.Str-20 || hung.Dex != calm.Dex-15 || hung.Vit != calm.Vit-20 || hung.FR != 0 {
		t.Errorf("hungover barbarian %d/%d/%d resist %d", hung.Str, hung.Dex, hung.Vit, hung.FR)
	}

	// the Monk fights bare-handed, blocks with his staff and swings it faster than anyone
	monk := NewHero(Monk)
	monk.Level = 20
	fists := monk.Sheet(nil)
	if fists.MinDam != 10 || fists.MaxDam != 20 || !fists.Block || fists.AttackFrames != 12 {
		t.Errorf("monk's fists: %d-%d damage, block=%v, %d frames", fists.MinDam, fists.MaxDam, fists.Block, fists.AttackFrames)
	}
	staff := monk.Sheet([]*Item{{Type: ITypeStaff, Class: "ICLASS_WEAPON", Loc: "ILOC_TWOHAND", MinDam: 2, MaxDam: 4}})
	if !staff.Block || staff.Flags&ISplFastBlock == 0 || staff.AttackFrames != 13 {
		t.Errorf("monk's staff: block=%v flags=%x %d frames", staff.Block, staff.Flags, staff.AttackFrames)
	}
}

// TestHellfireItems covers the expansion's item work: a plain Oil settles into one of the ten, and each oil
// edits only what it is allowed to.
func TestHellfireItems(t *testing.T) {
	d, err := Load(testFS())
	if err != nil {
		t.Fatal(err)
	}
	// a plain "Oil" must never stay plain
	g := NewGenerator(d, 1)
	g.R.SetSeed(99)
	oilIdx := -1
	for i, b := range d.Items {
		if b.MiscID == "IMISC_OILOF" {
			oilIdx = i
		}
	}
	if oilIdx < 0 {
		t.Fatal("no IMISC_OILOF in the item table")
	}
	for _, lvl := range []int{1, 5, 20} {
		it := g.PlainItem(oilIdx)
		if it.MiscID == "IMISC_OILOF" || it.Value == 0 || !IsOil(it.MiscID) {
			t.Fatalf("lvl %d: oil stayed %s (%s, %d gold)", lvl, it.MiscID, it.Name, it.Value)
		}
	}

	rng := rand.New(rand.NewSource(7))
	armor := func() *Item {
		return &Item{Class: "ICLASS_ARMOR", Type: ITypeHArmor, AC: 10, Durability: 5, MaxDur: 20}
	}
	weapon := func() *Item {
		return &Item{Class: "ICLASS_WEAPON", Type: ITypeSword, MinDam: 4, MaxDam: 8, Durability: 5, MaxDur: 20}
	}
	if ApplyOil("IMISC_OILSHARP", armor(), rng.Intn) || ApplyOil("IMISC_OILHARD", weapon(), rng.Intn) {
		t.Error("an oil was poured on something it cannot improve")
	}
	if a := armor(); !ApplyOil("IMISC_OILHARD", a, rng.Intn) || a.AC <= 10 {
		t.Errorf("hardening left the armour at %d", a.AC)
	}
	if w := weapon(); !ApplyOil("IMISC_OILSHARP", w, rng.Intn) || w.MaxDam != 9 {
		t.Errorf("sharpness left the damage at %d-%d", w.MinDam, w.MaxDam)
	}
	if w := weapon(); !ApplyOil("IMISC_OILBSMTH", w, rng.Intn) || w.Durability != 9 {
		t.Errorf("blacksmith oil left durability at %d/%d", w.Durability, w.MaxDur)
	}
	if w := weapon(); !ApplyOil("IMISC_OILPERM", w, rng.Intn) || w.MaxDur != DurIndestructible {
		t.Errorf("permanence left durability at %d/%d", w.Durability, w.MaxDur)
	}
	if q := (&Item{Class: "ICLASS_QUEST"}); ApplyOil("IMISC_OILBSMTH", q, rng.Intn) {
		t.Error("a quest item took an oil")
	}
}

// MI_Cbolt: a spark heading +x (dir 7, bpath 0) covers 3 tiles in the 17 ticks before its first turn.
func TestChargedBoltFlight(t *testing.T) {
	f := ChargedBoltFlight(7, 1)
	for i, p := range f[:17] {
		if p[1] != 0 {
			t.Fatalf("tick %d: %v left the x axis", i, p)
		}
	}
	if f[16] != [2]int{3, 0} || len(f) != 256 {
		t.Fatalf("after 17 ticks at %v, %d ticks", f[16], len(f))
	}
}

// GetMissilePos at velocity 16: 2-3 ticks a tile along an axis, 4 on a diagonal (a Nova ball or a Flame Wave
// hits a monster that often).
func TestMissileFlightTicksPerTile(t *testing.T) {
	for _, c := range []struct{ dx, dy, lo, hi int }{{1, 0, 2, 3}, {1, 1, 4, 4}} {
		count := map[[2]int]int{}
		for _, p := range MissileFlight(c.dx, c.dy, 16, 60) {
			count[p]++
		}
		for p, n := range count {
			if p != [2]int{} && (p[0] < 10 && p[1] < 10) && (n < c.lo || n > c.hi) {
				t.Errorf("(%d,%d): %d ticks on %v", c.dx, c.dy, n, p)
			}
		}
	}
}

// MI_Manashield: spell level 1 pays 4/9 of the damage, level 7 and up 40/63.
func TestManaShieldCost(t *testing.T) {
	if got := ManaShieldCost(630, 0); got != 630 {
		t.Errorf("slvl 0: %d", got)
	}
	if got := ManaShieldCost(630, 1); got != 280 {
		t.Errorf("slvl 1: %d", got)
	}
	if got := ManaShieldCost(630, 9); got != 400 {
		t.Errorf("slvl 9: %d", got)
	}
}

func TestTypedSpellsNameTheirDamageType(t *testing.T) {
	for spell, kind := range SpellResist {
		if kind == ResistNone {
			continue
		}
		want := [...]string{"", "fire", "lightning", "magic"}[kind]
		if text := SpellDamageText(spell, 50, 10, 3); !strings.Contains(strings.ToLower(text), want) {
			t.Errorf("spell %d says %q, want it to name %s", spell, text, want)
		}
	}
}

func TestDamagingSpellsShowARange(t *testing.T) {
	for _, spell := range []int{SplInferno, SplFlash, SplFireWall, SplLightWall, SplRingOfFire, SplGuardian} {
		if _, hi, ok := SpellDamageRange(spell, 50, 10, 3); !ok || hi <= 0 {
			t.Errorf("spell %d has no damage range", spell)
		}
	}
}

// TestDamageRollsTolerateInvertedRange guards the trapped-chest crash: MI_LArrow rolls mind and maxd
// independently, so mind can exceed maxd; D1's GenerateRnd returns 0 there instead of panicking.
func TestDamageRollsTolerateInvertedRange(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	var s Sheet
	if got := MonsterDamage(rng, s, 15, 14); got != 15<<6 {
		t.Fatalf("MonsterDamage(15,14) = %d, want %d", got, 15<<6)
	}
	if got := TrapDamage64(rng, s, 15, 14, false); got != 15<<6 {
		t.Fatalf("TrapDamage64(15,14) = %d, want %d", got, 15<<6)
	}
	if got := MissileDamage64(rng, s, 15, 14, true); got != 64 {
		t.Fatalf("MissileDamage64(15,14,shift) = %d, want 64", got)
	}
}
