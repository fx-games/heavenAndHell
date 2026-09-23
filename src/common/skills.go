package common

import (
	"fmt"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/rules"
	"rpg/src/stats"
	"rpg/src/util"
)

// The class skills (_pAblSpells): the Warrior's Item Repair, the Rogue's Trap Disarm, the Sorcerer's Staff
// Recharge and, from Hellfire, the Monk's Vision, the Bard's Identify and the Barbarian's Rage. They cost no
// mana; like D1's cursor, repair, recharge and identify ask for an item and disarm for an object.
func (f *Factory) classSkill(player *Actor) (*TargetedAction, *UntargetedAction) {
	skill := f.Generator.D.Class(player.hero.Class).Skill
	if spell, ok := rules.ClassSkills[skill]; !ok || f.Generator.D.Spells[spell].ID == "" {
		return nil, nil // classes.rec names no skill, or the mod has no such spell: a class skill is content
	}
	switch skill {
	case "vision":
		return nil, f.castSkill("Vision", rules.SplSearch)
	case "rage":
		return nil, f.castSkill("Rage", rules.SplRage)
	case "identify":
		action := NewUntargetedAction("Identify", f.skillIcon(rules.SplIdentify))
		action.SetEffect(&UntargetedEffect{Effect: func(p *Actor, _ EffectSource, _ bool) {
			f.pickIdentifyTarget(p, func() {
				f.castSpell(p, rules.SplIdentify, p.hero.SpellLevel(p.HeroSheet(), rules.SplIdentify)+1, p.MapPosition())
			})
		}, Description: []util.LabelText{util.LabelTextFromString(f.Generator.D.Spells[rules.SplIdentify].Description)}})
		return nil, action
	case "repair":
		return nil, f.itemSkill("Item Repair", rules.SplRepair, rules.CanRepair, func(it *Item) {
			f.PlaySfx("IS_REPAIR", player.MapPosition()) // DoRepair
			if rules.RepairItem(it.roll, player.hero.Level, rng.Intn) {
				name := it.Name()
				player.GetEquipment().UnEquip(it)
				player.GetInventory().Remove(it)
				f.Msg(fmt.Sprintf("Your %s is worn away.", name))
				return
			}
			f.Msg(fmt.Sprintf("You repair your %s.", it.Name()))
		})
	case "recharge":
		return nil, f.itemSkill("Staff Recharge", rules.SplRecharge, func(it *rules.Item) bool { return it.Type == rules.ITypeStaff && f.Generator.D.CanRecharge(it) }, func(it *Item) {
			f.Generator.D.RechargeItem(it.roll, player.hero.Level, rng.Intn)
			f.Msg(fmt.Sprintf("You recharge your %s.", it.Name()))
		})
	}
	action := NewTargetedAction("Trap Disarm", OneTargetTile(f.State.GetMap, util.Arguments{}),
		&HitEffect{Activate: func(caster *Actor, target geometry.Point, _ []geometry.Point) { f.Disarm(caster, target) },
			Description: []util.LabelText{util.LabelTextFromString(f.Generator.D.Spells[rules.SplDisarm].Description)}},
		spellTargeting(f, rules.SplDisarm))
	action.SetIcon(f.skillIcon(rules.SplDisarm))
	return action, nil
}

// castSkill is a class skill that just casts its spell on the hero, for free, at skill level 1 plus gear.
func (f *Factory) castSkill(name string, spell int) *UntargetedAction {
	action := NewUntargetedAction(name, f.skillIcon(spell))
	action.SetEffect(&UntargetedEffect{Effect: func(player *Actor, _ EffectSource, _ bool) {
		f.castSpell(player, spell, player.hero.SpellLevel(player.HeroSheet(), spell)+1, player.MapPosition())
	}, Description: []util.LabelText{util.LabelTextFromString(f.Generator.D.Spells[spell].Description)}})
	return action
}

// rageEffect ports ProcessRage: Hero.Rage counts the turns of fury down, then the same number of hangover
// turns back up, and the Barbarian pays 6 HP per level when it finally lets go (never enough to kill him).
func (f *Factory) rageEffect(clvl int) *ParametrizedStatusEffect {
	turns := rules.RageTurns(clvl)
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
			dam := min(rules.RageDamage(clvl), a.GetStats().GetResource(stats.Health).Value()-1)
			f.ActionBroker.Execute(NewChangeResourceAction(AbstractCause("rage"), a, stats.Health, float64(-dam)))
		}
		SyncHero(a)
	}, 2*turns)
}

// apply_rage_self ports AddRage: no raging while already raging, or while too wounded to survive the hangover.
// The status below does the rest; this is only the hero bookkeeping that starts it, reached by name.
func (f *Factory) getRageSelfAction(args util.Arguments) UntargetedEffect {
	return UntargetedEffect{
		Effect: func(user *Actor, source EffectSource, forced bool) {
			h := user.hero
			if h == nil {
				return
			}
			if h.Rage != 0 || user.GetStats().GetResource(stats.Health).Value() <= h.Level {
				f.Msg("You cannot work yourself up again.")
				return
			}
			h.Rage = rules.RageTurns(h.Level)
			user.AddParametrizedStatusEffect(f.rageEffect(h.Level))
			SyncHero(user)
		},
		Description: []util.LabelText{util.LabelTextFromString("Work yourself into a rage")},
	}
}

// skillIcon is the skills.rec icon of a spell or class skill.
func (f *Factory) skillIcon(spell int) int32 {
	return defs.Icon(defs.IconID("spell", f.Generator.D.Spells[spell].ID))
}

// itemSkill lists the pack's items the skill works on and applies it to the one picked, which takes a turn.
func (f *Factory) itemSkill(name string, spell int, applies func(*rules.Item) bool, perform func(*Item)) *UntargetedAction {
	action := NewUntargetedAction(name, f.skillIcon(spell))
	action.SetEffect(&UntargetedEffect{Effect: func(player *Actor, _ EffectSource, _ bool) {
		var items []*Item
		for _, it := range heroItems(player) {
			if it.roll != nil && applies(it.roll) {
				items = append(items, it)
			}
		}
		if len(items) == 0 {
			f.Msg("You have nothing for that.")
			return
		}
		openItemList(f.Show, player, items, func(item *Item) {
			perform(item)
			item.Refresh()
			SyncHero(player)
			f.ActionBroker.Execute(NewEndPlayerTurnAction(AbstractCause(ActionName(name)), 10))
		})
	}, Description: []util.LabelText{util.LabelTextFromString(f.Generator.D.Spells[spell].Description)}})
	return action
}

// openItemList lets the player pick one of items in the inventory list's look and hotkeys; suffix may be nil.
func openItemList(show func(UIRequest), player *Actor, items []*Item, pick func(*Item)) {
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
			show(CloseMenus{})
			pick(item)
		})
		list = append(list, entry)
	}
	show(ChooseFromList{list})
}

// heroItems are the pack's items and the equipped ones that aren't in it.
func heroItems(player *Actor) []*Item {
	inv := player.GetInventory()
	all := append([]*Item(nil), inv.Items()...)
	for _, it := range player.GetEquipment().AllItems() {
		if !inv.Has(it) {
			all = append(all, it)
		}
	}
	return all
}

// Disarm ports ACTION_DISARM on the object next to the hero: TryDisarm's Dexterity roll against the depth clears
// its trap flag and the wall traps aimed at it, then the object is operated anyway (a failed roll sets the trap off).
func (f *Factory) Disarm(player *Actor, target geometry.Point) {
	m := f.State.GetMap()
	o := objectAt(m, target)
	door, isDoor := doorAt(m, target)
	if isDoor && door.Prop == nil {
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
		f.hitObject(m, target, player)
		return
	}
	obj, name := door.Prop, "door"
	if o != nil {
		obj, name = o.O, o.Name()
	}
	if obj.TrapFlag && rng.Intn(100) <= rules.DisarmChance(player.HeroSheet().Dex, f.State.CurrentLevel().Depth) {
		PlayUICue("disarm")
		f.State.mapGen.disarmObject(m, obj)
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
		f.operateObject(o, player)
	} else {
		door.Open()
		f.ActionBroker.Execute(NewUpdateFoVAction(AbstractCause("Trap Disarm"), player))
	}
}

// disarmObject ports TryDisarm's success: the wall traps pointing at the object stop (their _oVar4 set to 1).
func (g *MapGenerator) disarmObject(m interface{ GetName() string }, obj *rules.Object) {
	obj.TrapFlag = false
	lvl := g.generated[m.GetName()]
	if lvl == nil {
		return
	}
	for _, t := range lvl.Objects {
		if (t.Kind == rules.ObjTRAPL || t.Kind == rules.ObjTRAPR) && t.Var1 == obj.X && t.Var2 == obj.Y {
			t.Var4 = 1
		}
	}
}

// oilAction ports DoOil: an oil asks for an item the way Item Repair does, and is used up only on one it can
// actually improve.
func (f *Factory) oilAction(item *Item, oil, name string) {
	item.SetCategory(defs.ItemCategoryPotion)
	item.SetCharges(1)
	action := NewUntargetedAction(name, item.icon)
	action.SetEffect(&UntargetedEffect{Effect: func(player *Actor, _ EffectSource, _ bool) {
		var targets []*Item
		for _, it := range heroItems(player) {
			if it != item && it.roll != nil && rules.OilTarget(oil, it.roll) {
				targets = append(targets, it)
			}
		}
		if len(targets) == 0 {
			f.Msg("You have nothing to use that on.")
			return
		}
		openItemList(f.Show, player, targets, func(target *Item) {
			if !rules.ApplyOil(oil, target.roll, rng.Intn) {
				f.Msg(fmt.Sprintf("The %s does nothing for your %s.", name, target.Name()))
				return
			}
			target.Refresh()
			player.GetInventory().Remove(item)
			SyncHero(player)
			f.Msg(fmt.Sprintf("You treat your %s with %s.", target.Name(), name))
			f.ActionBroker.Execute(NewEndPlayerTurnAction(AbstractCause(ActionName(name)), 10))
		})
	}, Description: []util.LabelText{util.LabelTextFromString(name)}})
	item.SetUntargetedAction(action)
}
