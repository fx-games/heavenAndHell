package common

import (
	"fmt"
	"rpg/src/d1"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/stats"
	"rpg/src/util"
)

// The class skills (_pAblSpells): the Warrior's Item Repair, the Rogue's Trap Disarm, the Sorcerer's Staff
// Recharge and, from Hellfire, the Monk's Vision, the Bard's Identify and the Barbarian's Rage. They cost no
// mana; like D1's cursor, repair, recharge and identify ask for an item and disarm for an object.
func (f *Factory) d1ClassSkill(player *Actor) (*TargetedAction, *UntargetedAction) {
	skill := d1.Class(player.hero.Class).Skill
	if spell, ok := d1.ClassSkills[skill]; !ok || f.Generator.D.Spells[spell].ID == "" {
		return nil, nil // hero.rec names no skill, or the mod has no such spell: a class skill is content
	}
	switch skill {
	case "vision":
		return nil, f.d1CastSkill("Vision", d1.SplSearch)
	case "rage":
		return nil, f.d1CastSkill("Rage", d1.SplRage)
	case "identify":
		action := NewUntargetedAction("Identify", f.d1SkillIcon(d1.SplIdentify))
		action.SetEffect(&UntargetedEffect{Effect: func(p *Actor, _ EffectSource) {
			f.pickD1IdentifyTarget(p, func() {
				f.castD1(p, d1.SplIdentify, p.hero.SpellLevel(p.HeroSheet(), d1.SplIdentify)+1, p.MapPosition())
			})
		}, Description: []util.LabelText{util.LabelTextFromString(d1.SpellDescriptions[d1.SplIdentify])}})
		return nil, action
	case "repair":
		return nil, f.d1ItemSkill("Item Repair", d1.SplRepair, d1.CanRepair, func(it *Item) {
			f.PlayD1Sfx("IS_REPAIR", player.MapPosition()) // DoRepair
			if d1.RepairItem(it.d1, player.hero.Level, d1Rng.Intn) {
				name := it.Name()
				player.GetEquipment().UnEquip(it)
				player.GetInventory().Remove(it)
				f.Msg(fmt.Sprintf("Your %s is worn away.", name))
				return
			}
			f.Msg(fmt.Sprintf("You repair your %s.", it.Name()))
		})
	case "recharge":
		return nil, f.d1ItemSkill("Staff Recharge", d1.SplRecharge, func(it *d1.Item) bool { return it.Type == d1.ITypeStaff && f.Generator.D.CanRecharge(it) }, func(it *Item) {
			f.Generator.D.RechargeItem(it.d1, player.hero.Level, d1Rng.Intn)
			f.Msg(fmt.Sprintf("You recharge your %s.", it.Name()))
		})
	}
	action := NewTargetedAction("Trap Disarm", OneTargetTile(f.State.GetMap, util.Arguments{}),
		&HitEffect{Activate: func(caster *Actor, target geometry.Point, _ []geometry.Point) { f.D1Disarm(caster, target) },
			Description: []util.LabelText{util.LabelTextFromString(d1.SpellDescriptions[d1.SplDisarm])}},
		d1SpellTargeting(f, d1.SplDisarm))
	action.SetIcon(f.d1SkillIcon(d1.SplDisarm))
	return action, nil
}

// d1CastSkill is a class skill that just casts its spell on the hero, for free, at skill level 1 plus gear.
func (f *Factory) d1CastSkill(name string, spell int) *UntargetedAction {
	action := NewUntargetedAction(name, f.d1SkillIcon(spell))
	action.SetEffect(&UntargetedEffect{Effect: func(player *Actor, _ EffectSource) {
		f.castD1(player, spell, player.hero.SpellLevel(player.HeroSheet(), spell)+1, player.MapPosition())
	}, Description: []util.LabelText{util.LabelTextFromString(d1.SpellDescriptions[spell])}})
	return action
}

// d1RageEffect ports ProcessRage: Hero.Rage counts the turns of fury down, then the same number of hangover
// turns back up, and the Barbarian pays 6 HP per level when it finally lets go (never enough to kill him).
func (f *Factory) d1RageEffect(clvl int) *ParametrizedStatusEffect {
	turns := d1.RageTurns(clvl)
	return NewParametrizedStatusEffect(StatusRage, func(a *Actor) {
		h := a.hero
		switch {
		case h.Rage > 1:
			h.Rage--
		case h.Rage > 0:
			h.Rage = -turns // the fury is spent, the hangover starts
			f.Msg("Your rage subsides.")
		case h.Rage < -1:
			h.Rage++
		default:
			h.Rage = 0
			dam := min(d1.RageDamage(clvl), a.GetStats().GetResource(stats.Health).Value()-1)
			f.ActionBroker.Execute(NewChangeResourceAction(AbstractCause("rage"), a, stats.Health, float64(-dam)))
		}
		SyncHero(a)
	}, 2*turns)
}

// apply_rage_self ports AddRage: no raging while already raging, or while too wounded to survive the hangover.
// The status below does the rest; this is only the hero bookkeeping that starts it, reached by name.
func (f *Factory) getRageSelfAction(args util.Arguments) UntargetedEffect {
	return UntargetedEffect{
		Effect: func(user *Actor, source EffectSource) {
			h := user.hero
			if h == nil {
				return
			}
			if h.Rage != 0 || user.GetStats().GetResource(stats.Health).Value() <= h.Level {
				f.Msg("You cannot work yourself up again.")
				return
			}
			h.Rage = d1.RageTurns(h.Level)
			user.AddParametrizedStatusEffect(f.d1RageEffect(h.Level))
			SyncHero(user)
		},
		Description: []util.LabelText{util.LabelTextFromString("Work yourself into a rage")},
	}
}

// d1SkillIcon is the skills.rec icon of a spell or class skill.
func (f *Factory) d1SkillIcon(spell int) int32 {
	return defs.Icon(defs.IconID("spell", f.Generator.D.Spells[spell].ID))
}

// d1ItemSkill lists the pack's items the skill works on and applies it to the one picked, which takes a turn.
func (f *Factory) d1ItemSkill(name string, spell int, applies func(*d1.Item) bool, perform func(*Item)) *UntargetedAction {
	action := NewUntargetedAction(name, f.d1SkillIcon(spell))
	action.SetEffect(&UntargetedEffect{Effect: func(player *Actor, _ EffectSource) {
		var items []*Item
		for _, it := range d1HeroItems(player) {
			if it.d1 != nil && applies(it.d1) {
				items = append(items, it)
			}
		}
		if len(items) == 0 {
			f.Msg("You have nothing for that.")
			return
		}
		openD1ItemList(f.UI(), player, items, func(item *Item) {
			perform(item)
			item.RefreshD1()
			SyncHero(player)
			f.ActionBroker.Execute(NewEndPlayerTurnAction(AbstractCause(ActionName(name)), 10))
		})
	}, Description: []util.LabelText{util.LabelTextFromString(d1.SpellDescriptions[spell])}})
	return action
}

// openD1ItemList lets the player pick one of items in the inventory list's look and hotkeys; suffix may be nil.
func openD1ItemList(ui UserInterface, player *Actor, items []*Item, pick func(*Item)) {
	inv := player.GetInventory()
	stacks := make([]InventoryStack, len(items))
	for i, it := range items {
		stacks[i] = InventoryStack{it}
	}
	SortInventory(stacks)
	extra := 'A' // equipped items outside the pack have no inventory letter
	var list []*TextualStack
	for _, stack := range stacks {
		item := stack[0]
		letter := inv.GetLetterForItem(item)
		if letter == 0 {
			letter, extra = extra, extra+1
		}
		entry := NewTextualStack(stack, letter, func(rune) {
			ui.PopAll()
			pick(item)
		})
		list = append(list, entry)
	}
	ui.OpenItemList(list)
}

// d1HeroItems are the pack's items and the equipped ones that aren't in it.
func d1HeroItems(player *Actor) []*Item {
	inv := player.GetInventory()
	all := append([]*Item(nil), inv.Items()...)
	for _, it := range player.GetEquipment().AllItems() {
		if !inv.Has(it) {
			all = append(all, it)
		}
	}
	return all
}

// D1Disarm ports ACTION_DISARM on the object next to the hero: TryDisarm's Dexterity roll against the depth clears
// its trap flag and the wall traps aimed at it, then the object is operated anyway (a failed roll sets the trap off).
func (f *Factory) D1Disarm(player *Actor, target geometry.Point) {
	m := f.State.GetMap()
	o := d1ObjectAt(m, target)
	door, isDoor := d1DoorAt(m, target)
	if isDoor && door.D1 == nil {
		isDoor = false
	}
	if o == nil && !isDoor {
		f.Msg("There is nothing to disarm there.")
		return
	}
	if geometry.DistanceChebyshev(player.MapPosition(), target) > 1 {
		f.Msg("You need to stand next to it.")
		return
	}
	f.ActionBroker.Execute(NewEndPlayerTurnAction(AbstractCause("Trap Disarm"), 10))
	if o != nil && f.Generator.D.Objects[o.O.Kind].Break == 1 { // barrels and crucified skeletons are attacked
		f.hitD1Object(m, target, player)
		return
	}
	obj, name := door.D1, "door"
	if o != nil {
		obj, name = o.O, o.Name()
	}
	if obj.TrapFlag && d1Rng.Intn(100) <= d1.DisarmChance(player.HeroSheet().Dex, f.State.CurrentLevel().Depth) {
		PlayUICue("disarm")
		f.State.mapGen.disarmD1Object(m, obj)
		if o != nil {
			o.Disarmed = true
		} else {
			door.Disarmed = true
		}
		f.Msg(fmt.Sprintf("You disarm the %s.", name))
	} else {
		f.Msg(fmt.Sprintf("You find no trap on the %s you can disarm.", name))
	}
	if o != nil {
		f.operateD1Object(o, player)
	} else {
		door.Open()
		f.ActionBroker.Execute(NewUpdateFoVAction(AbstractCause("Trap Disarm"), player))
	}
}

// disarmD1Object ports TryDisarm's success: the wall traps pointing at the object stop (their _oVar4 set to 1).
func (g *MapGenerator) disarmD1Object(m interface{ GetName() string }, obj *d1.Object) {
	obj.TrapFlag = false
	lvl := g.d1Levels[m.GetName()]
	if lvl == nil {
		return
	}
	for _, t := range lvl.Objects {
		if (t.Kind == d1.ObjTRAPL || t.Kind == d1.ObjTRAPR) && t.Var1 == obj.X && t.Var2 == obj.Y {
			t.Var4 = 1
		}
	}
}

// d1OilAction ports DoOil: an oil asks for an item the way Item Repair does, and is used up only on one it can
// actually improve.
func (f *Factory) d1OilAction(item *Item, oil, name string) {
	item.SetCategory(defs.ItemCategoryPotion)
	item.SetCharges(1)
	action := NewUntargetedAction(name, item.icon)
	action.SetEffect(&UntargetedEffect{Effect: func(player *Actor, _ EffectSource) {
		var targets []*Item
		for _, it := range d1HeroItems(player) {
			if it != item && it.d1 != nil && d1.OilTarget(oil, it.d1) {
				targets = append(targets, it)
			}
		}
		if len(targets) == 0 {
			f.Msg("You have nothing to use that on.")
			return
		}
		openD1ItemList(f.UI(), player, targets, func(target *Item) {
			if !d1.ApplyOil(oil, target.d1, d1Rng.Intn) {
				f.Msg(fmt.Sprintf("The %s does nothing for your %s.", name, target.Name()))
				return
			}
			target.RefreshD1()
			player.GetInventory().Remove(item)
			SyncHero(player)
			f.Msg(fmt.Sprintf("You treat your %s with %s.", target.Name(), name))
			f.ActionBroker.Execute(NewEndPlayerTurnAction(AbstractCause(ActionName(name)), 10))
		})
	}, Description: []util.LabelText{util.LabelTextFromString(name)}})
	item.SetUntargetedAction(action)
}
