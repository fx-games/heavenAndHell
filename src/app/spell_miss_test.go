package app

import (
	"rpg/src/common"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/rules"
	"rpg/src/stats"
	"strings"
	"testing"
)

// A spell carries a to-hit roll (MonsterMHit). A miss used to cost mana and say nothing, which reads as
// "my spells don't work on this monster": every cast that deals no damage has to report itself.
func TestSpellMissIsReported(t *testing.T) {
	g, _ := createGameAndUIContext()
	g.GetAudio().SetDisablePlayback(true)
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "") // Warrior: low Magic, so it misses often
	s := g.GetWorldState()
	s.MapTransition(gridmap.Transition{TargetMap: levelAt(s, 1), TargetLocation: "ladder_up"})
	m, player := s.GetMap(), s.GetPlayer()
	f := g.GetFactory()
	var msgs []string
	prev := f.Msg
	f.Msg = func(msg string) { msgs = append(msgs, msg); prev(msg) }

	var mon *common.Actor
	for _, a := range m.Actors() {
		if a != player && a.Monster() != nil {
			mon = a
			break
		}
	}
	if mon == nil {
		t.Fatal("no monster on level 1")
	}
	g.GetActionBroker().Execute(common.NewSpellAction(player, rules.SplTeleport, 1, mon.MapPosition()))
	for _, d := range geometry.Dirs { // stand beside it, so the bolt cannot be stopped on the way
		if p := mon.MapPosition().Add(geometry.Point{X: d.X, Y: d.Y}); m.IsWalkable(p) && !m.IsActorAt(p) {
			m.MoveActor(player, p)
			break
		}
	}
	hp := mon.GetStats().GetResource(stats.Health)
	misses, casts := 0, 60
	for i := 0; i < casts; i++ {
		mon.GetStats().SetAttribute(stats.MaxHealth, 100000)
		hp.SetToMax()
		msgs = msgs[:0]
		g.GetActionBroker().Execute(common.NewSpellAction(player, rules.SplFirebolt, 1, mon.MapPosition()))
		if hp.Value() < hp.GetMax() {
			continue
		}
		misses++
		found := false
		for _, msg := range msgs {
			found = found || strings.Contains(msg, "misses")
		}
		if !found {
			t.Fatalf("a firebolt that dealt no damage said nothing: %v", msgs)
		}
	}
	if misses == 0 {
		t.Fatal("no miss in 60 casts; the test never exercised the miss path")
	}
}

// A resistant monster takes a quarter of the damage. In whole hit points that quarter used to floor to zero:
// the cast landed, the mana was spent, and the monster took nothing, which is indistinguishable from immunity.
// Charged Bolt rolls 1-2 points for a Warrior (rnd(Mag/4)+1 at Magic 10), so a quarter of it was ALWAYS nothing.
// D1 keeps hit points in 1/64, where a quarter of a point is still a quarter of a point.
func TestResistedSpellStillHurts(t *testing.T) {
	g, _ := createGameAndUIContext()
	g.GetAudio().SetDisablePlayback(true)
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "") // Warrior: Magic 10, so the roll is 1-2
	s := g.GetWorldState()
	s.MapTransition(gridmap.Transition{TargetMap: levelAt(s, 1), TargetLocation: "ladder_up"})
	m, player := s.GetMap(), s.GetPlayer()

	var mon *common.Actor
	for _, a := range m.Actors() {
		if a != player && a.Monster() != nil {
			mon = a
			break
		}
	}
	if mon == nil {
		t.Fatal("no monster on level 1")
	}
	g.GetActionBroker().Execute(common.NewSpellAction(player, rules.SplTeleport, 1, mon.MapPosition()))
	for _, d := range geometry.Dirs {
		if p := mon.MapPosition().Add(geometry.Point{X: d.X, Y: d.Y}); m.IsWalkable(p) && !m.IsActorAt(p) {
			m.MoveActor(player, p)
			break
		}
	}
	mon.Monster().MagicRes = rules.ResistLightning // resisted, not immune
	hp := mon.GetStats().GetResource(stats.Health)
	for i := 0; i < 80; i++ {
		mon.GetStats().SetAttribute(stats.MaxHealth, 100000)
		hp.SetToMax()
		before := hp.Value()
		g.GetActionBroker().Execute(common.NewSpellAction(player, rules.SplChargedBolt, 1, mon.MapPosition()))
		if hp.Value() < before {
			return // a resisted hit took something off, which is all this is about
		}
	}
	t.Fatal("a resisted spell never took a single hit point off in 80 casts: the quarter is rounding away")
}
