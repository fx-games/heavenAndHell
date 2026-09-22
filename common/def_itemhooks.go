package common

import (
	"math/rand"

	"rpg/d1"

	"rpg/stats"
	"rpg/util"
)

// ItemHookPowers are the affix powers that carry an on-hit hook instead of a stat: a mod's affix table names one
// in PLPower (H&H's xl_isuf.txt), D1's own generator skips it like any power it doesn't know, and NewD1Item hangs
// the hook on the game item. The names are the engine's vocabulary; a mod only picks from them.
var ItemHookPowers = map[string]func() ItemActionHook{
	"HOOK_MANA_ON_KILL":     restoreManaOnKill,
	"HOOK_FIREBALL_ON_SHOT": fireballOnRanged,
	"HOOK_RETURN_DAMAGE":    returnDamageOnHit,
	"HOOK_TRIPLE_DAMAGE":    randomlyTripleDamageOnMelee,
}

// d1ItemHooks are the hooks a rolled item's affixes carry.
func d1ItemHooks(it *d1.Item) []ItemActionHook {
	var hooks []ItemActionHook
	for _, power := range []string{it.PrePower, it.SufPower} {
		if newHook, ok := ItemHookPowers[power]; ok {
			hooks = append(hooks, newHook())
		}
	}
	return hooks
}

func NewHooksForItem(source *rand.Rand, level int, count int) []ItemActionHook {
	hooksFactories := []func() ItemActionHook{
		restoreManaOnKill,
		fireballOnRanged,
		returnDamageOnHit,
		randomlyTripleDamageOnMelee,
	}
	randomIndices := source.Perm(len(hooksFactories))
	if count > len(hooksFactories) {
		count = len(hooksFactories)
	}
	hooks := make([]ItemActionHook, 0)
	for i := 0; i < count; i++ {
		hook := hooksFactories[randomIndices[i]]()
		hooks = append(hooks, hook)
	}
	return hooks
}

func fireballOnRanged() ItemActionHook {
	return NewOnRangedHook("cast fireball", fireBallOnRangedAttack)
}

func restoreManaOnKill() ItemActionHook {
	return NewOnDieHook("restore 4 mana", manaForKills)
}

func returnDamageOnHit() ItemActionHook {
	return NewOnAfterMeleeHook("return damage", returnDamage)
}

func randomlyTripleDamageOnMelee() ItemActionHook {
	return NewOnBeforeMeleeHook("5% to triple damage", chanceToTripleDamage)
}

func chanceToTripleDamage(itemContext *ItemContext, action Action, context ActionContext) []Action {
	meleeAction, isMelee := action.(*MeleeAttackAction)
	if !isMelee {
		return nil
	}
	attacker := meleeAction.GetAttacker()
	if attacker != itemContext.GetActor() {
		return nil
	}
	if rand.Intn(100) < 5 {
		meleeAction.SetDamageFactor(3)
	}
	return nil
}

func returnDamage(itemContext *ItemContext, action Action, context ActionContext) []Action {
	factor := 0.1
	meleeAction, isMelee := action.(*MeleeAttackAction)
	if !isMelee {
		return nil
	}
	mainDefender := meleeAction.GetDefender()
	if mainDefender != itemContext.GetActor() {
		return nil
	}
	damageDealt := meleeAction.GetTotalDamageDealt()
	returnedDamage := max(1, int(float64(damageDealt)*factor))
	return OneAction(NewChangeResourceAction(action.Cause(), meleeAction.GetAttacker(), stats.Health, float64(-returnedDamage)))
}

func fireBallOnRangedAttack(itemContext *ItemContext, action Action, context ActionContext) []Action {
	rangedAction, isRanged := action.(*RangedAttackAction)
	if !isRanged {
		return nil
	}
	attacker := rangedAction.GetAttacker()
	if attacker != itemContext.GetActor() {
		return nil
	}

	targetRules := NewRangedAttackTargetingRules(util.Arguments{})
	rangedTargetRules := targetRules.GetTargetRules(attacker, DijkstraAoE(context.GameState.GetMap, util.Arguments{"3"}), context.GameState.GetMap()).(*RangedTargetingRules)

	targetPos := rangedAction.GetTargetPos()

	cause := NewActorCause(ActionNameRangedAttack, attacker)
	return OneAction(NewDamageSpellAction(cause, rangedAction.GetAttacker(), 8, targetPos, rangedTargetRules.GetHitpositions(targetPos)))
}

// stat change on kill
func manaForKills(itemContext *ItemContext, action Action, context ActionContext) []Action {
	amount := 4
	return OneAction(NewChangeResourceAction(action.Cause(), itemContext.GetActor(), stats.Mana, float64(amount)))
}
