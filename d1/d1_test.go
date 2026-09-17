package d1

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
)

func TestLoadAndGenerate(t *testing.T) {
	d, err := Load(os.DirFS("../files"))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Prefixes) != 86 || len(d.Suffixes) != 98 || len(d.UniqueItems) != 111 || len(d.UniqueMonsters) != 100 {
		t.Fatalf("sizes: pre=%d suf=%d uitems=%d umon=%d", len(d.Prefixes), len(d.Suffixes), len(d.UniqueItems), len(d.UniqueMonsters))
	}
	// with Hellfire off the tables must roll exactly like D1 1.09's, which is what TestGoldenLevels pins
	d1only, err := LoadD1(os.DirFS("../files"))
	if err != nil {
		t.Fatal(err)
	}
	hfPre, hfSuf, hfItems := 0, 0, 0
	for _, a := range d1only.Prefixes {
		if a.Hellfire {
			hfPre++
		}
	}
	for _, a := range d1only.Suffixes {
		if a.Hellfire {
			hfSuf++
		}
	}
	for _, b := range d1only.Items {
		if b.Hellfire {
			hfItems++
		}
	}
	hfUniq := 0
	for _, u := range d1only.UniqueItems {
		if u.Hellfire {
			hfUniq++
		}
	}
	if hfPre != 3 || hfSuf != 3 || hfUniq != 21 || hfItems != 12 || len(d.Items) != 166 {
		t.Fatalf("hellfire rows: pre=%d suf=%d uniques=%d items=%d, hellfire table %d", hfPre, hfSuf, hfUniq, hfItems, len(d.Items))
	}
	// Hellfire's AllItemsList order (1.09's itemdat.cpp under HELLFIRE)
	for idx, name := range map[int]string{5: "Short Staff of Mana", 22: "Cathedral Map", 32: "Bovine Plate", 83: "Blacksmith Oil",
		86: "Oil", 87: "Elixir of Strength", 91: "Scroll of Healing", 92: "Scroll of Search", 114: "Book of ", 118: "Dagger",
		160: "Amulet", 161: "Rune of Fire", 165: "Rune of Stone"} {
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

	if SpeedTier(RefWalkTicks, 24) != 0.5 || SpeedTier(RefWalkTicks, 8) != 1 || SpeedTier(RefAttackTicks, 12) != 1.5 {
		t.Fatal("speed tiers")
	}
}

func TestStores(t *testing.T) {
	d1only, err := LoadD1(os.DirFS("../files"))
	if err != nil {
		t.Fatal(err)
	}
	hf, err := Load(os.DirFS("../files"))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []*Data{d1only, hf} {
		g := NewGenerator(d, 3)
		lim := g.limits()
		for lvl := 6; lvl <= 16; lvl += 5 {
			smith := g.SmithStock(lvl)
			for _, it := range smith {
				if it.Type == ITypeMisc || it.Type == ITypeStaff && (!d.Hellfire || it.Spell != 0) || it.Type == ITypeRing || it.Magical != 0 || !it.Identified {
					t.Fatalf("hellfire %v: smith sells %+v", d.Hellfire, it)
				}
			}
			if len(smith) > lim.smithItems-1 {
				t.Fatalf("hellfire %v: smith has %d items", d.Hellfire, len(smith))
			}
			witch := g.WitchStock(lvl, [NumSpells]int{})
			if witch[0].MiscID != "IMISC_MANA" || witch[1].MiscID != "IMISC_FULLMANA" || witch[2].Spell != 7 || len(witch) < 10 || len(witch) >= lim.witchItems {
				t.Fatalf("hellfire %v: witch stock starts wrong: %s %s %s (%d items)", d.Hellfire, witch[0].Name, witch[1].Name, witch[2].Name, len(witch))
			}
			for _, it := range witch {
				if d.Hellfire && IsOil(it.MiscID) {
					t.Fatalf("hellfire witch sells %s", it.Name)
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
				if it.MiscID == "IMISC_HEAL" || it.MiscID == "IMISC_MANA" || d.Hellfire && it.MiscID == "IMISC_ELIXSTR" {
					t.Fatalf("hellfire %v: healer random stock has %s", d.Hellfire, it.Name)
				}
			}
			rogue := &Shopper{Class: Rogue, Str: 20, Mag: 15, Dex: 30}
			boy := g.BoyItem(lvl, rogue)
			if boy.Magical == 0 || boy.IValue > lim.boyMaxVal || (d.WirtPrice(boy) < BuyPrice(boy)) != d.Hellfire {
				t.Fatalf("hellfire %v: wirt item %+v at %d", d.Hellfire, boy, d.WirtPrice(boy))
			}
		}

		p := &Premium{}
		g.RefillPremium(p, 1, nil)
		shift := 2
		if d.Hellfire {
			shift = 3
		}
		moved := p.Items[shift+1]
		g.RefillPremium(p, 2, nil)
		if p.Level != 2 || p.Items[1] != moved || p.Items[lim.premiumItems-1] == nil || lim.premiumItems < premiumSlots && p.Items[lim.premiumItems] != nil {
			t.Fatalf("hellfire %v: premium rotation: level %d", d.Hellfire, p.Level)
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
		if d.Hellfire && (!d.SmithBuys(oil) || d.WitchBuys(oil)) {
			t.Fatal("hellfire oil sell filters")
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
	d, err := Load(os.DirFS("../files"))
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
	if StoneTurns(1) != 14 || FireWallTurns(0) != 20 {
		t.Errorf("stone %d turns, fire wall %d turns", StoneTurns(1), FireWallTurns(0))
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
	d, err := Load(os.DirFS("../files"))
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
	d, err := Load(os.DirFS("../files"))
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
	d, err := Load(os.DirFS("../files"))
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

// TestHellfireItems covers the expansion's item work: the toggle keeps D1's drop tables intact, a plain Oil
// settles into one of the ten, and each oil edits only what it is allowed to.
func TestHellfireItems(t *testing.T) {
	d, err := Load(os.DirFS("../files"))
	if err != nil {
		t.Fatal(err)
	}
	// Hellfire's droppable items reach the floor only with the toggle on
	seen := map[string]bool{}
	for _, on := range []bool{false, true} {
		d.Hellfire = on
		g := NewGenerator(d, 1)
		g.R.SetSeed(12345)
		for n := 0; n < 20000; n++ {
			if it := g.FloorItem(8, n%5 == 0, false); it != nil && d.Items[it.BaseIdx].Hellfire {
				seen[map[bool]string{false: "off", true: "on"}[on]] = true
			}
		}
	}
	if seen["off"] || !seen["on"] {
		t.Errorf("hellfire items dropped with the toggle off=%v, on=%v", seen["off"], seen["on"])
	}

	// a plain "Oil" must never stay plain
	d.Hellfire = true
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
