package d1

import (
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

	g := NewGenerator(d, rand.New(rand.NewSource(1)))
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

func TestGenerateL1Connected(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for n := 0; n < 200; n++ {
		dlvl := n%4 + 1
		lvl := GenerateL1(rng, dlvl)
		seen := map[[2]int]bool{lvl.StairsUp: true}
		queue := [][2]int{lvl.StairsUp}
		floor := 0
		for x := 0; x < LevelSize; x++ {
			for y := 0; y < LevelSize; y++ {
				if lvl.Walkable(x, y) {
					floor++
				}
			}
		}
		for len(queue) > 0 {
			p := queue[0]
			queue = queue[1:]
			for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				q := [2]int{p[0] + d[0], p[1] + d[1]}
				if lvl.Walkable(q[0], q[1]) && !seen[q] {
					seen[q] = true
					queue = append(queue, q)
				}
			}
		}
		if !seen[lvl.StairsDn] {
			t.Fatalf("level %d (dlvl %d): stairs down unreachable\n%s", n, dlvl, render(lvl))
		}
		if len(seen) < floor*9/10 {
			t.Fatalf("level %d (dlvl %d): only %d of %d walkable cells reachable\n%s", n, dlvl, len(seen), floor, render(lvl))
		}
	}
}

func render(l *Level) string {
	var b strings.Builder
	for y := 0; y < LevelSize; y++ {
		for x := 0; x < LevelSize; x++ {
			switch {
			case [2]int{x, y} == l.StairsUp:
				b.WriteByte('<')
			case [2]int{x, y} == l.StairsDn:
				b.WriteByte('>')
			default:
				b.WriteByte("#.+"[l.Cells[x][y]])
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
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
	g := NewGenerator(d, rand.New(rand.NewSource(3)))
	for lvl := 6; lvl <= 16; lvl += 5 {
		for _, it := range g.SmithStock(lvl) {
			if it.Type == ITypeMisc || it.Type == ITypeStaff || it.Type == ITypeRing || it.Magical != 0 || !it.Identified {
				t.Fatalf("smith sells %+v", it)
			}
		}
		witch := g.WitchStock(lvl)
		if witch[0].MiscID != "IMISC_MANA" || witch[1].MiscID != "IMISC_FULLMANA" || witch[2].Spell != 7 || len(witch) < 10 {
			t.Fatalf("witch stock starts wrong: %s %s %s (%d items)", witch[0].Name, witch[1].Name, witch[2].Name, len(witch))
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
