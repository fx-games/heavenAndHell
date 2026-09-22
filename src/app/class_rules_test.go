package app

import (
	"testing"

	"rpg/src/d1"
)

// A class's rules are its hero.rec record: whatever a class does differently is a flag or field any class of any
// mod can take, not its position in the roster.
func TestClassRulesAreTheirRecords(t *testing.T) {
	h := newHeavenAndHellGame(t, NewDummyUI())
	h.NewGame(h.GetGameData().ClassDefinitions[0], false, "")
	archer, mage := d1.ClassByKey("archer"), d1.ClassByKey("mage")
	if archer < 0 || mage < 0 || d1.ClassByKey("rogue") >= 0 {
		t.Fatalf("H&H class keys: archer %d, mage %d, rogue %d", archer, mage, d1.ClassByKey("rogue"))
	}
	if !d1.FullMissileDamageBonus(archer) || d1.FullMissileDamageBonus(mage) {
		t.Error("full_missile_bonus: the Archer's flag, not the Mage's")
	}
	if got := d1.Class(mage).SpellToHit; got != 20 {
		t.Errorf("Mage spell_to_hit %d, want 20", got)
	}
	// a flag moves with the record: give the Mage the Monk's fists and the Mage fights like a monk
	hero := d1.NewHero(mage)
	hero.Level = 20
	if s := hero.Sheet(nil); s.MaxDam >= 20 {
		t.Fatalf("Mage's bare hands already hit for %d", s.MaxDam)
	}
	d1.SetClassFlagForTest(mage, d1.ClassTrainedFists)
	if s := hero.Sheet(nil); s.MinDam != 10 || s.MaxDam != 20 {
		t.Errorf("a Mage with trained_fists: %d-%d, want 10-20", s.MinDam, s.MaxDam)
	}
}
