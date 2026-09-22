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
	rules := h.GetWorldState().GetPlayer().Hero().Rules()
	archer, mage := rules.ClassByKey("archer"), rules.ClassByKey("mage")
	if archer < 0 || mage < 0 || rules.ClassByKey("rogue") >= 0 {
		t.Fatalf("H&H class keys: archer %d, mage %d, rogue %d", archer, mage, rules.ClassByKey("rogue"))
	}
	if !rules.FullMissileDamageBonus(archer) || rules.FullMissileDamageBonus(mage) {
		t.Error("full_missile_bonus: the Archer's flag, not the Mage's")
	}
	if got := rules.Class(mage).SpellToHit; got != 20 {
		t.Errorf("Mage spell_to_hit %d, want 20", got)
	}
	// a flag moves with the record: give the Mage the Monk's fists and the Mage fights like a monk
	hero := rules.NewHero(mage)
	hero.Level = 20
	if s := hero.Sheet(nil); s.MaxDam >= 20 {
		t.Fatalf("Mage's bare hands already hit for %d", s.MaxDam)
	}
	rules.SetClassFlagForTest(mage, d1.ClassTrainedFists)
	if s := hero.Sheet(nil); s.MinDam != 10 || s.MaxDam != 20 {
		t.Errorf("a Mage with trained_fists: %d-%d, want 10-20", s.MinDam, s.MaxDam)
	}
}
