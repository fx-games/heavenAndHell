package rules

import (
	"encoding/json"
	"testing"
)

// A save keeps spell levels by id: a named skill's level comes back even when its row has moved, and an old
// save's array of D1 levels still loads.
func TestSpellLevelsSaveByID(t *testing.T) {
	d, err := Load(testFS())
	if err != nil {
		t.Fatal(err)
	}
	rain, ok := d.SpellIdx("arrow_rain")
	if !ok {
		t.Fatal("the engine's arrow_rain is not in the spell table")
	}
	h := d.NewHero(Warrior)
	h.SpellLevels[rain], h.SpellLevels[SplFirebolt] = 3, 2
	b, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	d.Spells[rain], d.Spells[rain-1] = d.Spells[rain-1], d.Spells[rain] // the table's order changed since the save
	var back Hero
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	d.Adopt(&back)
	if back.SpellLevels[rain-1] != 3 || back.SpellLevels[SplFirebolt] != 2 {
		t.Errorf("after a round trip arrow_rain is %d, firebolt %d; want 3 and 2", back.SpellLevels[rain-1], back.SpellLevels[SplFirebolt])
	}
	var old Hero
	if err := json.Unmarshal([]byte(`{"SpellLevels":[0,4]}`), &old); err != nil {
		t.Fatal(err)
	}
	d.Adopt(&old)
	if old.SpellLevels[SplFirebolt] != 4 || len(old.SpellLevels) != len(d.Spells) {
		t.Errorf("an old save's firebolt is %d over %d slots", old.SpellLevels[SplFirebolt], len(old.SpellLevels))
	}
}

// A scroll or book naming a named skill casts or teaches that skill at a level its item level gives, and a
// skill(id|n) modifier adds n levels to that one skill once the hero knows it.
func TestSkillItems(t *testing.T) {
	d, err := Load(testFS())
	if err != nil {
		t.Fatal(err)
	}
	g := NewGenerator(d, 1)
	rain, _ := d.SpellIdx("arrow_rain")
	for _, misc := range []string{"IMISC_SCROLL", "IMISC_BOOK"} {
		it := g.ItemAttrs(&ItemData{Name: "Tome", MiscID: misc, Spell: "arrow_rain"}, 12, 12, false)
		if it.Spell != rain || it.SpellID != "arrow_rain" || it.SLvl != 1+12/5 || it.Name != "Tome" {
			t.Errorf("%s of arrow_rain: spell %d (%q) at level %d, named %q", misc, it.Spell, it.SpellID, it.SLvl, it.Name)
		}
	}
	ring := g.ItemAttrs(&ItemData{Name: "Ring", Modifiers: []string{"skill(arrow_rain|2)"}}, 5, 5, false)
	h := d.NewHero(Warrior)
	if s := h.Sheet([]*Item{ring}); h.SpellLevel(s, rain) != 0 {
		t.Errorf("the ring teaches arrow_rain to a hero who doesn't know it: level %d", h.SpellLevel(s, rain))
	}
	h.SpellLevels[rain] = 1
	if s := h.Sheet([]*Item{ring}); h.SpellLevel(s, rain) != 3 {
		t.Errorf("arrow_rain 1 with a skill(arrow_rain|2) ring is %d, want 3", h.SpellLevel(s, rain))
	}
}
