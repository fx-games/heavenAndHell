package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"rpg/src/common"
	"rpg/src/game"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/rules"
	"rpg/src/stats"
)

// autoplay is a bot that plays a mod's first class down the dungeon to measure its pacing: per depth, the hero's
// level on arrival, the turns and kills it took, the potions drunk and the trips home. The ideas are the Angband
// Borg's (repo.or.cz/angband/borg.git), cut down: explore everything, fight what it sees, drink below half health,
// and go home when a fight turns bad.
//
//	go run ./src autoplay <mod> [seed] [runs]
//
// ponytail: a trip home is a free full heal (no walk, no shop, no gold), and the bot keeps its starting gear and
// never casts; buy potions, equip drops and cast spells when the numbers need to reflect a real player's pacing.
func autoplay() {
	mod, seed, runs := "heavenandhell", int64(1), 1
	if len(os.Args) > 2 {
		mod = os.Args[2]
	}
	if len(os.Args) > 3 {
		seed, _ = strconv.ParseInt(os.Args[3], 10, 64)
	}
	if len(os.Args) > 4 {
		runs, _ = strconv.Atoi(os.Args[4])
	}
	game.SaveDir = filepath.Join(os.TempDir(), fmt.Sprintf("heavenAndHell_autoplay_%d", os.Getpid()))
	defer os.RemoveAll(game.SaveDir)
	modOverride = mod
	g, _ := createGameAndUIContext()
	g.GetAudio().SetDisablePlayback(true)
	common.Clock = func() time.Time { return time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC) }
	fmt.Printf("autoplay %s, seeds %d..%d\n%5s %4s %4s %6s %5s %5s %5s %s\n", mod, seed, seed+int64(runs)-1,
		"seed", "dlvl", "clvl", "turns", "kills", "pots", "home", "")
	for r := int64(0); r < int64(runs); r++ {
		g.SeedRandom(seed + r)
		g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
		botRun(g, seed+r)
	}
}

// botRun plays one hero from depth 1 until it dies or the dungeon ends, printing a line per depth.
func botRun(g *game.GameInstance, seed int64) {
	s, broker := g.GetWorldState(), g.GetActionBroker()
	player := func() *common.Actor { return s.GetPlayer() }
	hp := func() *stats.Resource { return player().GetStats().GetResource(stats.Health) }
	cause := common.AbstractCause("autoplay")
	turns, kills, pots, home := 0, 0, 0, 0
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
		h := player().Hero()
		for ; h.StatPts > 0; h.StatPts-- { // Str, Dex and Vit in turn, like the playthrough's boost
			h.AddBaseStat([]int{rules.AttrStr, rules.AttrDex, rules.AttrVit}[h.StatPts%3], 1)
		}
		common.SyncHero(player())
	}
	fraction := func() float64 { return float64(hp().Value()) / float64(max(hp().GetMax(), 1)) }
	drink := func() bool {
		for _, it := range player().GetInventory().Items() {
			if it.IsPotion() && strings.Contains(it.Name(), "Healing") {
				g.GetActions().(*game.Actions).ActivateItemActionForPlayer(it)
				for i := 0; i < 20; i++ {
					broker.Tick()
				}
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
			fmt.Printf("depth %d: %d monsters\n", dlvl, n)
		}
		clvl, t0, k0, p0, h0 := player().Hero().Level, turns, kills, pots, home
		goal := player().MapPosition() // the explore target, kept until reached: re-picking every step oscillates
		hasUnexplored := func(p geometry.Point) bool {
			m := s.GetMap()
			return len(m.GetFilteredNeighbors(p, func(n geometry.Point) bool { return m.IsTileWalkable(n) && !m.IsExplored(n) })) > 0
		}
		for budget := 4000; budget > 0 && player().IsAlive(); budget-- {
			enemies := s.GetVisibility().GetVisibleEnemies()
			if fraction() < 0.5 && !drink() && (len(enemies) > 0 || fraction() < 0.3) {
				hp().SetToMax() // ponytail: a free town portal and healer
				home++
				continue
			}
			if len(enemies) > 0 {
				target := enemies[0]
				for _, e := range enemies[1:] {
					if geometry.DistanceChebyshev(e.MapPosition(), player().MapPosition()) < geometry.DistanceChebyshev(target.MapPosition(), player().MapPosition()) {
						target = e
					}
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
		fmt.Printf("%5d %4d %4d %6d %5d %5d %5d\n", seed, dlvl, clvl, turns-t0, kills-k0, pots-p0, home-h0)
		if !player().IsAlive() {
			fmt.Printf("%5d died on depth %d at character level %d\n", seed, dlvl, player().Hero().Level)
			return
		}
	}
}
