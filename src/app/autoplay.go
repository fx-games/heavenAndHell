package app

import (
	"cmp"
	"errors"
	"fmt"
	"log"
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
	"rpg/src/gui"
	"rpg/src/rules"
	"rpg/src/stats"

	"github.com/hajimehoshi/ebiten/v2"
)

// autoplay is a bot that plays a mod's class down the dungeon to measure its pacing: per depth, the hero's level on
// arrival, the turns, kills and experience it took, potions drunk, trips home, gear equipped, spells cast and gold.
// The ideas are the Angband Borg's (repo.or.cz/angband/borg.git), cut down: explore everything, cast what reaches
// the nearest enemy, else fight it, drink below half health, equip what beats what it wears, and go home when a
// fight turns bad or the light burns out, buying healing potions and a light there.
//
//	go run ./src autoplay <mod> [seed] [runs] [class index]
//
// AUTOPLAY_TACTICAL=1 plays smarter: no spell into a resistance, back into a corridor when surrounded, the weakest
// monster in reach first. Criterion 4 of REVIEW.md wants that to beat the plain bot clearly.
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
	tactical := os.Getenv("AUTOPLAY_TACTICAL") != "" // plays to monster weaknesses and fights from corridors
	game.SaveDir = filepath.Join(os.TempDir(), fmt.Sprintf("heavenAndHell_autoplay_%d", os.Getpid()))
	defer os.RemoveAll(game.SaveDir)
	modOverride = mod
	watch := os.Getenv("AUTOPLAY_WATCH") != "" // in the game's window, one bot turn every AUTOPLAY_SPEED frames
	g, ctx := createGameAndUIContext()
	if !watch {
		g.GetAudio().SetDisablePlayback(true)
	}
	common.Clock = func() time.Time { return time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC) }
	play := func() {
		playRuns(g, mod, seed, runs, class, tactical)
	}
	if !watch {
		play()
		return
	}
	watchBot(g, ctx, play)
}

// playRuns plays runs heroes of the class, one seed each, printing the report.
func playRuns(g *game.GameInstance, mod string, seed int64, runs, class int, tactical bool) {
	fmt.Printf("autoplay %s, %s, tactical %v, seeds %d..%d\n%5s %4s %4s %6s %5s %6s %5s %5s %5s %5s %5s %5s %5s %5s %5s %5s %6s\n", mod, g.GetGameData().ClassDefinitions[class].Name, tactical, seed, seed+int64(runs)-1,
		"seed", "dlvl", "clvl", "turns", "kills", "exp", "pots", "home", "equip", "casts", "items", "books", "open", "kite", "shots", "light", "gold")
	for r := int64(0); r < int64(runs); r++ {
		g.SeedRandom(seed + r)
		g.NewUnsavedGame(g.GetGameData().ClassDefinitions[class])
		botRun(g, seed+r, tactical)
	}
}

// botRun plays one hero from depth 1 until it dies or the dungeon ends, printing a line per depth.
func botRun(g *game.GameInstance, seed int64, tactical bool) {
	s, broker := g.GetWorldState(), g.GetActionBroker()
	player := func() *common.Actor { return s.GetPlayer() }
	hp := func() *stats.Resource { return player().GetStats().GetResource(stats.Health) }
	cause := common.AbstractCause("autoplay")
	turns, kills, pots, home, equips, casts, itemCasts, books, opened, kites, shots := 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
	// potion: a healing or mana potion by name (H&H) or by D1's IMISC_ kind (full ones and rejuvenation count too)
	potion := func(it *common.Item, kind string) bool {
		if !it.IsPotion() {
			return false
		}
		if r := it.Roll(); r != nil && strings.HasPrefix(r.MiscID, "IMISC_") {
			heals := r.MiscID == "IMISC_HEAL" || r.MiscID == "IMISC_FULLHEAL"
			mana := r.MiscID == "IMISC_MANA" || r.MiscID == "IMISC_FULLMANA"
			both := r.MiscID == "IMISC_REJUV" || r.MiscID == "IMISC_FULLREJUV"
			return kind == "Healing" && (heals || both) || kind == "Mana" && (mana || both)
		}
		return strings.Contains(it.Name(), kind)
	}
	tick := func() {
		for i := 0; i < 20; i++ {
			broker.Tick()
		}
		if botPace != nil {
			botPace() // watched: hand the frame back to the window until the next turn
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
	// archer: a hero with a bow in hand shoots instead of swinging
	archer := func() bool { return player().GetEquipment().HasRangedWeaponEquipped() }
	// upgrade equips what beats what the hero wears on every value the item tooltip compares, or fills an empty slot
	isUpgrade := func(it *common.Item) bool {
		if slot := it.SlotName(); !it.IsEquippable() || !it.MeetsRequirements() || it.IsLightSource() || slot == defs.SlotNameNotEquippable || slot == defs.SlotNameQuiver || slot >= defs.SlotNameQuickSlot1 { // throwables and belt items are no upgrade
			return false
		}
		if it.IsWeapon() && archer() && (it.Roll() == nil || it.Roll().Type != rules.ITypeBow) {
			return false // an archer keeps a bow in hand
		}
		displaced, cmp := player().GetEquipment().CompareForEquipping(it)
		better, worse := len(displaced) == 0, false
		for _, v := range cmp {
			better, worse = better || v > 0, worse || v < 0
		}
		return better && !worse
	}
	// lightOf is how far a light source shines: its roll's light_radius, as PlayerLightRadius reads it
	lightOf := func(it *common.Item) int {
		if it == nil || it.Roll() == nil {
			return 0
		}
		return it.Roll().PLLight
	}
	// betterLight: a brighter light, or a fresh one when the worn one is burning down
	betterLight := func(it *common.Item) bool {
		cur := player().GetEquipment().GetLightSource()
		return it.IsLightSource() && (cur == nil || lightOf(it) > lightOf(cur) ||
			cur.GetCharges() < 500 && lightOf(it) == lightOf(cur) && it.GetCharges() > cur.GetCharges())
	}
	failed := map[*common.Item]bool{} // spell items whose cast was refunded (the spell couldn't go off)
	broke := false
	lightLow := func() bool {
		l := player().GetEquipment().GetLightSource()
		return s.CarriedLighting() && !s.IsPlayerInTown() && (l == nil || l.GetCharges() >= 0 && l.GetCharges() < 300)
	}
	isBook := func(it *common.Item) bool { return it.Roll() != nil && it.Roll().MiscID == "IMISC_BOOK" }
	// offensive is a spell item worth aiming at a monster: D1's scrolls and staves with a targeted spell
	offensive := func(it *common.Item) bool {
		t := it.GetTargetedAction()
		if t == nil || !it.HasChargesLeft() || !it.MeetsRequirements() || failed[it] {
			return false
		}
		return !harmless(t.Name())
	}
	// keep is what the pack holds on to: healing and mana potions, spell items it can aim, a spare light it'd wear
	keep := func(it *common.Item) bool {
		return potion(it, "Healing") || potion(it, "Mana") || offensive(it) || betterLight(it) || isUpgrade(it)
	}
	upgrade := func() {
		for _, it := range player().GetInventory().Items() {
			if r := it.Roll(); r != nil && strings.HasPrefix(r.MiscID, "IMISC_ELIX") || r != nil && r.MiscID == "IMISC_SPECELIX" {
				actions.ActivateItemActionForPlayer(it) // an elixir: a permanent stat point, drunk at once
				tick()
				pots++
				continue
			}
			if isBook(it) && it.MeetsRequirements() { // read: learns its spell or raises it
				actions.ActivateItemActionForPlayer(it)
				tick()
				books++
				continue
			}
			if betterLight(it) {
				actions.EquipItem(it)
				tick()
				equips++
				continue
			}
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
	// caster: a hero that has cast a spell from its mana (mana potions are worth its gold)
	caster := func() bool { return casts > 0 }
	mana := func() float64 {
		m := player().GetStats().GetResource(stats.Mana)
		return float64(m.Value()) / float64(max(m.GetMax(), 1))
	}
	// canCast: mana for a spell, a charged spell item, or a mana potion to get there
	canCast := func() bool {
		for _, e := range common.SkillBar(player()) {
			if t, ok := e.(*common.TargetedAction); ok && !s.LacksMana(player(), t.Name()) {
				return true
			}
		}
		for _, it := range append(player().GetInventory().Items(), player().GetEquipment().AllItems()...) {
			if offensive(it) || potion(it, "Mana") {
				return true
			}
		}
		return false
	}
	// weak: a monster the hero's weapon kills in two average blows, not worth a spell (tactical)
	weak := func(target *common.Actor) bool {
		h, mon := player().Hero(), target.Monster()
		if !tactical || h == nil || mon == nil {
			return false
		}
		lo, hi := h.MeleeDamageRange(player().HeroSheet(), mon.Data.Class)
		return target.GetStats().GetResource(stats.Health).Value() <= lo+hi
	}
	// worthIt: a tactical bot casts nothing its target resists or is immune to (it swings instead)
	worthIt := func(target *common.Actor, spell int) bool {
		resisted, immune := s.SpellResisted(target, spell)
		return !tactical || !resisted && !immune
	}
	// cast fires the first skill with the mana for it that can reach target; false when none can
	var castSpell, castItem func(target *common.Actor) bool
	// cast picks what to throw at target: nothing at a weak one (the blade saves the mana), spells while the mana
	// lasts, and below 40% mana (tactical) the staves' charges and scrolls first
	cast := func(target *common.Actor) bool {
		if weak(target) && geometry.DistanceChebyshev(target.MapPosition(), player().MapPosition()) <= 1 {
			return false
		}
		if tactical && mana() < 0.4 {
			return castItem(target) || castSpell(target)
		}
		return castSpell(target) || castItem(target)
	}
	castSpell = func(target *common.Actor) bool {
		for _, e := range common.SkillBar(player()) {
			t, ok := e.(*common.TargetedAction)
			if !ok || harmless(t.Name()) || s.LacksMana(player(), t.Name()) || !worthIt(target, s.SpellByName(t.Name())) {
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
	castItem = func(target *common.Actor) bool {
		for _, it := range append(player().GetInventory().Items(), player().GetEquipment().AllItems()...) {
			if !offensive(it) || !worthIt(target, it.Roll().Spell) {
				continue
			}
			t := it.GetTargetedAction()
			if !slices.Contains(t.GetTargetingRules(player(), s.GetMap()).Targets(), target.MapPosition()) {
				continue
			}
			if it.Roll() == nil || it.Roll().Type != rules.ITypeStaff { // a scroll is used up (ActivateItemActionForPlayer)
				broker.Emit(common.ItemUsed{Item: it, User: player()})
			}
			it.UseCharge() // as the targeting UI does (beginActionTargeting)
			if !it.HasChargesLeft() && it.SelfDestructs() {
				player().GetInventory().Remove(it)
			}
			charges := it.GetCharges()
			t.OnSelected(player(), target.MapPosition(), t.GetHitPositions(player().MapPosition(), target.MapPosition()))
			if it.GetCharges() > charges {
				failed[it] = true
				continue
			}
			tick()
			turns++
			itemCasts++
			return true
		}
		return false
	}
	// goHome is a trip to town: healed, then the healing potions and a fresh light the gold buys
	// goHome is a trip to town; back to the same spot (a town portal), or with toStairs down the level's stairs
	var goHomeVia func(toStairs bool)
	goHome := func() { goHomeVia(false) }
	goHomeVia = func(toStairs bool) {
		level, at := s.GetMap().GetName(), player().MapPosition()                                // a town portal: back to this spot afterwards
		s.MapTransition(gridmap.Transition{TargetMap: s.DeepestTown(), TargetLocation: "spawn"}) // the shops restock
		hp().SetToMax()                                                                          // ponytail: the portal and the walk to the healer are free
		home++
		for _, v := range s.GetMap().Actors() {
			if !v.IsVendor() {
				continue
			}
			for _, stack := range common.SellableStacks(v, player()) { // everything it doesn't wear or use
				for _, it := range stack {
					if !keep(it) {
						common.SellItem(player(), it)
					}
				}
			}
		}
		reserve := 0 // under carried light: the price of the cheapest light in town
		for _, v := range s.GetMap().Actors() {
			for _, it := range v.GetInventory().Items() {
				if s.CarriedLighting() && v.IsVendor() && it.IsLightSource() && (reserve == 0 || it.BuyPrice() < reserve) {
					reserve = it.BuyPrice()
				}
			}
		}
		// identify magic gear first (D1's Cain): only then can it be judged against what the hero wears
		for _, v := range s.GetMap().Actors() {
			for _, rec := range s.Factory().Gen().D.VendorsOf(v.GetInternalName()) {
				for _, sv := range rec.Services {
					var fee int
					if _, err := fmt.Sscanf(sv, "identify(%d)", &fee); err != nil {
						continue
					}
					for _, it := range player().GetInventory().Items() {
						r := it.Roll()
						if r == nil || r.Magical == 0 || r.Identified || !it.IsEquippable() || player().GetStats().GetAttribute(stats.Gold)-reserve < fee {
							continue
						}
						player().GetStats().DecrementBy(stats.Gold, fee)
						r.Identified = true
						broker.Emit(common.ItemIdentified{Item: r})
						it.Refresh()
					}
				}
			}
		}
		upgrade()
		d := s.Factory().Gen().D // the witch recharges staves (a caster's second mana pool)
		for _, it := range append(player().GetInventory().Items(), player().GetEquipment().AllItems()...) {
			gold := player().GetStats().GetAttribute(stats.Gold) - reserve
			if r := it.Roll(); r != nil && d.CanRecharge(r) && d.RechargeCost(r) <= gold {
				player().GetStats().DecrementBy(stats.Gold, d.RechargeCost(r))
				r.Charges = r.MaxCharges
				it.Refresh()
				failed[it] = false
			}
		}
		for _, it := range player().GetInventory().Items() { // what no shop buys and the bot won't use: left in town
			if !keep(it) {
				player().GetInventory().Remove(it)
			}
		}
		for _, v := range s.GetMap().Actors() {
			if !v.IsVendor() {
				continue
			}
			stock := slices.Clone(v.GetInventory().Items())
			rank := func(it *common.Item) int { // the light first (the dark can't wait), then potions, then gear
				switch {
				case betterLight(it):
					return 0
				case caster() && potion(it, "Mana"):
					return 1
				case potion(it, "Healing"):
					return 2
				}
				return 3
			}
			slices.SortStableFunc(stock, func(a, b *common.Item) int { return rank(a) - rank(b) })
			for _, it := range stock {
				gold := player().GetStats().GetAttribute(stats.Gold)
				if !betterLight(it) {
					gold -= reserve // the next light is paid for before anything else
				}
				gear := (isUpgrade(it) || isBook(it) && it.MeetsRequirements()) && it.BuyPrice() <= gold/2 // keep potion money
				if (betterLight(it) || gear || potion(it, "Healing") || caster() && potion(it, "Mana")) && it.BuyPrice() <= gold {
					common.BuyItem(player(), v, it)
					upgrade()
				}
			}
		}
		s.MapTransition(gridmap.Transition{TargetMap: level, TargetLocation: "ladder_up"})
		if !toStairs {
			s.GetMap().MoveActor(player(), at)
			broker.Execute(common.NewUpdateFoVAction(cause, player()))
		}
	}
	act := func(a common.Action) {
		broker.Execute(a)
		broker.Execute(common.NewEndPlayerTurnAction(cause, 10))
		tick()
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
		clvl, e0, t0, k0, p0, h0, q0, c0, i0, b0, o0, x0, y0 := player().Hero().Level, player().Hero().Exp, turns, kills, pots, home, equips, casts, itemCasts, books, opened, kites, shots
		waited := map[*common.Actor]int{}   // turns waited for a monster to step in
		kited := map[*common.Actor]int{}    // steps backed away from a monster: at most 8, then it's fought
		chased := map[*common.Actor]int{}   // steps taken toward a monster
		fled := 0                           // danger bailouts on this level
		skip := map[*common.Item]bool{}     // items the bot gave up on
		skipObj := map[common.Object]bool{} // containers opened or out of reach
		goal := player().MapPosition()      // the explore target, kept until reached: re-picking every step oscillates
		hasUnexplored := func(p geometry.Point) bool {
			m := s.GetMap()
			return len(m.GetFilteredNeighbors(p, func(n geometry.Point) bool { return m.IsTileWalkable(n) && !m.IsExplored(n) })) > 0
		}
		budget := 6000
		for ; budget > 0 && player().IsAlive(); budget-- {
			enemies := slices.DeleteFunc(s.GetVisibility().GetVisibleEnemies(), func(e *common.Actor) bool { return chased[e] > 60 })
			if tactical && len(enemies) > 0 && danger(enemies, player().MapPosition()) >= hp().Value() && !drink("Healing") {
				// the Borg's rule: when the next turn could kill, leave; back by the stairs, not into the same fight
				if fled++; fled > 10 { // this level beats the hero: the run ends here, not with a dive past it
					fmt.Printf("%5d gave up on depth %d at character level %d\n", seed, dlvl, player().Hero().Level)
					return
				}
				goHomeVia(true)
				continue
			}
			if fraction() < 0.5 && !drink("Healing") && (len(enemies) > 0 || fraction() < 0.3) || lightLow() && !broke {
				goHome()
				broke = lightLow() // no gold for a light: burn this one down, go home again when hurt
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
				if tactical && (caster() && !weak(target) && canCast() || archer()) && reachers(enemies, player().MapPosition()) > 0 && kited[target] < 8 {
					kited[target]++
					// kite: a caster falls back along a path to a corridor farther from them, where they come single
					// file, and casts from there; with no such corridor near, one step out of reach
					if step, ok := retreat(s.GetMap(), player().MapPosition(), enemies); ok {
						mv := common.NewMoveAction(cause, player(), step, true)
						mv.Step = true
						act(mv)
						kites++
						continue
					}
					m, best, bestD := s.GetMap(), player().MapPosition(), 0
					for _, n := range m.GetFilteredNeighborsForMovement(player().MapPosition(), m.CanPlaceActorHere) {
						if reachers(enemies, n) > 0 {
							continue
						}
						d := 99
						for _, e := range enemies {
							d = min(d, geometry.DistanceChebyshev(e.MapPosition(), n))
						}
						if d > bestD {
							best, bestD = n, d
						}
					}
					if best != player().MapPosition() {
						mv := common.NewMoveAction(cause, player(), best, true)
						mv.Step = true
						act(mv)
						kites++
						continue
					}
				}
				if tactical {
					// the Borg's "back away": when half the hero's health is at stake here, step to the neighbor
					// where the least damage can reach (out of a crowd, into a corridor or doorway)
					here := danger(enemies, player().MapPosition())
					if here*2 >= hp().Value() {
						best, bestN := player().MapPosition(), here
						for _, n := range s.GetMap().GetFilteredNeighborsForMovement(player().MapPosition(), s.GetMap().CanPlaceActorHere) {
							if c := danger(enemies, n); c < bestN {
								best, bestN = n, c
							}
						}
						if best != player().MapPosition() {
							mv := common.NewMoveAction(cause, player(), best, true)
							mv.Step = true
							act(mv)
							continue
						}
					}
					// the weakest one in reach first: one dead monster hits back less than two hurt ones
					for _, e := range enemies {
						if geometry.DistanceChebyshev(e.MapPosition(), player().MapPosition()) <= 1 && (geometry.DistanceChebyshev(target.MapPosition(), player().MapPosition()) > 1 ||
							e.GetStats().GetResource(stats.Health).Value() < target.GetStats().GetResource(stats.Health).Value()) {
							target = e
						}
					}
				}
				if alive := target.IsAlive(); cast(target) || caster() && !weak(target) && drink("Mana") && cast(target) {
					if alive && !target.IsAlive() {
						kills++
					}
					continue
				}
				if archer() && clearLine(s.GetMap(), player().MapPosition(), target.MapPosition()) {
					alive := target.IsAlive() // an archer shoots, point blank too
					act(common.NewRangedAttackAction(common.NewActorCause("Ranged attack", player()), player(), target.MapPosition()))
					if alive && !target.IsAlive() {
						kills++
					}
					shots++
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
				if tactical && geometry.DistanceChebyshev(target.MapPosition(), player().MapPosition()) == 2 && !shooter(target) && waited[target] < 2 {
					waited[target]++ // let it step in: the hero swings first instead of walking into its blow
					act(common.NewEndPlayerTurnAction(cause, 0))
					continue
				}
				if chased[target]++; stepToward(target.MapPosition()) {
					continue // 60 steps after one monster without reaching it (a shooter across a gap): it's left be
				}
				act(common.NewEndPlayerTurnAction(cause, 0)) // an archer out of reach: wait for it
				continue
			}
			m := s.GetMap()
			// containers it has seen get opened: chests, sarcophagi, bookcases...
			if p := m.GetNearestExploredObject(player().MapPosition(), func(o common.Object) bool {
				c, ok := o.(*common.Prop)
				return ok && c.IsUnlootedContainer() && m.IsExplored(c.MapPosition()) && !skipObj[o]
			}); p != player().MapPosition() {
				o, _ := m.TryGetObjectAt(p)
				if geometry.DistanceChebyshev(p, player().MapPosition()) <= 1 {
					o.ActorBumpedInto(player())
					broker.Execute(common.NewEndPlayerTurnAction(cause, 10))
					tick()
					turns++
					opened++
					skipObj[o] = true // opened, or it won't open
				} else if !stepToward(p) {
					skipObj[o] = true
				}
				continue
			}
			// a full pack goes home to sell, then comes back for the rest
			if player().GetInventory().IsFull() && m.GetNearestExploredItem(player().MapPosition(), func(it *common.Item) bool {
				return !skip[it] && m.IsExplored(it.MapPosition()) && !player().CanPickup(it)
			}) != player().MapPosition() {
				goHome()
				if player().GetInventory().IsFull() { // nothing to sell: leave what doesn't fit
					for _, it := range m.Items() {
						skip[it] = skip[it] || !player().CanPickup(it)
					}
				}
				continue
			}
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
					goHome() // explored all it can reach: sell the loot and stock up before the next depth
					break
				}
			}
			if !stepToward(goal) { // out of the bot's reach (behind an object): give its frontier up
				for _, n := range m.GetFilteredNeighbors(goal, func(geometry.Point) bool { return true }) {
					m.SetExplored(n)
				}
				goal = player().MapPosition()
			}
		}
		if budget == 0 && player().IsAlive() { // it couldn't finish the level: that ends the run, never a dive past it
			fmt.Printf("%5d gave up on depth %d at character level %d (out of steps)\n", seed, dlvl, player().Hero().Level)
			return
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
		fmt.Printf("%5d %4d %4d %6d %5d %6d %5d %5d %5d %5d %5d %5d %5d %5d %5d %5d %6d\n", seed, dlvl, clvl, turns-t0, kills-k0, player().Hero().Exp-e0, pots-p0, home-h0, equips-q0, casts-c0, itemCasts-i0, books-b0, opened-o0, kites-x0, shots-y0, s.PlayerLightRadius(), player().GetStats().GetAttribute(stats.Gold))
		if !player().IsAlive() {
			fmt.Printf("%5d died on depth %d at character level %d\n", seed, dlvl, player().Hero().Level)
			return
		}
	}
}

// retreat is the first step toward the nearest corridor cell (two or fewer open cardinal neighbors) within 10
// steps that no enemy stands next to and that is farther from the nearest enemy than from; false when there is none.
func retreat(m *gridmap.GridMap[*common.Actor, *common.Item, common.Object], from geometry.Point, enemies []*common.Actor) (geometry.Point, bool) {
	nearest := func(p geometry.Point) int {
		d := 99
		for _, e := range enemies {
			d = min(d, geometry.DistanceChebyshev(e.MapPosition(), p))
		}
		return d
	}
	corridor := func(p geometry.Point) bool {
		return len(m.GetFilteredNeighbors(p, func(n geometry.Point) bool {
			return (n.X == p.X || n.Y == p.Y) && m.IsTileWalkable(n)
		})) <= 2
	}
	first := map[geometry.Point]geometry.Point{} // each cell reached: the first step on the way there
	queue := []geometry.Point{from}
	dist := map[geometry.Point]int{from: 0}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if p != from && corridor(p) && reachers(enemies, p) == 0 && nearest(p) > nearest(from) {
			return first[p], true
		}
		if dist[p] >= 10 {
			continue
		}
		for _, n := range m.GetFilteredNeighborsForMovement(p, m.CanPlaceActorHere) {
			if _, seen := dist[n]; seen || reachers(enemies, n) > 0 && dist[p] == 0 {
				continue // the first step must leave their reach, or it takes a free blow
			}
			dist[n] = dist[p] + 1
			if p == from {
				first[n] = n
			} else {
				first[n] = first[p]
			}
			queue = append(queue, n)
		}
	}
	return geometry.Point{}, false
}

// danger is the Borg's danger of standing at p, cut down: the most every monster within two steps (one to close
// in, one to strike) can deal in a blow.
func danger(enemies []*common.Actor, p geometry.Point) (sum int) {
	for _, e := range enemies {
		if mon := e.Monster(); mon != nil && geometry.DistanceChebyshev(e.MapPosition(), p) <= 2 {
			sum += mon.Data.Damage.Max()
		}
	}
	return sum
}

// shooter: a monster that attacks from afar (D1's archers and casters); waiting for it to come is a free shot.
func shooter(a *common.Actor) bool {
	ai := a.Monster().Data.AI
	for _, r := range []string{"BOW", "MAGE", "COUNSLR", "SUCC", "LAZ", "ACID", "MEGA", "SNOT"} {
		if strings.Contains(ai, r) {
			return true
		}
	}
	return false
}

// harmless: a spell or skill that doesn't hurt what it's aimed at (a class skill like Trap Disarm among them)
func harmless(name string) bool {
	for _, h := range []string{"Heal", "Teleport", "Telekinesis", "Identify", "Infravision", "Recharge", "Repair", "Disarm", "Resurrect", "Portal", "Phasing"} {
		if strings.Contains(name, h) {
			return true
		}
	}
	return false
}

// reachers counts the enemies standing next to p.
func reachers(enemies []*common.Actor, p geometry.Point) (n int) {
	for _, e := range enemies {
		if geometry.DistanceChebyshev(e.MapPosition(), p) <= 1 {
			n++
		}
	}
	return n
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// botPace, when set, pauses the bot after every turn until the window lets it take the next (watchBot).
var botPace func()

// watchHost is the game's window with the bot at the controls: every speed frames it lets the bot take one turn
// before the frame is updated and drawn. The bot runs in its own goroutine but only while Update waits for it,
// so it never touches the game while a frame is drawn.
type watchHost struct {
	*gui.UIController
	step          func()
	speed, frames int
}

func (h *watchHost) Update() error {
	if h.frames++; h.frames >= h.speed {
		h.frames = 0
		h.step()
	}
	return h.UIController.Update()
}

// watchBot opens the game's window and lets the bot play in it (AUTOPLAY_WATCH=1; AUTOPLAY_SPEED frames per turn,
// default 3, at 60 frames a second).
func watchBot(g *game.GameInstance, ctx common.UIContext, play func()) {
	ui := newEbitenUI(g, ctx)
	speed, _ := strconv.Atoi(os.Getenv("AUTOPLAY_SPEED"))
	resume, yield, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	botPace = func() { yield <- struct{}{}; <-resume }
	go func() {
		<-resume
		play()
		close(done)
	}()
	finished := false
	host := &watchHost{UIController: ui, speed: cmp.Or(speed, 3), step: func() {
		if finished {
			return
		}
		resume <- struct{}{}
		select {
		case <-yield:
		case <-done:
			finished = true
		}
	}}
	size := ui.GetDeviceIndependentScreenSize()
	ebiten.SetWindowTitle("Of Heaven and Hell - autoplay")
	ebiten.SetWindowSize(size.X, size.Y)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(host); err != nil && !errors.Is(err, ebiten.Termination) {
		log.Fatal(err)
	}
}
