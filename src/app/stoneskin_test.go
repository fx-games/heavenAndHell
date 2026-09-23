package app

import (
	"rpg/src/common"
	"strings"
	"testing"
)

// Stoneskin is one status: its armor class, its slow step and the sheet's note come and go with it.
func TestStoneskinIsItsStatus(t *testing.T) {
	g := newHeavenAndHellGame(t, NewDummyUI())
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	p := g.GetWorldState().GetPlayer()
	step := p.MoveCost()
	p.AddStatusEffect(common.StatusStoneSkin, 5)
	s := p.HeroSheet()
	if ac := s.AC + s.BonusAC + s.Dex/5; ac < common.StoneSkinAC || p.MoveCost() != 2*step {
		t.Errorf("stoneskin: armor class %d, step %d (was %d)", ac, p.MoveCost(), step)
	}
	if rows := p.SheetSections()[2].Rows; !strings.Contains(rows[0][1], "Stoneskin, 5 turns") {
		t.Errorf("sheet does not say why: %v", rows[0])
	}
	p.RemoveStatusEffect(common.StatusStoneSkin)
	if s := p.HeroSheet(); s.AC+s.BonusAC+s.Dex/5 >= common.StoneSkinAC || p.MoveCost() != step {
		t.Error("stoneskin outlasted its status")
	}
}

// Bless is its status too: +5 armor class, or double it from 5 up, and the sheet says so.
func TestBlessIsItsStatus(t *testing.T) {
	g := newHeavenAndHellGame(t, NewDummyUI())
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	p := g.GetWorldState().GetPlayer()
	s := p.HeroSheet()
	ac := s.AC + s.BonusAC + s.Dex/5
	p.AddStatusEffect(common.StatusBlessed, 3)
	s = p.HeroSheet()
	if got := s.AC + s.BonusAC + s.Dex/5; got != ac+max(5, ac) {
		t.Errorf("blessed armor class %d, want %d", got, ac+max(5, ac))
	}
	if rows := p.SheetSections()[2].Rows; !strings.Contains(rows[0][1], "Blessed, 3 turns") {
		t.Errorf("sheet does not say why: %v", rows[0])
	}
}
