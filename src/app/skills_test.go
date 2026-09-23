package app

import (
	"fmt"
	"rpg/src/common"
	"rpg/src/geometry"
	"rpg/src/rules"
	"rpg/src/stats"
	"strings"
	"testing"
)

// Every engine skill (src/rules/skills.rec) cast by a class that doesn't start with it, next to a monster, does
// something: it hurts or afflicts a monster, moves the hero, or puts a status on him.
func TestEngineSkillsWork(t *testing.T) {
	g := newHeavenAndHellGame(t, NewDummyUI())
	g.GetAudio().SetDisablePlayback(true)
	g.NewGame(g.GetGameData().ClassDefinitions[1], false, "") // the Archer, who starts with a bow
	s := g.GetWorldState()
	down, ok := s.GetMap().GetTransitionAt(s.GetMap().GetNamedLocation("to_cathedral"))
	if !ok {
		t.Fatal("no way down from town")
	}
	s.MapTransition(down)
	player, d := s.GetPlayer(), g.GetFactory().Generator.D
	snapshot := func() string {
		out := fmt.Sprint(player.MapPosition(), player.GetStatusEffects().AllEntries(), player.GetStats().GetResource(stats.Health).Value(), len(s.GetMap().Objects()))
		for _, a := range s.GetMap().Actors() {
			if a != player {
				out += fmt.Sprint(a.GetStats().GetResource(stats.Health).Value(), a.MapPosition(), a.GetStatusEffects().AllEntries(), len(a.GetParametrizedStatusEffects()))
			}
		}
		return out
	}
	for spell := rules.NumSpells; spell < len(d.Spells); spell++ {
		sd := d.Spells[spell]
		if sd.Sustained {
			continue // TestSustainedAndReadiedSkills
		}
		t.Run(sd.ID, func(t *testing.T) {
			changed := false
			for try := 0; try < 20 && !changed; try++ {
				m := s.GetMap()
				// a monster with two free cells beside it: the hero's, and one to aim a move at; each try another
				type spot struct {
					mon          *common.Actor
					beside, free geometry.Point
				}
				var spots []spot
				for _, a := range m.Actors() {
					var cells []geometry.Point
					for _, dir := range geometry.Dirs {
						if p := a.MapPosition().Add(dir); a != player && a.IsAlive() && a.IsHostileTo(player) && m.IsWalkable(p) && !m.IsActorAt(p) && !m.IsObjectAt(p) {
							cells = append(cells, p)
						}
					}
					for i := 0; i+1 < len(cells); i++ {
						spots = append(spots, spot{a, cells[i], cells[len(cells)-1-i]})
					}
				}
				if len(spots) == 0 {
					t.Fatal("no monster left with free cells beside it")
				}
				sp := spots[(try*7)%len(spots)]
				mon, beside, free := sp.mon, sp.beside, sp.free
				m.MoveActor(player, beside)
				g.GetActionBroker().Execute(common.NewUpdateFoVAction(common.AbstractCause("test"), player)) // Shout scares whom he sees
				mon.AddStatusEffect(common.StatusStone, 3)                                                   // every blow and arrow lands
				player.GetStats().GetResource(stats.Health).SetToMax()
				player.GetStats().GetResource(stats.Health).Decrease(5) // room for a heal
				before := snapshot()
				target := mon.MapPosition()
				switch {
				case !sd.Targeted, sd.ID == "shield": // Shield lands on the hero or an ally, never on a foe
					target = player.MapPosition()
				case sd.Targeting == "direct" && sd.ID == "blink", strings.HasPrefix(sd.Targeting, "earthshaker"):
					target = free // a move needs a free cell to land on
				}
				g.GetActionBroker().Execute(common.NewSpellAction(player, spell, 3, target))
				changed = snapshot() != before
			}
			if !changed {
				t.Errorf("%s did nothing in 20 casts", sd.Name)
			}
		})
	}
}

// Multi-Shot is a toggle, cast on and off; Eternal Vow survives one
// lethal blow; Debilitating Shot afflicts the foe the next arrow hits.
func TestSustainedAndReadiedSkills(t *testing.T) {
	g := newHeavenAndHellGame(t, NewDummyUI())
	g.GetAudio().SetDisablePlayback(true)
	g.NewGame(g.GetGameData().ClassDefinitions[1], false, "")
	s := g.GetWorldState()
	down, _ := s.GetMap().GetTransitionAt(s.GetMap().GetNamedLocation("to_cathedral"))
	s.MapTransition(down)
	player, d, f := s.GetPlayer(), g.GetFactory().Generator.D, g.GetFactory()
	multi, _ := d.SpellIdx("multi_shot")
	f.CastFromMemory(player, multi, player.MapPosition())
	if !player.HasStatusEffect(common.StatusMultiShot) {
		t.Fatal("casting Multi-Shot did not raise it")
	}
	f.CastFromMemory(player, multi, player.MapPosition())
	if player.HasStatusEffect(common.StatusMultiShot) {
		t.Fatal("casting Multi-Shot again did not lower it")
	}

	vow, _ := d.SpellIdx("eternal_vow")
	g.GetActionBroker().Execute(common.NewSpellAction(player, vow, 1, player.MapPosition()))
	g.GetActionBroker().Execute(common.NewChangeResourceAction(common.AbstractCause("test"), player, stats.Health, -100000))
	if hp := player.GetStats().GetResource(stats.Health).Value(); hp != 1 {
		t.Errorf("under Eternal Vow a lethal blow left %d life, want 1", hp)
	}
	if player.HasStatusEffect(common.StatusEternalVow) {
		t.Error("the vow held but did not end")
	}

	deb, _ := d.SpellIdx("debilitating_shot")
	g.GetActionBroker().Execute(common.NewSpellAction(player, deb, 2, player.MapPosition()))
	if !player.HasStatusEffect(common.StatusDebilitating) {
		t.Fatal("Debilitating Shot readied nothing")
	}
	m := s.GetMap()
	for _, a := range m.Actors() {
		if a == player || !a.IsHostileTo(player) {
			continue
		}
		a.AddStatusEffect(common.StatusStone, 3)
		for i := 0; i < 10 && player.HasStatusEffect(common.StatusDebilitating) && a.IsAlive(); i++ {
			g.GetActionBroker().Execute(common.NewRangedAttackAction(common.AbstractCause("test"), player, a.MapPosition()))
		}
		break
	}
	if player.HasStatusEffect(common.StatusDebilitating) {
		t.Error("no arrow used up the readied Debilitating Shot")
	}
}
