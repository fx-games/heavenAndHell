package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"rpg/src/common"
	"rpg/src/defs"
	"rpg/src/game"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/stats"
)

// autoplay is a bot that plays a mod's class down the dungeon to measure its pacing: per depth, the hero's level on
// arrival, the turns, kills and experience it took, potions drunk, trips home, gear equipped, spells cast and gold.
// The ideas are the Angband Borg's (repo.or.cz/angband/borg.git), cut down: explore everything, cast what reaches
// the nearest enemy, else fight it, drink below half health, equip what beats what it wears, and go home when a
// fight turns bad or the light burns out, buying healing potions and a light there.
//
//	go run ./src autoplay <mod> [seed] [runs] [class index]
//
// The healer in town heals for free (D1's Pepin, H&H's Kirioth), so the trip home only skips the walk.
// ponytail: the portal is free and the bot never identifies; model them when they start to matter.
func autoplay() {
	mod, seed, runs, class := "heavenandhell", int64(1), 1, 0
	if len(os.Args) > 2 {
		mod = os.Args[2]
	}
	if len(os.Args) > 3 {
		seed, _ = strconv.ParseInt(os.Args[3], 10, 64)
	}
	if len(os.Args) > 4 {
		runs, _ = strconv.Atoi(os.Args[4])
	}
	if len(os.Args) > 5 {
		class, _ = strconv.Atoi(os.Args[5])
	}
	game.SaveDir = filepath.Join(os.TempDir(), fmt.Sprintf("heavenAndHell_autoplay_%d", os.Getpid()))
	defer os.RemoveAll(game.SaveDir)
	modOverride = mod
	g, _ := createGameAndUIContext()
	g.GetAudio().SetDisablePlayback(true)
	common.Clock = func() time.Time { return time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC) }
	fmt.Printf("autoplay %s, %s, seeds %d..%d\n%5s %4s %4s %6s %5s %6s %5s %5s %5s %5s %6s\n", mod, g.GetGameData().ClassDefinitions[class].Name, seed, seed+int64(runs)-1,
		"seed", "dlvl", "clvl", "turns", "kills", "exp", "pots", "home", "equip", "casts", "gold")
	for r := int64(0); r < int64(runs); r++ {
		g.SeedRandom(seed + r)
		g.NewGame(g.GetGameData().ClassDefinitions[class], false, "")
		botRun(g, seed+r)
	}
}

// botRun plays one hero from depth 1 until it dies or the dungeon ends, printing a line per depth.
func botRun(g *game.GameInstance, seed int64) {
	s, broker := g.GetWorldState(), g.GetActionBroker()
	player := func() *common.Actor { return s.GetPlayer() }
	hp := func() *stats.Resource { return player().GetStats().GetResource(stats.Health) }
	cause := common.AbstractCause("autoplay")
	turns, kills, pots, home, equips, casts := 0, 0, 0, 0, 0, 0
	potion := func(it *common.Item, kind string) bool { return it.IsPotion() && strings.Contains(it.Name(), kind) }
	tick := func() {
		for i := 0; i < 20; i++ {
			broker.Tick()
		}
	}
	// spendPoints puts stat points where the class's caps are highest (the lowest share of its cap first) and skill
	// points into the least-raised skill
	spendPoints := func() {
		for more := true; more; {
			more = false
			var pick *common.SheetAttribute
			for _, rows := range [][]common.SheetAttribute{player().SheetAttributes(), s.SkillRows(player())} {
				pick = nil
				for i, r := range rows {
					if r.CanRaise && (pick == nil || r.Base*pick.Max < pick.Base*r.Max) {
						pick = &rows[i]
					}
				}
				if pick != nil {
					pick.Raise()
					more = true
				}
			}
		}
		common.SyncHero(player())
	}
	actions := g.GetActions().(*game.Actions)
	// upgrade equips what beats what the hero wears on every value the item tooltip compares, or fills an empty slot
	isUpgrade := func(it *common.Item) bool {
		if slot := it.SlotName(); !it.IsEquippable() || !it.MeetsRequirements() || it.IsLightSource() || slot == defs.SlotNameNotEquippable || slot == defs.SlotNameQuiver || slot >= defs.SlotNameQuickSlot1 { // throwables and belt items are no upgrade
			return false
		}
		displaced, cmp := player().GetEquipment().CompareForEquipping(it)
		better, worse := len(displaced) == 0, false
		for _, v := range cmp {
			better, worse = better || v > 0, worse || v < 0
		}
		return better && !worse
	}
	upgrade := func() {
		for _, it := range player().GetInventory().Items() {
			if isUpgrade(it) {
				if os.Getenv("AUTOPLAY_DEBUG") != "" {
					fmt.Printf("  equip %s\n", it.Name())
				}
				actions.EquipItem(it)
				tick()
				turns++
				equips++
			}
		}
	}
	// cast fires the first skill with the mana for it that can reach target; false when none can
	cast := func(target *common.Actor) bool {
		for _, e := range common.SkillBar(player()) {
			t, ok := e.(*common.TargetedAction)
			if !ok || s.LacksMana(player(), t.Name()) {
				continue
			}
			if !slices.Contains(t.GetTargetingRules(player(), s.GetMap()).Targets(), target.MapPosition()) {
				continue
			}
			t.OnSelected(player(), target.MapPosition(), t.GetHitPositions(player().MapPosition(), target.MapPosition()))
			tick()
			turns++
			casts++
			return true
		}
		return false
	}
	// goHome is a trip to town: healed, then the healing potions and a fresh light the gold buys
	goHome := func() {
		level, at := s.GetMap().GetName(), player().MapPosition()                                // a town portal: back to this spot afterwards
		s.MapTransition(gridmap.Transition{TargetMap: s.DeepestTown(), TargetLocation: "spawn"}) // the shops restock
		hp().SetToMax()                                                                          // ponytail: the portal and the walk to the healer are free
		home++
		light := player().GetEquipment().GetLightSource()
		for _, v := range s.GetMap().Actors() {
			if !v.IsVendor() {
				continue
			}
			for _, stack := range common.SellableStacks(v, player()) { // everything it doesn't wear or drink
				for _, it := range stack {
					if !potion(it, "Healing") && !potion(it, "Mana") {
						common.SellItem(player(), it)
					}
				}
			}
		}
		for _, v := range s.GetMap().Actors() {
			if !v.IsVendor() {
				continue
			}
			for _, it := range v.GetInventory().Items() {
				needLight := it.IsLightSource() && (light == nil || light.GetCharges() < 500)
				gear := isUpgrade(it) && it.BuyPrice() <= player().GetStats().GetAttribute(stats.Gold)/2 // keep potion money
				if (needLight || gear || potion(it, "Healing") || casts > 0 && potion(it, "Mana")) && it.BuyPrice() <= player().GetStats().GetAttribute(stats.Gold) {
					common.BuyItem(player(), v, it)
					if gear {
						upgrade()
					}
					if needLight {
						actions.EquipItem(it)
						tick()
						light = it
					}
				}
			}
		}
		s.MapTransition(gridmap.Transition{TargetMap: level, TargetLocation: "ladder_up"})
		s.GetMap().MoveActor(player(), at)
		broker.Execute(common.NewUpdateFoVAction(cause, player()))
	}
	act := func(a common.Action) {
		broker.Execute(a)
		broker.Execute(common.NewEndPlayerTurnAction(cause, 10))
		for i := 0; i < 20; i++ {
			broker.Tick()
		}
		turns++
		if os.Getenv("AUTOPLAY_DEBUG") != "" {
			fmt.Printf("turn %d hp %d/%d at %v %T\n", turns, hp().Value(), hp().GetMax(), player().MapPosition(), a)
		}
		spendPoints()
	}
	fraction := func() float64 { return float64(hp().Value()) / float64(max(hp().GetMax(), 1)) }
	drink := func(kind string) bool {
		for _, it := range player().GetInventory().Items() {
			if potion(it, kind) {
				actions.ActivateItemActionForPlayer(it)
				tick()
				pots++
				return true
			}
		}
		return false
	}
	// passable is where the bot walks: open floor, closed doors (it opens them) and unseen actors (it bumps them)
	passable := func(to geometry.Point) func(geometry.Point) bool {
		m := s.GetMap()
		return func(p geometry.Point) bool {
			if p == to || s.IsClosedDoorAt(p) {
				return m.IsTileWalkable(p)
			}
			if o, ok := m.TryGetObjectAt(p); ok && !o.IsWalkable(player()) {
				return false
			}
			a, ok := m.TryGetActorAt(p)
			return m.IsTileWalkable(p) && (!ok || a == player())
		}
	}
	// stepToward takes one step on the way to target (opening a door on the way); false when there is no way
	stepToward := func(target geometry.Point) bool {
		m := s.GetMap()
		path := m.GetJPSPath(player().MapPosition(), target, passable(target))
		if len(path) <= 1 {
			return false
		}
		next := path[1]
		switch a, ok := m.TryGetActorAt(next); {
		case s.IsClosedDoorAt(next):
			act(common.NewOpenDoorAction(cause, player(), next))
		case ok && a.IsHostileTo(player()):
			act(common.NewMeleeAttackAction(cause, player(), a))
		case ok:
			return false
		default:
			mv := common.NewMoveAction(cause, player(), next, true)
			mv.Step = true
			act(mv)
			if player().MapPosition() != next { // something unseen or unwalkable stood in the way
				return false
			}
			if _, item := m.TryGetItemAt(next); item {
				act(common.NewPickUpAction(cause, player()))
				upgrade()
			}
		}
		return true
	}

	for dlvl := 1; ; dlvl++ {
		level := levelAt(s, dlvl)
		if level == "" {
			fmt.Printf("%5d cleared the dungeon\n", seed)
			return
		}
		s.MapTransition(gridmap.Transition{TargetMap: level, TargetLocation: "ladder_up"})
		if os.Getenv("AUTOPLAY_DEBUG") != "" {
			n := 0
			for _, a := range s.GetMap().Actors() {
				if a.Monster() != nil && a.IsAlive() {
					n++
				}
			}
			fmt.Printf("depth %d: %d monsters, light radius %d, %d walkable cells\n", dlvl, n, s.PlayerLightRadius(), len(reachable(s.GetMap(), player().MapPosition())))
			byType := map[string]int{}
			for _, a := range s.GetMap().Actors() {
				if a.Monster() != nil && a.IsAlive() {
					byType[fmt.Sprintf("%s(mlvl %d, exp %d)", a.Monster().Data.ID, a.Monster().Data.Level, a.Monster().Data.Exp)]++
				}
			}
			fmt.Println(byType)
		}
		clvl, e0, t0, k0, p0, h0, q0, c0 := player().Hero().Level, player().Hero().Exp, turns, kills, pots, home, equips, casts
		skip := map[*common.Item]bool{} // items the bot gave up on
		goal := player().MapPosition()  // the explore target, kept until reached: re-picking every step oscillates
		hasUnexplored := func(p geometry.Point) bool {
			m := s.GetMap()
			return len(m.GetFilteredNeighbors(p, func(n geometry.Point) bool { return m.IsTileWalkable(n) && !m.IsExplored(n) })) > 0
		}
		for budget := 4000; budget > 0 && player().IsAlive(); budget-- {
			enemies := s.GetVisibility().GetVisibleEnemies()
			if fraction() < 0.5 && !drink("Healing") && (len(enemies) > 0 || fraction() < 0.3) || s.PlayerLightRadius() == 0 {
				goHome()
				if s.PlayerLightRadius() == 0 {
					fmt.Printf("%5d stranded on depth %d at character level %d: no light and no gold for one\n", seed, dlvl, player().Hero().Level)
					return
				}
				continue
			}
			if len(enemies) > 0 {
				target := enemies[0]
				for _, e := range enemies[1:] {
					if geometry.DistanceChebyshev(e.MapPosition(), player().MapPosition()) < geometry.DistanceChebyshev(target.MapPosition(), player().MapPosition()) {
						target = e
					}
				}
				if alive := target.IsAlive(); cast(target) || casts > 0 && drink("Mana") && cast(target) {
					if alive && !target.IsAlive() {
						kills++
					}
					continue
				}
				if geometry.DistanceChebyshev(target.MapPosition(), player().MapPosition()) <= 1 {
					alive := target.IsAlive() // the dying stay in view until their death animation ends
					act(common.NewMeleeAttackAction(cause, player(), target))
					if alive && !target.IsAlive() {
						kills++
					}
					continue
				}
				if stepToward(target.MapPosition()) {
					continue
				}
				act(common.NewEndPlayerTurnAction(cause, 0)) // an archer out of reach: wait for it
				continue
			}
			m := s.GetMap()
			// items it has seen come first: loot is how the bot gears up and pays for potions
			wanted := func(it *common.Item) bool {
				return !skip[it] && m.IsExplored(it.MapPosition()) && player().CanPickup(it)
			}
			if it, ok := m.TryGetItemAt(player().MapPosition()); ok && wanted(it) {
				act(common.NewPickUpAction(cause, player()))
				upgrade()
				skip[it] = true // picked up, or it can't be
				continue
			}
			if p := m.GetNearestExploredItem(player().MapPosition(), wanted); p != player().MapPosition() {
				if !stepToward(p) {
					if it, ok := m.TryGetItemAt(p); ok {
						skip[it] = true
					}
				}
				continue
			}
			if goal == player().MapPosition() { // reached: what the light didn't show from here, it never will
				for _, n := range m.GetFilteredNeighbors(goal, func(geometry.Point) bool { return true }) {
					m.SetExplored(n)
				}
			}
			if goal == player().MapPosition() || !m.IsTileWalkable(goal) || !hasUnexplored(goal) {
				goal = m.GetNearestUnexploredWalkablePosition(player().MapPosition())
				if goal == player().MapPosition() {
					break // explored all it can reach
				}
			}
			if !stepToward(goal) { // out of the bot's reach (behind an object): give its frontier up
				for _, n := range m.GetFilteredNeighbors(goal, func(geometry.Point) bool { return true }) {
					m.SetExplored(n)
				}
				goal = player().MapPosition()
			}
		}
		if os.Getenv("AUTOPLAY_DEBUG") != "" {
			reach := reachable(s.GetMap(), player().MapPosition())
			left, inReach, seen := 0, 0, 0
			for _, a := range s.GetMap().Actors() {
				if a.Monster() != nil && a.IsAlive() {
					left++
					if reach[a.MapPosition()] {
						inReach++
					}
					if s.GetMap().IsExplored(a.MapPosition()) {
						seen++
					}
				}
			}
			fmt.Printf("left %d, reachable %d, on explored cells %d, explored %d cells\n", left, inReach, seen, func() (n int) {
				for p := range reach {
					if s.GetMap().IsExplored(p) {
						n++
					}
				}
				return
			}()*100/max(len(reach), 1))
		}
		fmt.Printf("%5d %4d %4d %6d %5d %6d %5d %5d %5d %5d %6d\n", seed, dlvl, clvl, turns-t0, kills-k0, player().Hero().Exp-e0, pots-p0, home-h0, equips-q0, casts-c0, player().GetStats().GetAttribute(stats.Gold))
		if !player().IsAlive() {
			fmt.Printf("%5d died on depth %d at character level %d\n", seed, dlvl, player().Hero().Level)
			return
		}
	}
}
