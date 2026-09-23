package app

import (
	"slices"
	"testing"

	"rpg/src/common"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/rules"
	"rpg/src/stats"
)

// H&H's on-hit suffixes are item powers like any other. Each must roll, and the two with their own code (thorns
// back at the attacker, mana for a kill) are played here: a power nothing fires is the silent failure this guards.
func TestHeavenAndHellOnHitPowersRollAndFire(t *testing.T) {
	g := newHeavenAndHellGame(t, NewDummyUI())
	g.SeedRandom(1) // the rolls and the blows below are chance: pinned, so a failure is one to replay
	common.SeedRandom(1)
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	f, s := g.GetFactory(), g.GetWorldState()
	player := s.GetPlayer()

	powers := []string{"return_damage", "mana_on_kill", "triple_damage", "fireball_shot"}
	for _, power := range powers {
		rolls := false
		for _, a := range f.Generator.D.Suffixes {
			rolls = rolls || a.Power == power && a.IType != 0
		}
		if !rolls {
			t.Errorf("H&H's xl_isuf.txt has no suffix with %s: it never rolls", power)
		}
	}
	// the first base item of each kind, given the suffix
	base := func(types ...int) *rules.Item {
		for i := range f.Generator.D.Items {
			if b := &f.Generator.D.Items[i]; slices.Contains(types, b.Type) {
				return f.Generator.PlainItem(b)
			}
		}
		t.Fatalf("H&H has no base item of types %v", types)
		return nil
	}
	found := map[string]*rules.Item{
		"return_damage": base(rules.ITypeMArmor, rules.ITypeLArmor, rules.ITypeHArmor),
		"mana_on_kill":  base(rules.ITypeHelm),
		"triple_damage": base(rules.ITypeSword, rules.ITypeAxe, rules.ITypeMace),
		"fireball_shot": base(rules.ITypeBow),
	}
	for power, it := range found {
		for _, a := range f.Generator.D.Suffixes {
			if a.Power == power {
				f.Generator.ApplySuffix(it, a)
			}
		}
		if props := it.PropertyList(); len(props) == 0 {
			t.Errorf("%s with %s shows no power", it.Name, power)
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
		item := f.NewRolledItem(found[power])
		player.GetInventory().Add(item)
		g.GetActionBroker().Execute(common.NewEquipItemAction(common.NewActorCause("test", player), player, item))
		if !slices.Contains(player.GetEquipment().AllItems(), item) {
			t.Fatalf("could not equip %s", item.Name())
		}
		return item
	}
	cause := common.NewActorCause("test", player)
	health := func(a *common.Actor) int { return a.GetStats().GetResource(stats.Health).Value() }

	wear("return_damage")
	monster.GetStats().GetResource(stats.Health).SetToMax()
	before := health(monster)
	for i := 0; i < 50 && health(monster) == before; i++ {
		player.GetStats().GetResource(stats.Health).SetToMax()
		g.GetActionBroker().Execute(common.NewMeleeAttackAction(common.NewActorCause("test", monster), monster, player))
	}
	if health(monster) == before {
		t.Error("thorns: fifty blows at the hero and the attacker never took damage back")
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
	wear("fireball_shot")
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

	wear("mana_on_kill")
	mana := player.GetStats().GetResource(stats.Mana)
	mana.SetToMin()
	g.GetActionBroker().Execute(common.NewDieAction(cause, monster))
	if mana.Value() == mana.Min() {
		t.Error("the archmage: a kill restored no mana")
	}
}
