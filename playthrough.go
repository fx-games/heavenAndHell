package main

import (
	"fmt"
	"os"
	"rpg/common"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
)

// playthrough is the headless smoke test from D1_PLAN.md: it grows with every milestone and fails loudly.
func playthrough() {
	fail := func(format string, args ...any) {
		fmt.Printf("PLAYTHROUGH FAILED: "+format+"\n", args...)
		os.Exit(1)
	}
	theGame, _ := createGameAndUIContext() // one instance: ebiten allows a single audio context per process
	for run := 1; run <= 3; run++ {
		if run > 1 {
			theGame.ResetGame(theGame.GetGameData().ClassDefinitions[run-1])
		}
		state := theGame.GetWorldState()
		player := state.GetPlayer()
		if player.Hero() == nil {
			fail("run %d: player has no D1 hero", run)
		}
		for dlvl := 1; dlvl <= 4; dlvl++ {
			name := defs.LevelNameFromDepth(dlvl)
			state.MapTransition(gridmap.Transition{TargetMap: name, TargetLocation: "ladder_up"})
			m := state.GetMap()
			if m.GetName() != name {
				fail("run %d: expected map %s, got %s", run, name, m.GetName())
			}
			up, dn := m.GetNamedLocation("ladder_up"), m.GetNamedLocation("ladder_down")
			if _, ok := m.GetDijkstraMapWithActorsNotBlocking(up, 10000)[dn]; !ok {
				fail("run %d dlvl %d: stairs down at %v unreachable from %v", run, dlvl, dn, up)
			}
			monsters := 0
			for _, a := range m.Actors() {
				if a.D1Monster() != nil {
					monsters++
				}
			}
			if monsters < 10 {
				fail("run %d dlvl %d: only %d D1 monsters spawned", run, dlvl, monsters)
			}
			kills, damageTaken, exp := fight(theGame.GetActionBroker(), state, player)
			if kills == 0 || damageTaken == 0 || exp == 0 {
				fail("run %d dlvl %d: fight didn't work (kills=%d damage taken=%d exp=%d)", run, dlvl, kills, damageTaken, exp)
			}
			items := 0
			for _, it := range m.Items() {
				if it.D1() != nil {
					items++
				}
			}
			fmt.Printf("run %d (%s) dlvl %d: %d monsters, %d kills, %d damage taken, level %d, %d D1 items on floor\n",
				run, player.Name(), dlvl, monsters, kills, damageTaken, player.Hero().Level, items)
		}
	}
	fmt.Println("PLAYTHROUGH OK")
}

// fight first stands next to the nearest monster for a while (monster AI and damage), then attacks until 5 kills.
// The player is teleported between targets and healed after every turn.
func fight(broker *common.Broker, state *common.State, player *common.Actor) (kills, damageTaken, exp int) {
	m := state.GetMap()
	hp := player.GetStats().GetResource(stats.Health)
	expBefore := player.Hero().Exp
	nearest := func() *common.Actor {
		var target *common.Actor
		for _, a := range m.Actors() {
			if a.D1Monster() != nil && a.IsAlive() && (target == nil ||
				geometry.DistanceChebyshev(a.MapPosition(), player.MapPosition()) < geometry.DistanceChebyshev(target.MapPosition(), player.MapPosition())) {
				target = a
			}
		}
		return target
	}
	approach := func(target *common.Actor) bool {
		if geometry.DistanceChebyshev(target.MapPosition(), player.MapPosition()) <= 1 {
			return true
		}
		free := m.GetFreeCardinalNeighbors(target.MapPosition())
		if len(free) == 0 {
			return false
		}
		m.MoveActor(player, free[0])
		player.SetDrawPosition(free[0].ToPointF())
		broker.Execute(common.NewUpdateFoVAction(common.AbstractCause("playthrough"), player))
		return true
	}
	endTurn := func() {
		before := hp.Value()
		broker.Execute(common.NewEndPlayerTurnAction(common.AbstractCause("playthrough"), 10))
		damageTaken += max(before-hp.Value(), 0)
		hp.SetToMax()
	}

	for turn := 0; turn < 40; turn++ {
		if target := nearest(); target != nil && approach(target) {
			endTurn()
		}
	}
	if os.Getenv("PLAYTHROUGH_DEBUG") != "" {
		for _, a := range m.Actors() {
			if mon := a.D1Monster(); mon != nil && geometry.DistanceChebyshev(a.MapPosition(), player.MapPosition()) <= 3 {
				fmt.Printf("DEBUG %s at %v: squelch=%d energy=%d lastMode=%d stand=%d ai=%s canSee=%v\n", a.Name(), a.MapPosition(),
					mon.Squelch, a.GetEnergy(), mon.LastMode, mon.StandTicks, mon.AI, player.CanSee(a.MapPosition()))
			}
		}
	}

	for turn := 0; turn < 600 && kills < 5; turn++ {
		target := nearest()
		if target == nil {
			break
		}
		if !approach(target) {
			continue
		}
		broker.Execute(common.NewMeleeAttackAction(common.NewActorCause("playthrough", player), player, target))
		if !target.IsAlive() {
			kills++
		}
		endTurn()
	}
	return kills, damageTaken, player.Hero().Exp - expBefore
}
