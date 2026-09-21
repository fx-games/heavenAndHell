// The engine rolls a lot of dice on math/rand's global source (map transitions, melee, drops). Since Go 1.20
// that source seeds itself from the OS, and rand.Seed is a no-op, so a pinned playthrough seed could not reach
// it: this directive restores rand.Seed so `playthrough <seed>` really replays. A normal launch never calls
// Seed and keeps its automatic seeding.
//go:debug randseednop=0

package main

import (
	"fmt"
	"image/color"
	"math/rand"
	"os"
	"path/filepath"
	"rpg/common"
	"rpg/d1"
	"rpg/defs"
	"rpg/game"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
	"strconv"
	"strings"
)

// playthrough is the headless smoke test from D1_PLAN.md: it grows with every milestone and fails loudly.
func playthrough() {
	fail := func(format string, args ...any) {
		fmt.Printf("PLAYTHROUGH FAILED: "+format+"\n", args...)
		os.Exit(1)
	}
	game.SaveDir = filepath.Join(os.TempDir(), "heavenAndHell_playthrough_saves")
	os.RemoveAll(game.SaveDir)
	defer os.RemoveAll(game.SaveDir)
	theGame, _ := createGameAndUIContext() // one instance: ebiten allows a single audio context per process
	theGame.GetAudio().SetDisablePlayback(true)
	// A smoke test that plays a different dungeon every launch reports a different result every launch: this
	// used to pass about five runs in six, which is indistinguishable from "the change you just made is fine".
	// Both time-seeded sources get pinned, and the seed is printed so a failure can be replayed verbatim.
	seed := int64(1)
	if len(os.Args) > 2 {
		if parsed, err := strconv.ParseInt(os.Args[2], 10, 64); err == nil {
			seed = parsed
		}
	}
	fmt.Printf("playthrough seed: %d (replay with: playthrough %d)\n", seed, seed)
	theGame.SeedRandom(seed)
	common.SeedD1Rng(seed)
	rand.Seed(seed)                                                           // the global source the engine rolls most of its dice on
	missiles := &missileCounter{}                                             // registered per check: a new game or a load makes a new broker
	wallTraps, chestTraps := 0, 0                                             // traps that fired while objects were operated
	for run := 1; run <= len(theGame.GetGameData().ClassDefinitions); run++ { // every class
		os.RemoveAll(game.SaveDir) // every run starts in slot 1: a new game never writes over a slot that is taken
		theGame.NewGame(theGame.GetGameData().ClassDefinitions[run-1], false, "")
		state := theGame.GetWorldState()
		player := state.GetPlayer()
		if player.Hero() == nil {
			fail("run %d: player has no D1 hero", run)
		}
		if c, rolled := state.GetWaterColor(), state.QuestActive(d1.QPWater) != d1.QuestNotAvail; (c.G > c.B) != rolled {
			fail("run %d: town water poisoned=%v at game start, but Poisoned Water rolled=%v", run, c.G > c.B, rolled)
		}
		runDamage := 0 // milestone 3: over sixteen levels of fighting, something has to get through
		// the checks park the hero in packs: one turn of a whole pack must not kill a low-HP sorcerer on 14
		player.Hero().MaxHPBase += 1000 << 6
		common.SyncHero(player)
		player.GetStats().GetResource(stats.Health).SetToMax()
		for dlvl := 1; dlvl <= 16; dlvl++ {
			name, entry := defs.LevelNameFromDepth(dlvl), "ladder_up"
			state.MapTransition(gridmap.Transition{TargetMap: name, TargetLocation: entry})
			m := state.GetMap()
			if m.GetName() != name {
				fail("run %d: expected map %s, got %s", run, name, m.GetName())
			}
			// the lighting the renderer asks for on every cell (hero torch, braziers, wall torches) must not crash
			for y := 0; y < m.MapSize().Y; y++ {
				for x := 0; x < m.MapSize().X; x++ {
					state.ApplyLighting(geometry.Point{X: x, Y: y}, color.RGBA{R: 200, G: 200, B: 200, A: 255}, uint64(x+y))
				}
			}
			// milestone 6: stairs (and the town warp on 5, 9, 13) reachable on every level type
			up := m.GetNamedLocation(entry)
			reach := reachable(m, up)
			if dlvl == 4 && state.D1Quests()[d1.QLtBanner].Active != d1.QuestNotAvail { // Snotspill's gang holds the stairs down
				if reach[m.GetNamedLocation("ladder_down")] {
					fail("run %d dlvl 4: the stairs down are open before Ogden's Sign is done", run)
				}
				state.SetQuestVar(d1.QLtBanner, 1, 3)
				common.UpdateD1Quests(state, m)
				reach = reachable(m, up)
				if m.GetCell(m.GetNamedLocation("ladder_down")).TileType.Special != gridmap.SpecialTileStairsDown {
					fail("run %d dlvl 4: opening Snotspill's hall drew over the stairs down", run)
				}
			}
			if dlvl == 13 && state.D1Quests()[d1.QWarlord].Active != d1.QuestNotAvail { // the way down leads through his hall
				if reach[m.GetNamedLocation("ladder_down")] {
					fail("run %d dlvl 13: the Warlord's hall is open before the Steel Tome is read", run)
				}
				for _, obj := range m.Objects() {
					if o, ok := obj.(*common.D1Object); ok && o.O.Kind == d1.ObjSTEELTOME && reach[m.GetFreeCardinalNeighbors(o.MapPosition())[0]] {
						o.ActorBumpedInto(player, nil)
					}
				}
				reach = reachable(m, up)
			}
			for _, loc := range []string{"ladder_down", "warp"} {
				p := m.GetNamedLocation(loc)
				exists := m.GetNamedLocationByPos(p) == loc
				want := loc == "ladder_down" && dlvl != 16 && dlvl != 20 && dlvl != 24 ||
					loc == "warp" && (dlvl == 5 || dlvl == 9 || dlvl == 13 || dlvl == 17 || dlvl == 21)
				if exists != want {
					fail("run %d dlvl %d: %s exists=%v", run, dlvl, loc, exists)
				}
				if _, ok := reach[p]; exists && !ok {
					dumpMap(m, reach)
					fail("run %d dlvl %d: %s at %v unreachable from %v", run, dlvl, loc, p, up)
				}
			}
			// milestone 7: the quest bosses wait on their levels, when D1's own per-seed quest RNG (InitQuests'
			// QuestGroup1/3/4) kept their quest active - Butcher/LtBanner/Garbud, Mushroom/Zhar/Anvil and
			// Veil/Warlord are each mutually exclusive, so not every seed places all four bosses.
			type boss struct {
				name string
				q    int
			}
			if b, ok := map[int]boss{2: {"The Butcher", d1.QButcher}, 8: {"Zhar the Mad", d1.QZhar},
				13: {"Warlord of Blood", d1.QWarlord}, 16: {"The Dark Lord", d1.QDiablo}}[dlvl]; ok {
				active := state.D1Quests()[b.q].Active != d1.QuestNotAvail
				if active && actorNamed(m, b.name) == nil {
					fail("run %d dlvl %d: %s is missing", run, dlvl, b.name)
				}
			}
			if dlvl == 16 { // Diablo's quarters open lever by lever
				pullD1Levers(state)
			}
			boost(player, dlvl)                                // a level-1 hero healed every turn gets stun-locked forever by hit recovery
			player.RemoveStatusEffect(common.StatusManaShield) // shrines grant it, and it lasts across levels: the fight measures HP
			monsters := 0
			for _, a := range m.Actors() {
				if a.D1Monster() != nil {
					monsters++
				}
			}
			if monsters < 10 {
				fail("run %d dlvl %d: only %d D1 monsters spawned", run, dlvl, monsters)
			}
			switch dlvl {
			case 3:
				checkDoppelganger(theGame, fail)
			case 5:
				checkRune(theGame, fail)
			}
			kills, damageTaken, exp := fight(theGame.GetActionBroker(), state, player)
			if kills == 0 || exp == 0 {
				fail("run %d dlvl %d: fight didn't work (kills=%d exp=%d)", run, dlvl, kills, exp)
			}
			// a blocking Monk can walk a level untouched, so the hero has to bleed somewhere in the run, not here
			runDamage += damageTaken

			// milestone 4: open chests, sarcophagi and barrels, use shrines
			trapped := 0
			for _, obj := range m.Objects() {
				if o, ok := obj.(*common.D1Object); ok && o.O.TrapFlag && o.O.Kind >= d1.ObjTCHEST1 && o.O.Kind <= d1.ObjTCHEST3 {
					trapped++
				}
			}
			objects, used, dropped := operateObjects(state, player)
			for _, obj := range m.Objects() {
				if o, ok := obj.(*common.D1Object); ok && o.O.TrapFlag && o.O.Kind >= d1.ObjTCHEST1 && o.O.Kind <= d1.ObjTCHEST3 {
					fail("run %d dlvl %d: the trap of the opened %s at %v didn't fire", run, dlvl, o.Name(), o.MapPosition())
				}
			}
			chestTraps += trapped
			wallTraps += len(common.SnapshotLevel(state, m).Traps)
			if dlvl != 16 && (objects < 9 || used == 0 || dropped == 0) {
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

			if dlvl > 4 { // milestone 6: every monster that casts does so, and it hurts
				checkCasters(theGame.GetActionBroker(), state, player, missiles, fail)
			}

			if dlvl == 1 { // every class' skill (_pAblSpells) must fire without blowing up
				checkClassSkill(theGame.GetFactory(), player, fail)
				checkDualWield(player, fail)
			}

			if run == 3 && dlvl == 2 { // milestone 5: the sorcerer casts every spell
				castAllSpells(theGame.GetFactory(), theGame.GetActionBroker(), state, player, fail)
				castHellfireSpells(theGame.GetFactory(), theGame.GetActionBroker(), state, player, fail)
				player = state.GetPlayer()
			}

			// milestone 2: save, load, and everything that matters is back (a load sees from where the hero stands)
			theGame.GetActionBroker().Execute(common.NewUpdateFoVAction(common.AbstractCause("playthrough"), player))
			disarmedDoor := checkDoorDisarm(theGame.GetFactory(), state, player, fail)
			doors := 0
			for _, obj := range state.GetMap().Objects() { // a door opened before the save stays open
				if door, ok := obj.(*common.Door); ok && door.D1 != nil {
					doors++
					if door.IsClosed() {
						door.Open()
					}
				}
			}
			before := saveCheckOf(state)
			if doors > 0 && before.OpenDoors == 0 { // Hell has no doors, and a seed can lay out a level without one
				fail("run %d dlvl %d: %d doors on the level, none of them open after opening one", run, dlvl, doors)
			}
			theGame.SaveGame()
			if err := theGame.LoadGame(); err != nil {
				fail("run %d dlvl %d: load failed: %v", run, dlvl, err)
			}
			state = theGame.GetWorldState()
			player = state.GetPlayer()
			after := saveCheckOf(state)
			if after.Explored < before.Explored { // loading arrives by the stairs first, which may explore a few more cells
				fail("run %d dlvl %d: load forgot explored cells (%d -> %d)", run, dlvl, before.Explored, after.Explored)
			}
			if disarmedDoor >= 0 && !doorDisarmed(state, disarmedDoor) {
				fail("run %d dlvl %d: load forgot the disarmed door", run, dlvl)
			}
			after.Explored = before.Explored
			if after != before {
				fail("run %d dlvl %d: save/load mismatch\n before %+v\n after  %+v", run, dlvl, before, after)
			}
			for _, it := range state.GetMap().Items() { // nothing lies in a wall, in lava or on a fence
				if p := it.MapPosition(); !state.GetMap().IsTileWalkable(p) {
					fail("run %d dlvl %d: %s lies on %s at %v", run, dlvl, it.Name(), state.GetMap().GetCell(p).TileType.DefinedDescription, p)
				}
			}
		}
	}
	// milestone 3: the D1 town after a trip down to level 16
	theGame.GetWorldState().MapTransition(gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: "spawn"})
	checkTown(theGame.GetWorldState(), fail)
	if wallTraps == 0 || chestTraps == 0 { // Catacombs and Caves chests are trapped 1 in 10, trap walls 10-25%
		fail("traps: %d wall traps and %d chest traps fired in 3 runs", wallTraps, chestTraps)
	}
	fmt.Printf("traps: %d wall traps and %d chest traps fired\n", wallTraps, chestTraps)
	if len(missiles.byAI) < 6 { // level monster types are random; six of the eight casting AIs always showed up
		fail("missile AIs: only %v cast", missiles.byAI)
	}
	fmt.Printf("monster missiles by AI: %v\n", missiles.byAI)

	questPath(theGame, fail)
	questLines(theGame, fail)

	// milestone 2: a permadeath save is gone once the death menu is built
	os.RemoveAll(game.SaveDir)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], true, "")
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
			// Diablo waits sealed in; a meleeing sorcerer can't outdamage his regeneration; talking bosses don't fight yet
			if a.D1Monster() != nil && !a.D1Monster().Golem && a.D1Monster().Data.ID != "MT_DIABLO" && a.D1Monster().TalkMsg == "" && a.IsAlive() && reachable && (target == nil ||
				geometry.DistanceChebyshev(a.MapPosition(), player.MapPosition()) < geometry.DistanceChebyshev(target.MapPosition(), player.MapPosition())) {
				target = a
			}
		}
		return target
	}
	moveTo := func(p geometry.Point) {
		m.MoveActor(player, p)
		player.SetDrawPosition(p.ToPointF())
		broker.Execute(common.NewUpdateFoVAction(common.AbstractCause("playthrough"), player))
	}
	approach := func(target *common.Actor) bool {
		if geometry.DistanceChebyshev(target.MapPosition(), player.MapPosition()) <= 1 {
			return true
		}
		free := m.GetFreeCardinalNeighbors(target.MapPosition())
		if len(free) == 0 {
			return false
		}
		moveTo(free[0])
		return true
	}
	endTurn := func() {
		before := hp.Value()
		broker.Execute(common.NewEndPlayerTurnAction(common.AbstractCause("playthrough"), 10))
		damageTaken += max(before-hp.Value(), 0)
		hp.SetToMax()
	}

	for turn := 0; turn < 200 && (turn < 40 || damageTaken == 0); turn++ { // casters may fade away: stay until hit
		if target := nearest(); target != nil && approach(target) {
			endTurn()
			// a counselor's flash only reaches the cells beside and below it (MI_Flash): try another side now and then
			if free := m.GetFreeCardinalNeighbors(target.MapPosition()); turn%10 == 9 && len(free) > 0 && target.IsAlive() {
				moveTo(free[turn/10%len(free)])
			}
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
		// no turn passes between blows: a hero healed every turn can be stun-locked forever while monsters regenerate
		broker.Execute(common.NewMeleeAttackAction(common.NewActorCause("playthrough", player), player, target))
		if !target.IsAlive() {
			kills++
		}
	}
	return kills, damageTaken, player.Hero().Exp - expBefore
}

// checkDoorDisarm: Trap Disarm on a trapped door clears its trap, stops the wall traps aimed at it and opens it
// without setting them off (ACTION_DISARM: TryDisarm, then OperateDoor). Returns the door's object index, or -1.
func checkDoorDisarm(f *common.Factory, state *common.State, player *common.Actor, fail func(string, ...any)) int {
	m := state.GetMap()
	for _, obj := range m.Objects() {
		door, ok := obj.(*common.Door)
		if !ok || door.D1 == nil || !door.D1.TrapFlag || !door.IsClosed() {
			continue
		}
		free := m.GetFreeCardinalNeighbors(door.MapPosition())
		if len(free) == 0 {
			continue
		}
		dex := player.Hero().BaseDex
		player.Hero().BaseDex = 250 // 2·Dex − 5·dlvl ≥ 100: the roll can't fail
		common.SyncHero(player)
		m.MoveActor(player, free[0])
		traps := len(common.SnapshotLevel(state, m).Traps)
		f.D1Disarm(player, door.MapPosition())
		player.Hero().BaseDex = dex
		common.SyncHero(player)
		if door.D1.TrapFlag || !door.Disarmed || door.IsClosed() || len(common.SnapshotLevel(state, m).Traps) <= traps {
			fail("disarming a trapped door: trapped=%v disarmed=%v closed=%v", door.D1.TrapFlag, door.Disarmed, door.IsClosed())
		}
		return door.D1.Index
	}
	return -1
}

func doorDisarmed(state *common.State, index int) bool {
	for _, obj := range state.GetMap().Objects() {
		if door, ok := obj.(*common.Door); ok && door.D1 != nil && door.D1.Index == index {
			return door.Disarmed && !door.D1.TrapFlag
		}
	}
	return false
}

type saveCheck struct {
	Map                              string
	Pos                              geometry.Point
	Level, Exp, StatPts, Gold, HP    int
	Items, Equipped, Monsters, Floor int
	UsedObjects, Traps, OpenDoors    int
	Corpses, Explored, AIState       int
	Golem, ManaShield                bool
}

// operateObjects bumps into every chest, sarcophagus, plain barrel and shrine on the level; the traps on them fire.
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
		case d1.ObjCHEST1, d1.ObjCHEST2, d1.ObjCHEST3, d1.ObjTCHEST1, d1.ObjTCHEST2, d1.ObjTCHEST3, d1.ObjSARC, d1.ObjBARREL, d1.ObjSHRINEL, d1.ObjSHRINER:
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
		if mon := a.D1Monster(); mon != nil && a.IsAlive() {
			c.Monsters++
			if !mon.Golem {
				c.AIState += mon.Goal + mon.GoalVar1 + mon.Squelch + mon.Dir + mon.StandTicks
			}
		}
	}
	c.Golem, c.ManaShield = common.D1Golem(m) != nil, player.HasStatusEffect(common.StatusManaShield)
	c.Corpses = len(m.DownedActors())
	for y := 0; y < d1.LevelSize; y++ {
		for x := 0; x < d1.LevelSize; x++ {
			if m.IsExplored(geometry.Point{X: x, Y: y}) {
				c.Explored++
			}
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
	snap := common.SnapshotLevel(state, m)
	c.Traps, c.OpenDoors = len(snap.Traps), len(snap.Doors)
	return c
}

// checkDoppelganger swings a Doppelganger's weapon at an ordinary monster: one blow in ten splits off a copy.
func checkDoppelganger(theGame *game.GameInstance, fail func(string, ...any)) {
	state, broker := theGame.GetWorldState(), theGame.GetActionBroker()
	m, player := state.GetMap(), state.GetPlayer()
	var weapon *d1.Item
	for _, it := range player.GetEquipment().AllItems() {
		if d := it.D1(); d != nil && d.Class == "ICLASS_WEAPON" && d.Type != d1.ITypeBow {
			weapon = d
		}
	}
	if weapon == nil {
		return // the class fights with a bow or bare-handed
	}
	weapon.DamAcFlags |= d1.IHfDoppelganger
	defer func() { weapon.DamAcFlags &^= d1.IHfDoppelganger }()
	count := func(mt int) (n int) {
		for _, a := range m.Actors() {
			if mon := a.D1Monster(); mon != nil && mon.Type == mt {
				n++
			}
		}
		return
	}
	for _, a := range m.Actors() {
		mon := a.D1Monster()
		if mon == nil || mon.Golem || mon.Unique != nil || !a.IsAlive() || len(m.GetFreeCardinalNeighbors(a.MapPosition())) < 2 {
			continue
		}
		m.MoveActor(player, m.GetFreeCardinalNeighbors(a.MapPosition())[0])
		before := count(mon.Type)
		a.AddStatusEffect(common.StatusStone, 1000) // held in place, and every blow lands
		for swing := 0; swing < 200 && count(mon.Type) == before && a.IsAlive(); swing++ {
			a.GetStats().GetResource(stats.Health).SetToMax()
			broker.Execute(common.NewMeleeAttackAction(common.AbstractCause("playthrough"), player, a))
		}
		if count(mon.Type) == before {
			fail("doppelganger: 200 blows at %s split off no copy", a.Name())
		}
		a.RemoveStatusEffect(common.StatusStone)
		return
	}
}

// checkRune sets a Rune of Fire under the hero beside a monster: stepping on it (the hero triggers runes too)
// sets it off. Whether the blast hurts the monster is a to-hit roll, so only the detonation is checked.
func checkRune(theGame *game.GameInstance, fail func(string, ...any)) {
	state, f, broker := theGame.GetWorldState(), theGame.GetFactory(), theGame.GetActionBroker()
	m, player := state.GetMap(), state.GetPlayer()
	for _, a := range m.Actors() {
		mon := a.D1Monster()
		if mon == nil || mon.Golem || !a.IsAlive() || mon.Data.ID == "MT_DIABLO" {
			continue
		}
		free := m.GetFreeCardinalNeighbors(a.MapPosition())
		if len(free) == 0 {
			continue
		}
		m.MoveActor(player, free[0])
		rune := f.NewD1Item(f.D1().PlainItem(161)) // Rune of Fire
		player.GetInventory().Add(rune)
		rune.GetTargetedAction().Execute(player, player.MapPosition(), player.MapPosition())
		if player.GetInventory().Has(rune) || !m.IsObjectAt(player.MapPosition()) {
			fail("rune: the Rune of Fire wasn't set under the hero")
			return
		}
		broker.Execute(common.NewEndPlayerTurnAction(common.AbstractCause("playthrough"), 10))
		if m.IsObjectAt(player.MapPosition()) {
			fail("rune: the hero stands on the Rune of Fire and it doesn't go off")
		}
		return
	}
}

// checkTown: D1 shopkeepers with priced D1 stock, gates by depth, broken gear and repairs.
func checkTown(state *common.State, fail func(string, ...any)) {
	m := state.GetMap()
	want := map[common.VendorType]string{common.VendorTypeWeapons: "Griswold", common.VendorTypeWands: "Adria",
		common.VendorTypeGeneral: "Pepin", common.VendorTypeCain: "Deckard Cain", common.VendorTypeWirt: "Wirt",
		common.VendorTypeDrunk: "Farnham", common.VendorTypeBarmaid: "Gillian"}
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
	for dlvl, gate := range common.D1TownGates { // all open after level 16, each leading to its level's warp
		if t, ok := m.GetTransitionAt(m.GetNamedLocation(gate)); !ok || t.TargetMap != defs.LevelNameFromDepth(dlvl) {
			fail("town: %s gate leads to %+v", gate, t)
		}
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

// castAllSpells teaches the hero all 26 spells and casts each at the nearest monster; then it walks through the
// town portal and back, which closes it.
// castHellfireSpells casts Hellfire's spells from memory: Mana and the Magi fill mana, Reflect stores blows,
// Berserk turns a monster, the walls and rings leave burning cells, Immolation hurts, Warp lands by the stairs and
// the Jester casts something.
func castHellfireSpells(f *common.Factory, broker *common.Broker, state *common.State, player *common.Actor, fail func(string, ...any)) {
	h, m := player.Hero(), state.GetMap()
	mana := player.GetStats().GetResource(stats.Mana)
	for sp := d1.SplMana; sp <= d1.SplSearch; sp++ {
		h.SpellLevels[sp] = 3
	}
	f.RefreshD1Spells(player)
	nearest := func() *common.Actor {
		var target *common.Actor
		for _, a := range m.Actors() {
			if mon := a.D1Monster(); mon != nil && !mon.Golem && !mon.Berserk && a.IsAlive() && len(m.GetFreeCardinalNeighbors(a.MapPosition())) > 0 && (target == nil ||
				geometry.DistanceChebyshev(a.MapPosition(), player.MapPosition()) < geometry.DistanceChebyshev(target.MapPosition(), player.MapPosition())) {
				target = a
			}
		}
		return target
	}
	cast := func(sp int, target geometry.Point) {
		mana.SetToMax()
		f.CastD1FromMemory(player, sp, target)
	}
	burning := func() (n int) {
		for _, o := range m.Objects() {
			if _, ok := o.(*common.DamageDealingObject); ok {
				n++
			}
		}
		return
	}

	mana.SetToMax()
	mana.Decrease(mana.Value() / 2)
	low := mana.Value()
	f.CastD1FromMemory(player, d1.SplMagi, player.MapPosition())
	if h.ManaBase != h.MaxManaBase-f.D1().D.ManaCost(h, player.HeroSheet(), d1.SplMagi) {
		fail("hellfire spells: the Magi left mana base %d of %d (mana %d before)", h.ManaBase, h.MaxManaBase, low)
	}
	stored := h.Reflections
	cast(d1.SplReflect, player.MapPosition())
	if h.Reflections <= stored || h.Reflections > stored+3*h.Level { // monsters may spend some in the turn the cast takes
		fail("hellfire spells: Reflect went from %d to %d blows, adding up to %d", stored, h.Reflections, 3*h.Level)
	}
	target := nearest()
	if target == nil {
		fail("hellfire spells: no monster to cast at")
		return
	}
	if free := m.GetFreeCardinalNeighbors(target.MapPosition()); geometry.DistanceChebyshev(target.MapPosition(), player.MapPosition()) > 1 {
		m.MoveActor(player, free[0])
		broker.Execute(common.NewUpdateFoVAction(common.AbstractCause("playthrough"), player))
	}
	for tries := 0; tries < 20 && !target.D1Monster().Berserk && target.IsAlive(); tries++ { // resistant monsters shrug it off half the time
		cast(d1.SplBerserk, target.MapPosition())
	}
	if target.IsAlive() && target.D1Monster().MagicRes&d1.ImmuneMagic == 0 && target.D1Monster().Unique == nil && !target.D1Monster().Berserk {
		fail("hellfire spells: Berserk didn't take %s", target.Name())
	}
	before := burning()
	cast(d1.SplRingOfFire, player.MapPosition())
	if burning() == before {
		fail("hellfire spells: Ring of Fire lit nothing")
	}
	if target = nearest(); target != nil {
		before = burning()
		cast(d1.SplLightWall, target.MapPosition())
		if burning() == before {
			fail("hellfire spells: Lightning Wall raised nothing")
		}
		cast(d1.SplImmolation, player.MapPosition())
		cast(d1.SplJester, target.MapPosition())
	}
	cast(d1.SplMana, player.MapPosition())
	lvl := state.D1Level(m)
	cast(d1.SplWarp, player.MapPosition())
	near := false
	for _, t := range lvl.Triggers {
		near = near || geometry.DistanceChebyshev(player.MapPosition(), geometry.Point{X: t.X - d1.CellOrigin, Y: t.Y - d1.CellOrigin}) <= 6
	}
	if !near {
		fail("hellfire spells: Warp left the hero at %v, away from the stairs", player.MapPosition())
	}
}

func castAllSpells(f *common.Factory, broker *common.Broker, state *common.State, player *common.Actor, fail func(string, ...any)) {
	spells := []int{d1.SplFirebolt, d1.SplHeal, d1.SplLightning, d1.SplFlash, d1.SplIdentify, d1.SplFireWall, d1.SplStone,
		d1.SplInfra, d1.SplPhasing, d1.SplManaShield, d1.SplFireball, d1.SplGuardian, d1.SplChain, d1.SplWave, d1.SplNova,
		d1.SplInferno, d1.SplGolem, d1.SplTeleport, d1.SplApocalypse, d1.SplElemental, d1.SplChargedBolt, d1.SplHolyBolt,
		d1.SplTelekinesis, d1.SplBloodStar, d1.SplBoneSpirit, d1.SplTown}
	h := player.Hero()
	for _, sp := range spells {
		h.SpellLevels[sp] = 5
	}
	h.MaxManaBase += 5000 << 6
	common.SyncHero(player)
	f.RefreshD1Spells(player)
	m := state.GetMap()
	mana, hp := player.GetStats().GetResource(stats.Mana), player.GetStats().GetResource(stats.Health)
	monsterHP := func() (sum int) {
		for _, a := range m.Actors() {
			if mon := a.D1Monster(); mon != nil && !mon.Golem && a.IsAlive() {
				sum += a.GetStats().GetResource(stats.Health).Value()
			}
		}
		return
	}
	if weapons := player.GetEquipment().AllItems(); len(weapons) > 0 && weapons[0].D1() != nil { // the dummy UI's item list never picks: a cancel
		d := *weapons[0].D1()
		d.Magical, d.Identified = 1, false
		unidentified := f.NewD1Item(&d)
		player.GetInventory().Add(unidentified)
		mana.SetToMax()
		full := mana.Value()
		f.CastD1FromMemory(player, d1.SplIdentify, player.MapPosition())
		if mana.Value() != full || unidentified.D1().Identified {
			fail("spells: cancelling Identify's item list took mana (%d/%d) or identified the item", mana.Value(), full)
		}
		// picking an item must identify it and open its description window
		realUI := f.UI
		picker := &identifyPickUI{UserInterface: realUI()}
		f.UI = func() common.UserInterface { return picker }
		mana.SetToMax()
		f.CastD1FromMemory(player, d1.SplIdentify, player.MapPosition())
		f.UI = realUI
		if !unidentified.D1().Identified || picker.shown != unidentified {
			fail("spells: Identify left the item unidentified (%v) or showed no description window (%v)", unidentified.D1().Identified, picker.shown)
		}
		player.GetInventory().Remove(unidentified)
	}
	dealt := 0
	var failed []string
	for _, sp := range spells {
		var target *common.Actor
		for _, a := range m.Actors() {
			if mon := a.D1Monster(); mon != nil && !mon.Golem && a.IsAlive() && len(m.GetFreeCardinalNeighbors(a.MapPosition())) > 0 && (target == nil ||
				geometry.DistanceChebyshev(a.MapPosition(), player.MapPosition()) < geometry.DistanceChebyshev(target.MapPosition(), player.MapPosition())) {
				target = a
			}
		}
		if target == nil {
			fail("spells: no monster left to cast at")
		}
		if sp == spells[0] {
			forecast, ok := common.D1ForecastSection(state, player, target)
			if !ok || len(forecast.Rows) != 2 || !strings.Contains(forecast.Rows[0][1], "to kill") && !strings.Contains(forecast.Rows[0][1], "can't") {
				fail("forecast against %s: %v", target.Name(), forecast)
			}
			fmt.Printf("forecast against %s: %v\n", target.Name(), forecast.Rows)
		}
		if geometry.DistanceChebyshev(target.MapPosition(), player.MapPosition()) > 1 {
			free := m.GetFreeCardinalNeighbors(target.MapPosition())[0]
			m.MoveActor(player, free)
			player.SetDrawPosition(free.ToPointF())
			broker.Execute(common.NewUpdateFoVAction(common.AbstractCause("playthrough"), player))
		}
		mana.SetToMax()
		hp.SetToMax()
		before, manaBefore := monsterHP(), mana.Value()
		f.CastD1FromMemory(player, sp, target.MapPosition())
		if mana.Value() == manaBefore {
			failed = append(failed, state.GetMap().GetName()+": "+fmt.Sprint(sp))
		}
		dealt += max(before-monsterHP(), 0)
		if os.Getenv("PLAYTHROUGH_DEBUG") != "" {
			fmt.Printf("DEBUG spell %d at %s %v res=%x (dist %d): %d damage, mana %d->%d\n", sp, target.Name(), target.MapPosition(), target.D1Monster().MagicRes,
				geometry.DistanceChebyshev(target.MapPosition(), player.MapPosition()), before-monsterHP(), manaBefore, mana.Value())
		}
	}
	for turn := 0; turn < 10; turn++ { // fire walls and the guardian burn on their own turns
		broker.Execute(common.NewEndPlayerTurnAction(common.AbstractCause("playthrough"), 10))
		hp.SetToMax()
	}
	golem := false
	for _, a := range m.Actors() {
		golem = golem || a.D1Monster() != nil && a.D1Monster().Golem
	}
	// Identify has nothing to identify and Telekinesis may find no room to push; everything else must cast.
	if len(failed) > 2 || dealt == 0 || !golem || state.D1Portal() == nil {
		fail("spells: failed casts %v, damage %d, golem %v, portal %v", failed, dealt, golem, state.D1Portal())
	}
	shield := player.HasStatusEffect(common.StatusManaShield)
	portal := *state.D1Portal()
	state.MapTransition(gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: "spawn"})
	if !state.GetMap().IsObjectAt(state.GetMap().GetNamedLocation("portal")) {
		fail("spells: no town portal in town")
	}
	state.MapTransition(gridmap.Transition{TargetMap: portal.Map, TargetLocation: "portal"})
	if state.D1Portal() != nil || state.GetMap().GetName() != portal.Map {
		fail("spells: town portal round trip didn't close the portal (%v on %s)", state.D1Portal(), state.GetMap().GetName())
	}
	if common.D1Golem(state.GetMap()) == nil || shield && !player.HasStatusEffect(common.StatusManaShield) {
		fail("spells: after the town portal round trip: golem %v, mana shield %v (had %v)", common.D1Golem(state.GetMap()), player.HasStatusEffect(common.StatusManaShield), shield)
	}
	fmt.Printf("spells: %d damage, failed casts %v, mana shield %v\n", dealt, failed, shield)
	// the golem and the mana shield now outlast level changes: dismiss them so later fights measure the hero's damage
	if golem := common.D1Golem(state.GetMap()); golem != nil {
		state.GetMap().RemoveActor(golem)
	}
	player.RemoveStatusEffect(common.StatusManaShield)
}

// boost stands in for a hero who fought their way down: character level 2×dlvl with the points in Str/Dex/Vit.
func boost(player *common.Actor, dlvl int) {
	h := player.Hero()
	if want := min(2*dlvl, 50); h.Level < want {
		h.AddExperience(h.Level, d1.ExpLvls[want-2]-h.Exp)
	}
	for ; h.StatPts > 0; h.StatPts-- {
		h.AddBaseStat([]int{d1.AttrStr, d1.AttrDex, d1.AttrVit}[h.StatPts%3], 1)
	}
	common.SyncHero(player)
	player.GetStats().GetResource(stats.Health).SetToMax()
}

// missileCounter counts monster missiles by the AI that cast them.
type missileCounter struct{ byAI map[string]int }

func (c *missileCounter) ActionName() common.ActionName { return common.ActionNameMonsterMissile }
func (c *missileCounter) IsBefore() bool                { return true }
func (c *missileCounter) IsAfter() bool                 { return false }
func (c *missileCounter) After(common.Action, common.ActionContext) []common.Action {
	return nil
}
func (c *missileCounter) Before(action common.Action, _ common.ActionContext) []common.Action {
	if c.byAI == nil {
		c.byAI = map[string]int{}
	}
	c.byAI[action.Cause().GetActor().D1Monster().AI]++
	return nil
}

// checkCasters puts one monster of every AI type on the level four cells from the player and lets it act for a while;
// the casters' missiles are counted.
func checkCasters(broker *common.Broker, state *common.State, player *common.Actor, missiles *missileCounter, fail func(string, ...any)) {
	broker.RegisterHook(missiles)
	defer broker.RemoveBeforeHook(missiles)
	m := state.GetMap()
	seen := map[string]bool{}
	for _, a := range append([]*common.Actor(nil), m.Actors()...) {
		mon := a.D1Monster()
		if mon == nil || !a.IsAlive() || seen[mon.AI] {
			continue
		}
		seen[mon.AI] = true
		before := 0
		for _, n := range missiles.byAI {
			before += n
		}
		hp := player.GetStats().GetResource(stats.Health)
		taken := 0
		for turn := 0; turn < 40; turn++ {
			if !a.IsAlive() {
				break
			}
			// stand four cells away in a line the monster can see
			if geometry.DistanceChebyshev(a.MapPosition(), player.MapPosition()) != 4 {
				for _, d := range common.D1Dirs {
					p := a.MapPosition().Add(geometry.Point{X: 4 * d.X, Y: 4 * d.Y})
					if m.Contains(p) && m.CanPlaceActorHere(p) && clearLine(m, a.MapPosition(), p) {
						m.MoveActor(player, p)
						player.SetDrawPosition(p.ToPointF())
						broker.Execute(common.NewUpdateFoVAction(common.AbstractCause("playthrough"), player))
						break
					}
				}
			}
			v := hp.Value()
			broker.Execute(common.NewEndPlayerTurnAction(common.AbstractCause("playthrough"), 10))
			taken += max(v-hp.Value(), 0)
			hp.SetToMax()
		}
		after := 0
		for _, n := range missiles.byAI {
			after += n
		}
		if os.Getenv("PLAYTHROUGH_DEBUG") != "" {
			fmt.Printf("DEBUG caster %s (%s) at %v: %d missiles, %d damage\n", a.Name(), mon.AI, a.MapPosition(), after-before, taken)
		}
	}
}

func clearLine(m *gridmap.GridMap[*common.Actor, *common.Item, common.Object], from, to geometry.Point) bool {
	los := geometry.LineOfSight(from, to, func(p geometry.Point) bool { return p == from || p == to || m.IsTransparent(p) })
	return los[len(los)-1] == to
}

func dumpMap(m *gridmap.GridMap[*common.Actor, *common.Item, common.Object], reach map[geometry.Point]bool) {
	for y := 0; y < 80; y++ {
		for x := 0; x < 80; x++ {
			p := geometry.Point{X: x, Y: y}
			_, r := reach[p]
			switch {
			case m.GetNamedLocationByPos(p) != "":
				fmt.Print("S")
			case m.IsActorAt(p):
				fmt.Print("a")
			case m.IsObjectAt(p) && !m.IsWalkable(p):
				fmt.Print("O")
			case m.IsObjectAt(p):
				fmt.Print("o")
			case !m.IsTileWalkable(p):
				fmt.Print("#")
			case r:
				fmt.Print(",")
			default:
				fmt.Print(".")
			}
		}
		fmt.Println()
	}
}

// reachable walks from p through floor, doors and barrels (bumping breaks them).
func objectAt(m *gridmap.GridMap[*common.Actor, *common.Item, common.Object], p geometry.Point) int {
	if m.IsObjectAt(p) {
		if o, ok := m.ObjectAt(p).(*common.D1Object); ok {
			return o.O.Kind
		}
	}
	return -1
}

func reachable(m *gridmap.GridMap[*common.Actor, *common.Item, common.Object], p geometry.Point) map[geometry.Point]bool {
	seen := map[geometry.Point]bool{p: true}
	for queue := []geometry.Point{p}; len(queue) > 0; queue = queue[1:] {
		for _, d := range common.D1Dirs {
			q := queue[0].Add(d)
			if seen[q] || !m.Contains(q) || !m.IsTileWalkable(q) {
				continue
			}
			if m.IsObjectAt(q) {
				o := m.ObjectAt(q)
				d1o, isD1 := o.(*common.D1Object)
				_, isDoor := o.(*common.Door)
				if !o.IsWalkable(nil) && !isDoor && !(isD1 && (d1o.O.Kind == d1.ObjBARREL || d1o.O.Kind == d1.ObjBARRELEX)) {
					continue
				}
			}
			seen[q] = true
			queue = append(queue, q)
		}
	}
	return seen
}

func actorNamed(m *gridmap.GridMap[*common.Actor, *common.Item, common.Object], name string) *common.Actor {
	for _, a := range m.Actors() {
		if a.Name() == name && a.IsAlive() {
			return a
		}
	}
	return nil
}

// questPath walks decision 15's critical path with a fresh hero: Leoric's tomb, the staff, Cain, the red portal,
// Lazarus, a save and load, the stairs to 16, the levers and Diablo. Bosses are struck down directly.
func questPath(theGame *game.GameInstance, fail func(string, ...any)) {
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "")
	state := theGame.GetWorldState()

	// D1 has no natural regeneration: waiting doesn't heal a hurt hero
	hp, mana := state.GetPlayer().GetStats().GetResource(stats.Health), state.GetPlayer().GetStats().GetResource(stats.Mana)
	common.SetResource(hp, hp.GetMax()/2)
	common.SetResource(mana, mana.GetMax()/2)
	for turn := 0; turn < 50; turn++ {
		theGame.GetActionBroker().Execute(common.NewEndPlayerTurnAction(common.AbstractCause("playthrough"), 10))
	}
	if hp.Value() != hp.GetMax()/2 || mana.Value() != mana.GetMax()/2 {
		fail("regeneration: HP %d/%d, mana %d/%d after 50 turns from half", hp.Value(), hp.GetMax(), mana.Value(), mana.GetMax())
	}

	travel := func(name, loc string) *gridmap.GridMap[*common.Actor, *common.Item, common.Object] {
		state.MapTransition(gridmap.Transition{TargetMap: name, TargetLocation: loc})
		return state.GetMap()
	}
	through := func(loc string) {
		m := state.GetMap()
		p := m.GetNamedLocation(loc)
		t, ok := m.GetTransitionAt(p)
		if !ok || m.GetNamedLocationByPos(p) != loc {
			fail("quests: no way through %s on %s", loc, m.GetName())
		}
		if reach := reachable(m, m.GetNamedLocation("ladder_up")); !reach[p] {
			dumpMap(m, reach)
			fail("quests: %s on %s unreachable", loc, m.GetName())
		}
		state.MapTransition(t)
	}
	slay := func(name string, q int) {
		a := actorNamed(state.GetMap(), name)
		if a == nil {
			fail("quests: %s not found on %s", name, state.GetMap().GetName())
		}
		hp := a.GetStats().GetResource(stats.Health).GetMax()
		theGame.GetActionBroker().Execute(common.NewChangeResourceAction(common.NewActorCause("playthrough", state.GetPlayer()), a, stats.Health, -float64(10*hp)))
		if state.QuestActive(q) != d1.QuestDone {
			fail("quests: killing %s didn't complete %s", name, state.QuestDefs()[q].Name)
		}
	}
	objectsOf := func(kind int) (out []*common.D1Object) {
		for _, obj := range state.GetMap().Objects() {
			if o, ok := obj.(*common.D1Object); ok && o.O.Kind == kind {
				out = append(out, o)
			}
		}
		return
	}

	travel(defs.LevelNameFromDepth(3), "ladder_up")
	// D1 monsters heal over time, unless a no-heal weapon hit them
	var healer, noHeal *common.Actor
	for _, a := range state.GetMap().Actors() {
		// single player halves monster HP down to 1: such a monster is already at max and has nothing to heal
		if mon := a.D1Monster(); mon != nil && mon.Unique == nil && a.GetStats().GetResource(stats.Health).GetMax() > 1 &&
			geometry.DistanceChebyshev(a.MapPosition(), state.GetPlayer().MapPosition()) > 15 {
			if healer == nil {
				healer = a
			} else if noHeal == nil {
				noHeal = a
			}
		}
	}
	noHeal.D1Monster().NoHeal = true
	var stoned []*common.Actor // stone monsters still heal but can't kill the level-1 hero while turns pass
	for _, a := range state.GetMap().Actors() {
		if a.D1Monster() != nil && a.IsAlive() {
			a.AddStatusEffect(common.StatusStone, 100)
			stoned = append(stoned, a)
		}
	}
	for _, a := range []*common.Actor{healer, noHeal} {
		common.SetResource(a.GetStats().GetResource(stats.Health), 1)
	}
	for turn := 0; turn < 50; turn++ {
		theGame.GetActionBroker().Execute(common.NewEndPlayerTurnAction(common.AbstractCause("playthrough"), 10))
		state.GetPlayer().GetStats().GetResource(stats.Health).SetToMax()
	}
	if h, n := healer.GetStats().GetResource(stats.Health).Value(), noHeal.GetStats().GetResource(stats.Health).Value(); h <= 1 || n != 1 {
		fail("monster regeneration: healer at %d HP, no-heal at %d HP after 50 turns from 1", h, n)
	}
	// D1 hit recovery: a hard hit costs the hero time and a monster energy; the lost time passes with the next turn
	player, broker := state.GetPlayer(), theGame.GetActionBroker()
	broker.Execute(common.NewChangeResourceAction(common.NewActorCause("playthrough", healer), player, stats.Health, -float64(player.Hero().Level)))
	if player.HitRecovery() == 0 {
		fail("hit recovery: a hit of %d didn't stagger the level %d hero", player.Hero().Level, player.Hero().Level)
	}
	broker.Execute(common.NewEndPlayerTurnAction(common.AbstractCause("playthrough"), 10))
	player.GetStats().GetResource(stats.Health).SetToMax()
	if player.HitRecovery() != 0 {
		fail("hit recovery: %d AUT still pending after the turn", player.HitRecovery())
	}
	for _, a := range stoned {
		a.RemoveStatusEffect(common.StatusStone)
	}
	checkItemEffects(broker, state, fail)
	var barrel *common.D1Object // anything that hits a barrel breaks it: here an arrow shot at its cell
	for _, obj := range state.GetMap().Objects() {
		if o, ok := obj.(*common.D1Object); ok && o.O.Active && (o.O.Kind == d1.ObjBARREL || o.O.Kind == d1.ObjBARRELEX) && geometry.DistanceChebyshev(o.MapPosition(), player.MapPosition()) > 3 {
			barrel = o
			break
		}
	}
	if barrel == nil {
		fail("barrels: none on %s to shoot at", state.GetMap().GetName())
	}
	broker.Execute(common.NewRangedAttackAction(common.NewActorCause("playthrough", player), player, barrel.MapPosition()))
	if barrel.O.Active {
		fail("barrels: an arrow didn't break the barrel at %v", barrel.MapPosition())
	}
	for _, a := range state.GetMap().Actors() {
		if mon := a.D1Monster(); mon != nil && a.IsAlive() && common.D1MonsterRecoveryAUT(mon) > 0 {
			a.GetStats().SetAttribute(stats.MaxHealth, 1000)
			a.GetStats().GetResource(stats.Health).SetToMax()
			broker.Execute(common.NewChangeResourceAction(common.NewActorCause("playthrough", player), a, stats.Health, -float64(mon.Level+3)))
			if lost := common.D1MonsterRecoveryAUT(mon); a.GetEnergy() > -lost {
				fail("hit recovery: %s has %d energy after a hard hit (should lose %d AUT)", a.Name(), a.GetEnergy(), lost)
			}
			break
		}
	}
	if state.D1Quests()[d1.QSkelKing].Active != d1.QuestNotAvail { // InitQuests drops Leoric or Poisoned Water per seed
		through("leoric")
		slay("Skeleton King", d1.QSkelKing)
		through("ladder_up")
		if state.GetMap().GetName() != defs.LevelNameFromDepth(3) {
			fail("quests: Leoric's tomb leads to %s", state.GetMap().GetName())
		}
	}

	m := travel(defs.LevelNameFromDepth(15), "ladder_up")
	if _, open := m.GetTransitionAt(m.GetNamedLocation("ladder_down")); open {
		fail("quests: level 15's stairs down are open before Lazarus died")
	}
	stands := objectsOf(d1.ObjLAZSTAND)
	if len(stands) != 1 || !reachable(m, m.GetNamedLocation("ladder_up"))[stands[0].MapPosition()] && len(m.GetFreeCardinalNeighbors(stands[0].MapPosition())) == 0 {
		fail("quests: %d staff stands on level 15", len(stands))
	}
	stands[0].ActorBumpedInto(state.GetPlayer(), nil)
	var staff *common.Item
	for _, it := range m.Items() {
		if it.D1() != nil && it.D1().Name == "Staff of Lazarus" {
			staff = it
		}
	}
	if staff == nil {
		fail("quests: the stand dropped no staff")
	}
	m.RemoveItem(staff)
	state.GetPlayer().GetInventory().Add(staff)
	travel(string(defs.LevelNameTown), "spawn")
	talkTo(theGame, common.VendorTypeCain)
	if state.QuestVar(d1.QBetrayer, 1) != 2 || d1Carried(state.GetPlayer(), d1.IdiLazStaff) {
		fail("quests: Cain didn't take the staff")
	}
	travel(defs.LevelNameFromDepth(15), "ladder_up")
	through("red_portal")
	m = state.GetMap()
	if _, open := m.GetTransitionAt(m.GetNamedLocation("exit")); open {
		fail("quests: the lair's portal is open before Lazarus died")
	}
	// Lazarus' lair: both Books of Vileness, read from their circles, then the central circle leads to him
	player = state.GetPlayer()
	player.Hero().MaxHPBase += 1000 << 6 // his lair is full of monsters for a level-1 warrior
	common.SyncHero(player)
	if actorNamed(m, "Arch-Bishop Lazarus") != nil {
		fail("quests: Lazarus isn't walled in before the Books of Vileness are read")
	}
	var center *common.D1Object
	for _, c := range append(objectsOf(d1.ObjMCIRCLE1), objectsOf(d1.ObjMCIRCLE2)...) {
		if c.MapPosition() == (geometry.Point{X: 35 - 16, Y: 36 - 16}) {
			center = c
		}
	}
	books := objectsOf(d1.ObjBOOK2L)
	if len(books) != 2 || center == nil {
		fail("quests: the lair has %d Books of Vileness, central circle %v", len(books), center != nil)
	}
	for _, book := range books {
		start := player.MapPosition()
		book.ActorBumpedInto(player, nil) // not on its circle: nothing happens
		if !book.O.Active || player.MapPosition() != start {
			fail("quests: a Book of Vileness worked away from its circle")
		}
		circle := book.MapPosition().Add(geometry.Point{Y: 1})
		if d1.ObjMCIRCLE2 != objectAt(m, circle) {
			fail("quests: no magic circle in front of the book at %v", book.MapPosition())
		}
		if !reachable(m, m.GetNamedLocation("ladder_up"))[circle] {
			fail("quests: the magic circle at %v can't be walked to from the lair's entrance", circle)
		}
		m.MoveActor(player, circle)
		book.ActorBumpedInto(player, nil)
		theGame.GetActionBroker().Execute(common.NewEndPlayerTurnAction(common.AbstractCause("playthrough"), 10))
		if book.O.Active || geometry.DistanceChebyshev(player.MapPosition(), circle) < 10 {
			fail("quests: the Book of Vileness at %v didn't teleport the hero (now at %v)", book.MapPosition(), player.MapPosition())
		}
	}
	m.MoveActor(player, center.MapPosition())
	center.ActorWalkedOn(player, nil)
	if state.QuestVar(d1.QBetrayer, 1) != 4 || player.MapPosition() != (geometry.Point{X: 35 - 16, Y: 46 - 16}) {
		fail("quests: the central circle didn't lead to Lazarus (qvar1 %d, hero at %v)", state.QuestVar(d1.QBetrayer, 1), player.MapPosition())
	}
	lazarus := actorNamed(m, "Arch-Bishop Lazarus")
	if lazarus == nil {
		fail("quests: the central circle didn't open Lazarus' room")
	}
	for turn := 0; turn < 5 && state.QuestVar(d1.QBetrayer, 1) < 6; turn++ {
		theGame.GetActionBroker().Execute(common.NewUpdateFoVAction(common.AbstractCause("playthrough"), player))
		theGame.GetActionBroker().Execute(common.NewEndPlayerTurnAction(common.AbstractCause("playthrough"), 10))
		player.GetStats().GetResource(stats.Health).SetToMax()
	}
	if state.QuestVar(d1.QBetrayer, 1) != 6 {
		dumpMap(m, reachable(m, player.MapPosition()))
		fail("quests: Lazarus didn't speak (qvar1 %d, Lazarus at %v seen %v goal %d talk %q, hero at %v)", state.QuestVar(d1.QBetrayer, 1),
			lazarus.MapPosition(), player.CanSee(lazarus.MapPosition()), lazarus.D1Monster().Goal, lazarus.D1Monster().TalkMsg, player.MapPosition())
	}
	for _, name := range []string{"Red Vex", "BlackJade"} {
		if actorNamed(m, name) == nil {
			fail("quests: %s is missing from %s after Lazarus spoke", name, m.GetName())
		}
	}
	slay("Arch-Bishop Lazarus", d1.QBetrayer)

	theGame.SaveGame()
	if err := theGame.LoadGame(); err != nil {
		fail("quests: load failed: %v", err)
	}
	state = theGame.GetWorldState()
	through("exit")
	through("ladder_down")
	m = state.GetMap()
	diablo := actorNamed(m, "The Dark Lord")
	if diablo == nil {
		fail("quests: Diablo is missing")
	}
	nextToDiablo := func() bool {
		reach := reachable(m, m.GetNamedLocation("ladder_up"))
		for _, d := range common.D1Dirs {
			if reach[diablo.MapPosition().Add(d)] {
				return true
			}
		}
		return false
	}
	if nextToDiablo() {
		fail("quests: Diablo is reachable before the levers are pulled")
	}
	levers := objectsOf(d1.ObjSWITCHSKL)
	if pulled := pullD1Levers(state); pulled != len(levers) {
		dumpMap(m, reachable(m, m.GetNamedLocation("ladder_up")))
		fail("quests: only %d of %d skull levers reachable one after another", pulled, len(levers))
	}
	if len(levers) == 0 || !nextToDiablo() {
		fail("quests: %d levers didn't open Diablo's chamber", len(levers))
	}
	slay("The Dark Lord", d1.QDiablo)
	fmt.Println("quests: Leoric, staff, Cain, red portal, Lazarus, save/load, levers and Diablo OK")
}

// questLines walks every other quest line of D1 with a fresh warrior, all quests made available (InitQuests leaves
// out one of each group per seed): who starts it, the set pieces that open, the items handed in and the rewards.
func questLines(theGame *game.GameInstance, fail func(string, ...any)) {
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "")
	state, broker := theGame.GetWorldState(), theGame.GetActionBroker()
	for q := range state.D1Quests() {
		state.D1Quests()[q].Active, state.D1Quests()[q].Var2 = d1.QuestInit, 0
	}
	player := func() *common.Actor { return state.GetPlayer() }
	cause := func() common.Cause { return common.NewActorCause("playthrough", player()) }
	travel := func(dlvl int) *gridmap.GridMap[*common.Actor, *common.Item, common.Object] {
		state.MapTransition(gridmap.Transition{TargetMap: defs.LevelNameFromDepth(dlvl), TargetLocation: "ladder_up"})
		state.SetLastDungeonLevel(max(state.GetLastDungeonLevel(), dlvl))
		common.UpdateD1Quests(state, state.GetMap())
		return state.GetMap()
	}
	must := func(ok bool, format string, args ...any) {
		if !ok {
			fail("quest lines: "+format, args...)
		}
	}
	log := func(q int) { must(state.QuestLog(q), "%s isn't in the quest log", state.QuestDefs()[q].Name) }
	done := func(q int) { must(state.QuestActive(q) == d1.QuestDone, "%s isn't done", state.QuestDefs()[q].Name) }
	pickUp := func(idx int) {
		m := state.GetMap()
		for _, it := range m.Items() {
			if it.D1() == nil || it.D1().BaseIdx != idx || !m.IsTileWalkable(it.MapPosition()) {
				continue // Valor's blood stones wait in alcoves the pedestal opens one by one
			}
			cells := m.GetFreeCellsForDistribution(it.MapPosition(), 1, func(p geometry.Point) bool { return m.CanPlaceActorHere(p) && !m.IsItemAt(p) })
			must(len(cells) > 0, "no room next to %s", it.Name())
			m.RemoveItem(it)
			m.AddItem(it, cells[0])
			m.MoveActor(player(), cells[0])
			broker.Execute(common.NewPickUpAction(cause(), player()))
			return
		}
		fail("quest lines: no %s on %s", theGame.GetFactory().D1().D.Items[idx].Name, m.GetName())
	}
	object := func(kind int) *common.D1Object {
		for _, obj := range state.GetMap().Objects() {
			if o, ok := obj.(*common.D1Object); ok && o.O.Kind == kind {
				return o
			}
		}
		fail("quest lines: no object %d on %s", kind, state.GetMap().GetName())
		return nil
	}
	monster := func(ai string) *common.Actor {
		for _, a := range state.GetMap().Actors() {
			if mon := a.D1Monster(); mon != nil && mon.AI == ai && a.IsAlive() {
				return a
			}
		}
		fail("quest lines: no %s on %s", ai, state.GetMap().GetName())
		return nil
	}
	reward := func(uid int) {
		for _, it := range state.GetMap().Items() {
			if it.D1() != nil && it.D1().Uid == uid {
				return
			}
		}
		fail("quest lines: no %s on %s", theGame.GetFactory().D1().D.UniqueItems[uid].Name, state.GetMap().GetName())
	}
	opens := func(loc string, open func()) {
		m := state.GetMap()
		p := m.GetNamedLocation(loc)
		must(!reachable(m, m.GetNamedLocation("ladder_up"))[p], "%s on %s is open too early", loc, m.GetName())
		open()
		must(reachable(m, m.GetNamedLocation("ladder_up"))[p], "%s on %s didn't open", loc, m.GetName())
	}
	f := theGame.GetFactory()

	talkTo(theGame, common.VendorTypeDeadGuy) // The Butcher
	log(d1.QButcher)

	travel(4)
	talkTo(theGame, common.VendorTypeOgden) // The Curse of King Leoric, then Ogden's Sign
	log(d1.QSkelKing)
	tomb := travel(3)
	if t, ok := tomb.GetTransitionAt(tomb.GetNamedLocation("leoric")); ok {
		state.MapTransition(t)
	} else {
		fail("quest lines: no way into King Leoric's Tomb")
	}
	// King Leoric's Tomb: breaking its four crucified skeletons opens the side room (BreakCrux)
	cruxes := 0
	walkable := func() (n int) {
		m := state.GetMap()
		for y := 0; y < d1.LevelSize; y++ {
			for x := 0; x < d1.LevelSize; x++ {
				if m.IsTileWalkable(geometry.Point{X: x, Y: y}) {
					n++
				}
			}
		}
		return
	}
	before := walkable()
	for kind := d1.ObjCRUX1; kind <= d1.ObjCRUX3; kind++ {
		for _, obj := range state.GetMap().Objects() {
			o, ok := obj.(*common.D1Object)
			if !ok || o.O.Kind != kind {
				continue
			}
			if before != walkable() {
				fail("quest lines: Leoric's tomb opened before the last crucified skeleton broke")
			}
			o.ActorBumpedInto(player(), nil)
			cruxes++
		}
	}
	if cruxes != 4 || walkable() <= before {
		fail("quest lines: breaking %d crucified skeletons didn't open Leoric's side room", cruxes)
	}
	talkTo(theGame, common.VendorTypeOgden)
	log(d1.QLtBanner)
	talkTo(theGame, common.VendorTypeWeapons) // The Magic Rock
	log(d1.QRock)

	travel(4)
	f.TalkToD1Monster(monster("AI_SNOTSPIL"))
	must(state.QuestVar(d1.QLtBanner, 1) == 2, "Snotspill's talk didn't send the hero for the banner")
	object(d1.ObjSIGNCHEST).ActorBumpedInto(player(), nil)
	pickUp(d1.IdiBanner)
	garbud := monster("AI_GARBUD") // Gharbad the Weak: talks, gives, talks, and fights
	for n := 1; n <= 4; n++ {
		must(garbud.D1Monster().TalkMsg == fmt.Sprintf("TEXT_GARBUD%d", n), "Gharbad says %s, not his line %d", garbud.D1Monster().TalkMsg, n)
		items := len(state.GetMap().Items())
		f.TalkToD1Monster(garbud)
		must(n != 2 || len(state.GetMap().Items()) > items, "Gharbad gave nothing")
		f.D1TalkerTurn(garbud, n == 4)
	}
	log(d1.QGarbud)
	must(garbud.D1Monster().TalkMsg == "" && f.D1TalkerTurn(garbud, true), "Gharbad doesn't fight after his last line")
	talkTo(theGame, common.VendorTypeOgden)
	done(d1.QLtBanner)
	reward(d1.UItemHarCrest)

	travel(5) // The Magic Rock, Valor
	pickUp(d1.IdiRock)
	object(d1.ObjBLOODBOOK).ActorBumpedInto(player(), nil)
	log(d1.QBlood)
	for i := 0; i < 3; i++ {
		pickUp(d1.IdiBldStone)
		object(d1.ObjPEDISTAL).ActorBumpedInto(player(), nil)
	}
	pickUp(d1.IdiArmOfVal)
	done(d1.QBlood)
	talkTo(theGame, common.VendorTypeWeapons)
	done(d1.QRock)
	reward(d1.UItemInfraRing)

	travel(6) // The Chamber of Bone
	opens("schamb", func() { object(d1.ObjBOOK2R).ActorBumpedInto(player(), nil) })
	log(d1.QSchamb)
	state.MapTransition(gridmap.Transition{TargetMap: "!gen_bonechamb_6", TargetLocation: "ladder_up"})
	guardian := player().Hero().SpellLevels[d1.SplGuardian]
	object(d1.ObjBOOK2L).ActorBumpedInto(player(), nil)
	must(player().Hero().SpellLevels[d1.SplGuardian] == guardian+1, "the Ancient Tome taught no Guardian")
	done(d1.QSchamb)

	travel(7) // Halls of the Blind
	object(d1.ObjBLINDBOOK).ActorBumpedInto(player(), nil)
	pickUp(d1.IdiOptAmulet)
	done(d1.QBlind)

	travel(8) // Zhar the Mad
	f.TalkToD1Monster(monster("AI_ZHAR"))
	log(d1.QZhar)

	travel(9) // Black Mushroom
	pickUp(d1.IdiFungalTm)
	talkTo(theGame, common.VendorTypeWands)
	log(d1.QMushroom)
	travel(9)
	object(d1.ObjMUSHPATCH).ActorBumpedInto(player(), nil)
	pickUp(d1.IdiMushroom)
	talkTo(theGame, common.VendorTypeWands)
	must(state.QuestVar(d1.QMushroom, 1) == d1.QSMushGiven, "Adria didn't take the mushroom")
	m := travel(9)
	for _, a := range m.Actors() {
		if mon := a.D1Monster(); mon != nil && mon.Unique == nil && a.IsAlive() {
			broker.Execute(common.NewChangeResourceAction(cause(), a, stats.Health, -1e6))
			break
		}
	}
	pickUp(d1.IdiBrain)
	talkTo(theGame, common.VendorTypeGeneral)
	pickUp(d1.IdiSpecElix)
	talkTo(theGame, common.VendorTypeWands)
	done(d1.QMushroom)

	talkTo(theGame, common.VendorTypeWeapons) // Anvil of Fury
	log(d1.QAnvil)
	travel(10)
	pickUp(d1.IdiAnvil)
	talkTo(theGame, common.VendorTypeWeapons)
	done(d1.QAnvil)
	reward(d1.UItemGriswold)

	travel(13) // Warlord of Blood: speaks once seen, then fights
	warlord := monster("AI_WARLORD")
	must(!f.D1TalkerTurn(warlord, true) && f.D1TalkerTurn(warlord, true), "the Warlord doesn't speak, then fight")

	travel(14) // Lachdanan
	lachdanan := monster("AI_LACHDAN")
	f.TalkToD1Monster(lachdanan)
	log(d1.QVeil)
	travel(15)
	pickUp(d1.IdiGldnElix)
	travel(14)
	lachdanan = monster("AI_LACHDAN")
	f.TalkToD1Monster(lachdanan)
	reward(d1.UItemSteelVeil)
	f.D1TalkerTurn(lachdanan, true)
	done(d1.QVeil)
	must(!lachdanan.IsAlive(), "Lachdanan didn't find his rest")

	townWater := func(poisoned bool) { // foul until the quest is done
		c := state.GetWaterColor()
		must(defs.IsTownLevel(state.GetMap().GetName()) && (c.G > c.B) == poisoned, "town water poisoned=%v wanted on %s (quest %d), color %v", poisoned, state.GetMap().GetName(), state.QuestActive(d1.QPWater), c)
	}
	talkTo(theGame, common.VendorTypeGeneral) // Poisoned Water Supply
	log(d1.QPWater)
	townWater(true)
	travel(2)
	state.MapTransition(gridmap.Transition{TargetMap: "!gen_pwater_2", TargetLocation: "ladder_up"})
	water := func(want string) { // the foul water is tinted light green, the clean water blue
		n := 0
		for y := 0; y < d1.LevelSize; y++ {
			for x := 0; x < d1.LevelSize; x++ {
				if t := state.GetMap().GetCell(geometry.Point{X: x, Y: y}).TileType; t.IsWater() && t.DefinedDescription == want {
					n++
				}
			}
		}
		if c := state.GetWaterColor(); n == 0 || (want == "poisoned water") != (c.G > c.B) {
			fail("quest lines: %d cells of %s, water color %v", n, want, c)
		}
	}
	water("poisoned water")
	for _, a := range append([]*common.Actor(nil), state.GetMap().Actors()...) {
		if a.D1Monster() != nil && a.IsAlive() {
			broker.Execute(common.NewChangeResourceAction(cause(), a, stats.Health, -1e6))
		}
	}
	done(d1.QPWater)
	water("clean water")
	talkTo(theGame, common.VendorTypeGeneral)
	reward(d1.UItemTRing)
	townWater(false)
	fmt.Println("quest lines: Butcher, Leoric, Ogden's Sign, Gharbad, Magic Rock, Valor, Chamber of Bone, Blind, Zhar, Black Mushroom, Anvil, Warlord, Lachdanan, Poisoned Water OK")
}

// pullD1Levers pulls every skull lever the hero can walk to, again and again as pulled levers open the level's
// quarters (diab1-4), and reports how many were pulled.
func pullD1Levers(state *common.State) (pulled int) {
	m := state.GetMap()
	for progress := true; progress; {
		progress = false
		reach := reachable(m, m.GetNamedLocation("ladder_up"))
		for _, obj := range m.Objects() {
			if o, ok := obj.(*common.D1Object); ok && o.O.Kind == d1.ObjSWITCHSKL && o.O.Active {
				for _, p := range m.GetAllCardinalNeighbors(o.MapPosition()) { // monsters may crowd round a lever
					if reach[p] && o.O.Active {
						o.ActorBumpedInto(state.GetPlayer(), nil)
						pulled, progress = pulled+1, true
					}
				}
			}
		}
	}
	return
}

// talkTo walks to town and talks to the townsperson of vendor type v, like bumping into them.
func talkTo(theGame *game.GameInstance, v common.VendorType) {
	state := theGame.GetWorldState()
	if !defs.IsTownLevel(state.GetMap().GetName()) {
		state.MapTransition(gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: "spawn"})
	}
	for _, a := range state.GetMap().Actors() {
		if a.GetVendorType() == v {
			theGame.GetActionBroker().Execute(common.NewTalkToAction(common.NewActorCause("playthrough", state.GetPlayer()), a))
			return
		}
	}
	fmt.Printf("PLAYTHROUGH FAILED: nobody in town for %s\n", v)
	os.Exit(1)
}

// d1Carried reports an item of base idx in the player's pack.
func d1Carried(player *common.Actor, idx int) bool {
	for _, it := range player.GetInventory().Items() {
		if it.D1() != nil && it.D1().BaseIdx == idx {
			return true
		}
	}
	return false
}

// checkItemEffects: a fire weapon with knockback burns and pushes a monster, a drain-life item drains the hero.
func checkItemEffects(broker *common.Broker, state *common.State, fail func(string, ...any)) {
	player, m := state.GetPlayer(), state.GetMap()
	var weapon *d1.Item
	for _, it := range player.GetEquipment().AllItems() {
		if d := it.D1(); d != nil && d.Class == "ICLASS_WEAPON" {
			weapon = d
		}
	}
	weapon.Flags |= d1.ISplFireDam | d1.ISplKnockback
	weapon.FMinDam, weapon.FMaxDam = 200, 200
	common.SyncHero(player)
	var target *common.Actor
	var from geometry.Point // a free cell next to the target with a free cell behind the target to push it into
	for _, a := range m.Actors() {
		if mon := a.D1Monster(); target == nil && mon != nil && a.IsAlive() && mon.MagicRes&(d1.ImmuneFire|d1.ResistFire) == 0 {
			for _, p := range m.GetFreeCardinalNeighbors(a.MapPosition()) {
				if behind := a.MapPosition().Add(a.MapPosition().Sub(p)); m.Contains(behind) && m.IsCurrentlyPassable(behind) {
					target, from = a, p
					break
				}
			}
		}
	}
	target.GetStats().SetAttribute(stats.MaxHealth, 100000)
	target.GetStats().GetResource(stats.Health).SetToMax()
	burned, pushed := false, false
	for try := 0; try < 200 && !(burned && pushed); try++ {
		if !pushed {
			m.MoveActor(player, from)
		}
		player.SetDrawPosition(player.MapPosition().ToPointF())
		hp, pos := target.GetStats().GetResource(stats.Health).Value(), target.MapPosition()
		broker.Execute(common.NewMeleeAttackAction(common.NewActorCause("playthrough", player), player, target))
		burned = burned || hp-target.GetStats().GetResource(stats.Health).Value() >= 200
		pushed = pushed || target.MapPosition() != pos
	}
	if !burned || !pushed {
		fail("item effects: fire damage %v, knockback %v", burned, pushed)
	}

	weapon.Flags = weapon.Flags&^(d1.ISplFireDam|d1.ISplKnockback) | d1.ISplDrainLife
	common.SyncHero(player)
	var stoned []*common.Actor // monsters stay out of it
	for _, a := range m.Actors() {
		if a.D1Monster() != nil && a.IsAlive() {
			a.AddStatusEffect(common.StatusStone, 100)
			stoned = append(stoned, a)
		}
	}
	hp := player.GetStats().GetResource(stats.Health)
	hp.SetToMax()
	for turn := 0; turn < 20; turn++ {
		broker.Execute(common.NewEndPlayerTurnAction(common.AbstractCause("playthrough"), 10))
	}
	if lost := hp.GetMax() - hp.Value(); lost < 9 || lost > 11 {
		fail("item effects: drain life took %d HP in 20 turns (want 10)", lost)
	}
	weapon.Flags &^= d1.ISplDrainLife
	common.SyncHero(player)
	for _, a := range stoned {
		a.RemoveStatusEffect(common.StatusStone)
	}
	hp.SetToMax()
}

// checkClassSkill fires the hero's class skill: the Warrior's Item Repair, the Rogue's Trap Disarm, the Sorcerer's
// Staff Recharge, the Monk's Vision, the Bard's Identify and the Barbarian's Rage. Rage lingers ticked through
// fury and hangover and checked on both sides; Vision lingers until the level changes.
func checkClassSkill(f *common.Factory, player *common.Actor, fail func(string, ...any)) {
	h, name := player.Hero(), d1.ClassNames[player.Hero().Class]
	calm := player.GetStats().GetAttribute(stats.Strength)
	// D1SpellSkills always puts the class skill first, targeted for the Rogue and untargeted for everyone else
	switch skills := player.GetUntargetedSkills(); {
	case len(skills) > 0:
		skills[0].Invoke(player, common.EffectSource("playthrough"))
	case len(player.GetTargetedSkills()) > 0:
		player.GetTargetedSkills()[0].Execute(player, player.MapPosition(), player.MapPosition())
	default:
		fail("%s: no class skill to fire", name)
	}
	if h.Class != d1.Barbarian {
		return
	}
	raging := player.GetStats().GetAttribute(stats.Strength)
	if h.Rage <= 0 || raging != calm+2*h.Level {
		fail("barbarian: Rage gave %d strength instead of %d, %d turns left", raging, calm+2*h.Level, h.Rage)
	}
	hung := 0
	for range 4 * d1.RageTurns(h.Level) { // fury, hangover, and back to normal
		player.BeforeTurn()
		if h.Rage < 0 {
			hung = player.GetStats().GetAttribute(stats.Strength)
		}
	}
	if hung != calm-2*h.Level || h.Rage != 0 || player.GetStats().GetAttribute(stats.Strength) != calm {
		fail("barbarian: Rage ran %d -> %d -> %d -> %d strength, %d turns left",
			calm, raging, hung, player.GetStats().GetAttribute(stats.Strength), h.Rage)
	}
}

// checkDualWield: only the Bard fills both hands with weapons, and a shield or a two-hander still takes the hand
// back off him. Everyone else keeps one weapon and an empty off hand.
func checkDualWield(player *common.Actor, fail func(string, ...any)) {
	e := player.GetEquipment()
	main, off := e.GetBySlot(defs.SlotNameMainHand), e.GetBySlot(defs.SlotNameOffHand)
	isWeapon := func(it *common.Item) bool { return it != nil && it.D1() != nil && it.D1().Class == "ICLASS_WEAPON" }
	if player.Hero().Class != d1.Bard {
		if isWeapon(off) {
			fail("%s: a weapon ended up in the off hand", d1.ClassNames[player.Hero().Class])
		}
		return
	}
	if !isWeapon(main) || !isWeapon(off) {
		fail("bard: started with %v / %v instead of a weapon in each hand", main, off)
	}
	dual := player.HeroSheet()
	if !dual.Holds(d1.ITypeSword) || dual.MinDam < 2 {
		fail("bard: both blades should count, got mask %b damage %d-%d", dual.WeaponMask, dual.MinDam, dual.MaxDam)
	}
}

// identifyPickUI picks the first offered item and records the description window Identify opens for it.
type identifyPickUI struct {
	common.UserInterface
	shown *common.Item
}

func (u *identifyPickUI) OpenItemList(items []*common.TextualStack) {
	if len(items) > 0 {
		items[0].OnSelected()
	}
}

func (u *identifyPickUI) ShowItemInfo(item *common.Item) { u.shown = item }
