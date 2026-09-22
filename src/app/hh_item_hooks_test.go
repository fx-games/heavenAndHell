package app

import (
	"slices"
	"testing"

	"rpg/src/common"
	"rpg/src/d1"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/stats"
)

// main's enchantments had two halves: stat modifiers and on-hit hooks. H&H's suffix table carries both; a hook
// suffix names one of the engine's ItemHookPowers, and the rolled item wears the hook. Each one is played here,
// not just found: a hook nothing fires is the silent failure this guards against.
func TestHeavenAndHellItemHooksRollAndFire(t *testing.T) {
	g := newHeavenAndHellGame(t, NewDummyUI())
	g.SeedRandom(1) // the rolls and the blows below are chance: pinned, so a failure is one to replay
	common.SeedD1Rng(1)
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	f, s := g.GetFactory(), g.GetWorldState()
	player := s.GetPlayer()

	// the suffix table carries every hook, each on an item type that can roll it
	for power := range common.ItemHookPowers {
		rolls := false
		for _, a := range f.Generator.D.Suffixes {
			rolls = rolls || a.Power == power && a.IType != 0
		}
		if !rolls {
			t.Errorf("H&H's xl_isuf.txt has no suffix with %s: the hook never rolls", power)
		}
	}
	// an item with such a suffix wears its hook: the first base item of each kind, given the suffix
	base := func(types ...int) *d1.Item {
		for idx, it := range f.Generator.D.Items {
			if slices.Contains(types, it.Type) {
				return f.Generator.PlainItem(idx)
			}
		}
		t.Fatalf("H&H has no base item of types %v", types)
		return nil
	}
	found := map[string]*d1.Item{
		"HOOK_RETURN_DAMAGE":    base(d1.ITypeMArmor, d1.ITypeLArmor, d1.ITypeHArmor),
		"HOOK_MANA_ON_KILL":     base(d1.ITypeHelm),
		"HOOK_TRIPLE_DAMAGE":    base(d1.ITypeSword, d1.ITypeAxe, d1.ITypeMace),
		"HOOK_FIREBALL_ON_SHOT": base(d1.ITypeBow),
	}
	for power, it := range found {
		it.SufPower = power
		if item := f.NewD1Item(it); len(item.GetItemHooks()) != 1 {
			t.Fatalf("%s with %s carries %d hooks, want 1", it.Name, power, len(item.GetItemHooks()))
		}
	}

	s.MapTransition(gridmap.Transition{TargetMap: levelAt(s, 1), TargetLocation: "ladder_up"})
	m := s.GetMap()
	var monster *common.Actor
	for _, a := range m.Actors() {
		if a != player && a.IsAlive() && a.IsHostile() {
			monster = a
			break
		}
	}
	if monster == nil {
		t.Fatal("no monster on level 1 to play the hooks on")
	}
	next := func() geometry.Point {
		for _, p := range m.GetFreeCardinalNeighbors(player.MapPosition()) {
			return p
		}
		t.Fatal("no room next to the hero")
		return geometry.Point{}
	}
	m.MoveActor(monster, next())
	wear := func(power string) *common.Item {
		item := f.NewD1Item(found[power])
		player.GetInventory().Add(item)
		g.GetActionBroker().Execute(common.NewEquipItemAction(common.NewActorCause("test", player), player, item))
		if !slices.Contains(player.GetEquipment().AllItems(), item) {
			t.Fatalf("could not equip %s", item.Name())
		}
		return item
	}
	cause := common.NewActorCause("test", player)
	health := func(a *common.Actor) int { return a.GetStats().GetResource(stats.Health).Value() }

	wear("HOOK_RETURN_DAMAGE")
	monster.GetStats().GetResource(stats.Health).SetToMax()
	before := health(monster)
	for i := 0; i < 50 && health(monster) == before; i++ {
		player.GetStats().GetResource(stats.Health).SetToMax()
		g.GetActionBroker().Execute(common.NewMeleeAttackAction(common.NewActorCause("test", monster), monster, player))
	}
	if health(monster) == before {
		t.Error("thorns: fifty blows at the hero and the attacker never took damage back")
	}

	wear("HOOK_TRIPLE_DAMAGE")
	tripled := false
	for i := 0; i < 400 && !tripled; i++ {
		monster.GetStats().GetResource(stats.Health).SetToMax()
		attack := common.NewMeleeAttackAction(cause, player, monster)
		g.GetActionBroker().Execute(attack)
		tripled = attack.GetDamageFactor() == 3
	}
	if !tripled {
		t.Error("fury: 400 blows and not one tripled (5% each)")
	}

	// the fireball's blast reaches a second monster beside the one shot at; the arrow alone can't
	var bystander *common.Actor
	for _, a := range m.Actors() {
		if a != player && a != monster && a.IsAlive() && a.IsHostile() {
			bystander = a
			break
		}
	}
	if bystander == nil {
		t.Fatal("no second monster on level 1")
	}
	far := monster.MapPosition()
	placed := false
	for reach := 1; reach <= 2 && !placed; reach++ { // beside the target first: the blast is centred on it
		for dy := -reach; dy <= reach && !placed; dy++ {
			for dx := -reach; dx <= reach && !placed; dx++ {
				p := far.Add(geometry.Point{X: dx, Y: dy})
				if p != far && max(dx, -dx, dy, -dy) == reach && m.CanPlaceActorHere(p) {
					m.MoveActor(bystander, p)
					placed = true
				}
			}
		}
	}
	if !placed {
		t.Fatalf("no room for a second monster within 2 of %v", far)
	}
	wear("HOOK_FIREBALL_ON_SHOT")
	player.GetStats().SetAttribute(stats.LightRadius, 10) // the blast is aimed at what the hero sees; a torch sees 1
	g.GetActionBroker().Execute(common.NewUpdateFoVAction(cause, player))
	bystander.GetStats().GetResource(stats.Health).SetToMax()
	hurt := health(bystander)
	for i := 0; i < 5 && health(bystander) == hurt; i++ {
		monster.GetStats().GetResource(stats.Health).SetToMax()
		g.GetActionBroker().Execute(common.NewRangedAttackAction(cause, player, far))
	}
	if health(bystander) == hurt {
		t.Error("flame: five shots and the fireball never hit the monster beside the target")
	}

	wear("HOOK_MANA_ON_KILL")
	mana := player.GetStats().GetResource(stats.Mana)
	mana.SetToMin()
	g.GetActionBroker().Execute(common.NewDieAction(cause, monster))
	if mana.Value() == mana.Min() {
		t.Error("the archmage: a kill restored no mana")
	}
}
