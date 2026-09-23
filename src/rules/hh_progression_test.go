package rules

import (
	"os"
	"testing"

	"rpg/files"
)

// Heaven & Hell's loot rolls at item level 2x depth, 2..24 over its twelve levels. Everything its tables define must
// be reachable in that range: each base some level drops, each suffix some level and base can roll, each unique a
// base that drops at its min_level; and every item level must have a suffix to roll, or magic items die out.
func TestHeavenAndHellLootIsReachable(t *testing.T) {
	if _, err := os.Stat("../../files/mods/heavenandhell"); err != nil {
		t.Skip("no heavenandhell mod")
	}
	files.SetFileSystem(os.DirFS("../../files").(files.FileSystem))
	files.SelectMod("heavenandhell")
	defer files.SelectMod("d1")
	d, err := Load(files.GetFileSystem())
	if err != nil {
		t.Fatal(err)
	}
	const lo, hi = 2, 24
	plt := map[int]int{ITypeSword: PLTWeap, ITypeAxe: PLTWeap, ITypeMace: PLTWeap, ITypeBow: PLTBow, ITypeShield: PLTShld,
		ITypeStaff: PLTStaff, ITypeLArmor: PLTArmo, ITypeHelm: PLTArmo, ITypeRing: PLTMisc, ITypeAmulet: PLTMisc}
	dropsSomewhere := func(b *ItemData, from, to int) bool {
		for l := from; l <= to; l++ {
			if dropsAt(b, l) {
				return true
			}
		}
		return false
	}
	for i := range d.Items {
		if b := &d.Items[i]; b.Drop != "IDROP_NEVER" && b.Type != ITypeGold && !dropsSomewhere(b, lo, hi) {
			t.Errorf("%s (dlvl %d, max_level %d) drops at no item level %d-%d", b.InternalName, b.MinMLvl, b.MaxLevel, lo, hi)
		}
	}
	for _, a := range d.Suffixes {
		ok := false
		for i := range d.Items {
			b := &d.Items[i]
			ok = ok || b.Drop != "IDROP_NEVER" && plt[b.Type]&a.IType != 0 && dropsSomewhere(b, max(a.MinLvl, lo), min(2*a.MinLvl, hi))
		}
		if !ok {
			t.Errorf("suffix %q (%s, min level %d) has no base to roll on", a.Name, a.Power, a.MinLvl)
		}
	}
	for l := lo; l <= hi; l++ {
		n := 0
		for _, a := range d.Suffixes {
			if a.MinLvl >= l/2 && a.MinLvl <= l {
				n++
			}
		}
		if n == 0 {
			t.Errorf("item level %d rolls no suffix", l)
		}
	}
	for _, u := range d.UniqueItems {
		ok := false
		for i := range d.Items {
			b := &d.Items[i]
			ok = ok || b.UniqueBase == u.Base && b.Drop != "IDROP_NEVER" && dropsSomewhere(b, max(u.MinLvl, lo), hi)
		}
		if !ok {
			t.Errorf("unique %s: no base %s drops from its min_level %d on", u.ID, u.Base, u.MinLvl)
		}
	}
}
