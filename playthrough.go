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
	missiles := &missileCounter{} // registered per check: a new game or a load makes a new broker
	for run := 1; run <= 3; run++ {
		theGame.NewGame(theGame.GetGameData().ClassDefinitions[run-1], false)
		state := theGame.GetWorldState()
		player := state.GetPlayer()
		if player.Hero() == nil {
			fail("run %d: player has no D1 hero", run)
		}
		for dlvl := 1; dlvl <= 16; dlvl++ {
			name := defs.LevelNameFromDepth(dlvl)
			state.MapTransition(gridmap.Transition{TargetMap: name, TargetLocation: "ladder_up"})
			m := state.GetMap()
			if m.GetName() != name {
				fail("run %d: expected map %s, got %s", run, name, m.GetName())
			}
			// milestone 6: stairs (and the town warp on 5, 9, 13) reachable on every level type
			up := m.GetNamedLocation("ladder_up")
			reach := reachable(m, up)
			for _, loc := range []string{"ladder_down", "warp"} {
				p := m.GetNamedLocation(loc)
				exists := m.GetNamedLocationByPos(p) == loc
				want := loc == "ladder_down" && dlvl < 16 || loc == "warp" && (dlvl == 5 || dlvl == 9 || dlvl == 13)
				if exists != want {
					fail("run %d dlvl %d: %s exists=%v", run, dlvl, loc, exists)
				}
				if _, ok := reach[p]; exists && !ok {
					dumpMap(m, reach)
					fail("run %d dlvl %d: %s at %v unreachable from %v", run, dlvl, loc, p, up)
				}
			}
			// milestone 7: the quest bosses wait on their levels
			if boss := map[int]string{2: "The Butcher", 8: "Zhar the Mad", 13: "Warlord of Blood", 16: "The Dark Lord"}[dlvl]; boss != "" && actorNamed(m, boss) == nil {
				fail("run %d dlvl %d: %s is missing", run, dlvl, boss)
			}
			if dlvl > 4 {
				boost(player, dlvl)
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
			if dlvl < 16 && (objects < 9 || used == 0 || dropped == 0) {
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

			if run == 3 && dlvl == 2 { // milestone 5: the sorcerer casts every spell
				castAllSpells(theGame.GetFactory(), theGame.GetActionBroker(), state, player, fail)
				player = state.GetPlayer()
			}

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
	// milestone 3: the D1 town after a trip down to level 16
	theGame.GetWorldState().MapTransition(gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: "spawn"})
	checkTown(theGame.GetWorldState(), fail)
	if len(missiles.byAI) < 6 { // level monster types are random; six of the eight casting AIs always showed up
		fail("missile AIs: only %v cast", missiles.byAI)
	}
	fmt.Printf("monster missiles by AI: %v\n", missiles.byAI)

	questPath(theGame, fail)

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
	portal := *state.D1Portal()
	state.MapTransition(gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: "spawn"})
	if !state.GetMap().IsObjectAt(state.GetMap().GetNamedLocation("portal")) {
		fail("spells: no town portal in town")
	}
	state.MapTransition(gridmap.Transition{TargetMap: portal.Map, TargetLocation: "portal"})
	if state.D1Portal() != nil || state.GetMap().GetName() != portal.Map {
		fail("spells: town portal round trip didn't close the portal (%v on %s)", state.D1Portal(), state.GetMap().GetName())
	}
	fmt.Printf("spells: %d damage, failed casts %v\n", dealt, failed)
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
				if !o.IsWalkable(nil) && !isDoor && !(isD1 && (d1o.O.Kind == d1.ObjBarrel || d1o.O.Kind == d1.ObjBarrelEx)) {
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
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false)
	state := theGame.GetWorldState()
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
	slay := func(name, flag string) {
		a := actorNamed(state.GetMap(), name)
		if a == nil {
			fail("quests: %s not found on %s", name, state.GetMap().GetName())
		}
		hp := a.GetStats().GetResource(stats.Health).GetMax()
		theGame.GetActionBroker().Execute(common.NewChangeResourceAction(common.NewActorCause("playthrough", state.GetPlayer()), a, stats.Health, -float64(10*hp)))
		if !state.GetFlags().IsSet(flag) {
			fail("quests: killing %s didn't set %s", name, flag)
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
	through("leoric")
	slay("Skeleton King", common.QuestLeoric)
	through("ladder_up")
	if state.GetMap().GetName() != defs.LevelNameFromDepth(3) {
		fail("quests: Leoric's tomb leads to %s", state.GetMap().GetName())
	}

	m := travel(defs.LevelNameFromDepth(15), "ladder_up")
	if _, open := m.GetTransitionAt(m.GetNamedLocation("ladder_down")); open {
		fail("quests: level 15's stairs down are open before Lazarus died")
	}
	stands := objectsOf(d1.ObjLazStand)
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
	if !common.D1GiveStaffToCain(state, func(string) {}) {
		fail("quests: Cain didn't take the staff")
	}
	travel(defs.LevelNameFromDepth(15), "ladder_up")
	through("red_portal")
	m = state.GetMap()
	for _, name := range []string{"Arch-Bishop Lazarus", "Red Vex", "BlackJade"} {
		if actorNamed(m, name) == nil {
			fail("quests: %s is missing from %s", name, m.GetName())
		}
	}
	if _, open := m.GetTransitionAt(m.GetNamedLocation("ladder_up")); open {
		fail("quests: the lair's portal is open before Lazarus died")
	}
	slay("Arch-Bishop Lazarus", common.QuestLazarus)

	theGame.SaveGame()
	if err := theGame.LoadGame(); err != nil {
		fail("quests: load failed: %v", err)
	}
	state = theGame.GetWorldState()
	through("ladder_up")
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
	levers := objectsOf(d1.ObjLever)
	reach := reachable(m, m.GetNamedLocation("ladder_up"))
	for _, l := range levers {
		if len(m.GetFreeCardinalNeighbors(l.MapPosition())) == 0 || !reach[m.GetFreeCardinalNeighbors(l.MapPosition())[0]] {
			dumpMap(m, reach)
			fail("quests: lever at %v unreachable", l.MapPosition())
		}
		l.ActorBumpedInto(state.GetPlayer(), nil)
	}
	if len(levers) != 4 || !nextToDiablo() {
		fail("quests: %d levers didn't open Diablo's chamber", len(levers))
	}
	slay("The Dark Lord", common.QuestDiablo)
	fmt.Println("quests: Leoric, staff, Cain, red portal, Lazarus, save/load, levers and Diablo OK")
}
