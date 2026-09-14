package main

import (
	"fmt"
	"os"
	"path/filepath"
	"rpg/common"
	"rpg/d1"
	"rpg/defs"
	"rpg/game"
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
	game.SaveFile = filepath.Join(os.TempDir(), "heavenAndHell_playthrough_save.json")
	defer os.Remove(game.SaveFile)
	theGame, _ := createGameAndUIContext() // one instance: ebiten allows a single audio context per process
	for run := 1; run <= 3; run++ {
		theGame.NewGame(theGame.GetGameData().ClassDefinitions[run-1], false)
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
			// milestone 4: open chests, sarcophagi and barrels, use shrines
			objects, used, dropped := operateObjects(state, player)
			if objects < 9 || used == 0 || dropped == 0 {
				fail("run %d dlvl %d: objects didn't work (objects=%d used=%d items dropped=%d)", run, dlvl, objects, used, dropped)
			}
			fmt.Printf("run %d dlvl %d: %d objects, %d used, %d items dropped\n", run, dlvl, objects, used, dropped)
			items := 0
			for _, it := range m.Items() {
				if it.D1() != nil {
					items++
				}
			}
			fmt.Printf("run %d (%s) dlvl %d: %d monsters, %d kills, %d damage taken, level %d, %d D1 items on floor\n",
				run, player.Name(), dlvl, monsters, kills, damageTaken, player.Hero().Level, items)

			// milestone 2: save, load, and everything that matters is back
			before := saveCheckOf(state)
			theGame.SaveGame()
			if err := theGame.LoadGame(); err != nil {
				fail("run %d dlvl %d: load failed: %v", run, dlvl, err)
			}
			state = theGame.GetWorldState()
			player = state.GetPlayer()
			if after := saveCheckOf(state); after != before {
				fail("run %d dlvl %d: save/load mismatch\n before %+v\n after  %+v", run, dlvl, before, after)
			}
		}
	}
	// milestone 3: the D1 town after a trip to level 4
	theGame.GetWorldState().MapTransition(gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: "spawn"})
	checkTown(theGame.GetWorldState(), fail)

	// milestone 2: a permadeath save is gone once the death menu is built
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], true)
	if !game.HasSave() {
		fail("permadeath: new game wrote no save")
	}
	theGame.StartMenuItems(true)
	if game.HasSave() {
		fail("permadeath: save survived death")
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
			reachable := geometry.DistanceChebyshev(a.MapPosition(), player.MapPosition()) <= 1 || len(m.GetFreeCardinalNeighbors(a.MapPosition())) > 0
			if a.D1Monster() != nil && a.IsAlive() && reachable && (target == nil ||
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

type saveCheck struct {
	Map                              string
	Pos                              geometry.Point
	Level, Exp, StatPts, Gold, HP    int
	Items, Equipped, Monsters, Floor int
	UsedObjects                      int
}

// operateObjects bumps into every chest, sarcophagus, plain barrel and shrine on the level.
func operateObjects(state *common.State, player *common.Actor) (objects, used, dropped int) {
	m := state.GetMap()
	before := len(m.Items())
	for _, obj := range append([]common.Object(nil), m.Objects()...) {
		o, ok := obj.(*common.D1Object)
		if !ok {
			continue
		}
		objects++
		switch o.O.Kind {
		case d1.ObjChest1, d1.ObjChest2, d1.ObjChest3, d1.ObjSarc, d1.ObjBarrel, d1.ObjShrineL, d1.ObjShrineR:
			o.ActorBumpedInto(player, nil)
			if !o.O.Active {
				used++
			}
		}
	}
	return objects, used, len(m.Items()) - before
}

func saveCheckOf(state *common.State) saveCheck {
	player := state.GetPlayer()
	m := state.GetMap()
	c := saveCheck{Map: m.GetName(), Pos: player.MapPosition(), Level: player.Hero().Level, Exp: player.Hero().Exp,
		StatPts: player.Hero().StatPts, Gold: player.GetStats().GetAttribute(stats.Gold),
		HP: player.GetStats().GetResource(stats.Health).Value(), Items: len(player.GetInventory().Items()),
		Equipped: len(player.GetEquipment().AllItems())}
	for _, a := range m.Actors() {
		if a.D1Monster() != nil && a.IsAlive() {
			c.Monsters++
		}
	}
	for _, it := range m.Items() {
		if it.D1() != nil {
			c.Floor++
		}
	}
	for _, obj := range m.Objects() {
		if o, ok := obj.(*common.D1Object); ok && !o.O.Active {
			c.UsedObjects++
		}
	}
	return c
}

// checkTown: D1 shopkeepers with priced D1 stock, gates by depth, broken gear and repairs.
func checkTown(state *common.State, fail func(string, ...any)) {
	m := state.GetMap()
	want := map[common.VendorType]string{common.VendorTypeWeapons: "Griswold", common.VendorTypeWands: "Adria",
		common.VendorTypeGeneral: "Pepin", common.VendorTypeCain: "Deckard Cain", common.VendorTypeWirt: "Wirt"}
	found := map[common.VendorType]*common.Actor{}
	for _, a := range m.Actors() {
		if name, ok := want[a.GetVendorType()]; ok && a.Name() == name {
			found[a.GetVendorType()] = a
		}
	}
	for vt, name := range want {
		if found[vt] == nil {
			fail("town: %s is missing", name)
		}
	}
	for _, vt := range []common.VendorType{common.VendorTypeWeapons, common.VendorTypeWands, common.VendorTypeGeneral, common.VendorTypeWirt} {
		inv := found[vt].GetInventory()
		if inv.IsEmpty() {
			fail("town: %s has no stock", want[vt])
		}
		for _, it := range inv.Items() {
			if it.D1() == nil || it.BuyPrice() <= 0 {
				fail("town: %s sells %s for %d", want[vt], it.Name(), it.BuyPrice())
			}
		}
	}
	if _, ok := m.GetTransitionAt(m.GetNamedLocation("to_cathedral")); !ok {
		fail("town: cathedral gate is closed")
	}
	if _, ok := m.GetTransitionAt(m.GetNamedLocation("to_catacombs")); ok {
		fail("town: catacombs gate is open before level 5 was reached")
	}

	player := state.GetPlayer()
	for _, it := range player.GetEquipment().AllItems() {
		if d := it.D1(); d != nil && d.Class == "ICLASS_WEAPON" && d.MaxDur > 0 && d.MaxDur < 255 {
			before := player.HeroSheet()
			d.Durability = 0
			if after := player.HeroSheet(); after.MaxDam == before.MaxDam && before.MaxDam > 1 {
				fail("town: broken %s still counts (max damage %d)", d.Name, after.MaxDam)
			}
			if !d1.CanRepair(d) || d1.RepairCost(d) <= 0 {
				fail("town: broken %s can't be repaired", d.Name)
			}
			d.Durability = d.MaxDur
		}
	}
}
