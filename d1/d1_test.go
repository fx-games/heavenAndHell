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
	if len(d.Prefixes) != 83 || len(d.Suffixes) != 95 || len(d.UniqueItems) != 90 || len(d.UniqueMonsters) != 97 {
		t.Fatalf("sizes: pre=%d suf=%d uitems=%d umon=%d", len(d.Prefixes), len(d.Suffixes), len(d.UniqueItems), len(d.UniqueMonsters))
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
	d, err := Load(os.DirFS("../files"))
	if err != nil {
		t.Fatal(err)
	}
	g := NewGenerator(d, 3)
	for lvl := 6; lvl <= 16; lvl += 5 {
		for _, it := range g.SmithStock(lvl) {
			if it.Type == ITypeMisc || it.Type == ITypeStaff || it.Type == ITypeRing || it.Magical != 0 || !it.Identified {
				t.Fatalf("smith sells %+v", it)
			}
		}
		witch := g.WitchStock(lvl, [37]int{})
		if witch[0].MiscID != "IMISC_MANA" || witch[1].MiscID != "IMISC_FULLMANA" || witch[2].Spell != 7 || len(witch) < 10 {
			t.Fatalf("witch stock starts wrong: %s %s %s (%d items)", witch[0].Name, witch[1].Name, witch[2].Name, len(witch))
		}
		book := &Item{MiscID: "IMISC_BOOK", Spell: SplFirebolt}
		var levels [37]int
		levels[SplFirebolt] = 2
		g.witchBookLevel(book, levels)
		if want := d.Spells[SplFirebolt].MinInt; book.MinMag != want+want*20/100+(want+want*20/100)*20/100 {
			t.Fatalf("witch book with spell level 2 needs %d magic (base %d)", book.MinMag, want)
		}
		healer := g.HealerStock(lvl)
		if healer[0].MiscID != "IMISC_HEAL" || healer[1].MiscID != "IMISC_FULLHEAL" {
			t.Fatalf("healer stock starts wrong: %s %s", healer[0].Name, healer[1].Name)
		}
		for _, it := range healer[2:] {
			if it.MiscID == "IMISC_HEAL" || it.MiscID == "IMISC_MANA" {
				t.Fatalf("healer random stock has %s", it.Name)
			}
		}
		if boy := g.BoyItem(lvl); boy.Magical == 0 || boy.IValue > boyMaxVal || WirtPrice(boy) <= BuyPrice(boy) {
			t.Fatalf("wirt item %+v", boy)
		}
	}

	p := &Premium{}
	g.RefillPremium(p, 1)
	third := p.Items[3]
	g.RefillPremium(p, 2)
	if p.Level != 2 || p.Items[1] != third || p.Items[5] == nil {
		t.Fatalf("premium rotation: level %d", p.Level)
	}
	p.Items[4] = nil
	g.RefillPremium(p, 2)
	if p.Items[4] == nil {
		t.Fatal("premium slot not refilled")
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
	if !SmithBuys(sword) || WitchBuys(sword) {
		t.Fatal("sell filters")
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
	for _, n := range []int{8, 9, 13, 24, 61, 62, 80, 82, 83, 87, 88, 89, 91, 94, 95} {
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
