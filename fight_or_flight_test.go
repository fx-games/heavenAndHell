package main

import (
	"rpg/common"
	"rpg/defs"
	"rpg/game"
	"rpg/geometry"
	"rpg/gridmap"
	"testing"
)

// Fight or flight, both halves on one monster: a monster the player shoots at from outside its own aggro range
// has to come for them (a zombie plinked from six tiles away used to wander off - its AI only homes in from
// four), and one that breaks has to gain ground while it runs instead of milling about.
//
// One test, one game: the audio context is a process-wide singleton, so a second game in the same process panics.
func TestAttackedMonsterFightsOrFlees(t *testing.T) {
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[1], false, true, "")
	state := theGame.GetWorldState()
	player := state.GetPlayer()
	hp := player.GetStats().GetResource("health")
	dist := func(a *common.Actor) int { return geometry.DistanceChebyshev(player.MapPosition(), a.MapPosition()) }

	monster := parkPlayerSixTilesFromAMonster(t, theGame)
	mon := monster.D1Monster()

	// fight: one arrow, and it should close the six tiles
	theGame.GetActions().RangedAttackAt(monster.MapPosition())
	if !monster.IsAlive() {
		t.Skip("the shot killed it")
	}
	if !mon.Enraged {
		t.Fatal("being shot at did not enrage the monster")
	}
	closest := 6
	for turn := 0; turn < 40 && monster.IsAlive(); turn++ {
		hp.SetToMax() // measuring the approach, not whether we survive it
		state.PlayerTookTurn(10)
		closest = min(closest, dist(monster))
	}
	// Crowding can keep it a few tiles out, so this only asks that it came for the player at all.
	if closest > 3 {
		t.Fatalf("%s (%s) never came for the player who shot it: closest %d of 6", monster.Name(), mon.AI, closest)
	}
	if !monster.IsAlive() {
		return
	}

	// flight: the same monster breaks, and has to actually get away
	mon.Flee, mon.Last = 6, player.MapPosition()
	start, farthest, ranFor := dist(monster), dist(monster), 0
	for turn := 0; turn < 40 && monster.IsAlive(); turn++ {
		hp.SetToMax()
		state.PlayerTookTurn(10)
		if mon.Flee > 0 {
			ranFor++
			farthest = max(farthest, dist(monster))
		}
	}
	if ranFor == 0 {
		return // cornered on turn one, so flee() handed it back to its AI to fight: the other branch
	}
	if farthest <= start {
		t.Fatalf("%s (%s) fled for %d turns without gaining ground on the player: %d away, started at %d",
			monster.Name(), mon.AI, ranFor, farthest, start)
	}
	if mon.Flee != 0 {
		t.Fatalf("still fleeing 40 turns on, Flee=%d", mon.Flee)
	}
}

// parkPlayerSixTilesFromAMonster puts the player six tiles from the first monster it can see from there.
func parkPlayerSixTilesFromAMonster(t *testing.T, theGame *game.GameInstance) *common.Actor {
	t.Helper()
	state := theGame.GetWorldState()
	player := state.GetPlayer()
	for dlvl := 1; dlvl <= 3; dlvl++ {
		state.MapTransition(gridmap.Transition{TargetMap: defs.LevelNameFromDepth(dlvl), TargetLocation: "ladder_up"})
		m := state.GetMap()
		for _, a := range append([]*common.Actor{}, m.Actors()...) {
			if a == player || a.D1Monster() == nil || !a.IsAlive() {
				continue
			}
			for _, spot := range m.GetPositionsByFilter(func(c *gridmap.MapCell[*common.Actor, *common.Item, common.Object]) bool {
				return c.TileType.IsWalkable
			}) {
				if geometry.DistanceChebyshev(spot, a.MapPosition()) != 6 || m.IsActorAt(spot) || !m.IsCurrentlyPassable(spot) {
					continue
				}
				m.MoveActor(player, spot)
				player.SetDrawPosition(spot.ToPointF())
				theGame.GetActionBroker().Execute(common.NewUpdateFoVAction(common.AbstractCause("test"), player))
				if player.CanSee(a.MapPosition()) {
					return a
				}
			}
		}
	}
	t.Skip("no monster six tiles away on this seed")
	return nil
}
