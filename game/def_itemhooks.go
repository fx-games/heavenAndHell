package game

import (
	"math/rand"
	"rpg/actions"
	"rpg/stats"
)

func NewHooksForItem(source *rand.Rand, level int, count int) []actions.ItemActionHook {
	hooksFactories := []func() actions.ItemActionHook{
		restoreManaOnKill,
		fireballOnRanged,
		returnDamageOnHit,
		randomlyTripleDamageOnMelee,
	}
	randomIndices := source.Perm(len(hooksFactories))
	if count > len(hooksFactories) {
		count = len(hooksFactories)
	}
	hooks := make([]actions.ItemActionHook, 0)
	for i := 0; i < count; i++ {
		hook := hooksFactories[randomIndices[i]]()
		hooks = append(hooks, hook)
	}
	return hooks
}

func fireballOnRanged() actions.ItemActionHook {
	return actions.NewOnRangedHook("cast fireball", fireBallOnRangedAttack)
}

func restoreManaOnKill() actions.ItemActionHook {
	return actions.NewOnDieHook("restore 4 mana", manaForKills)
}

func returnDamageOnHit() actions.ItemActionHook {
	return actions.NewOnAfterMeleeHook("return damage", returnDamage)
}

func randomlyTripleDamageOnMelee() actions.ItemActionHook {
	return actions.NewOnBeforeMeleeHook("5% to triple damage", chanceToTripleDamage)
}

func chanceToTripleDamage(context *actions.ItemContext, action actions.Action, engine actions.Engine) []actions.Action {
	meleeAction, isMelee := action.(*actions.MeleeAttackAction)
	if !isMelee {
		return nil
	}
	attacker := meleeAction.GetAttacker()
	if attacker != context.GetActor() {
		return nil
	}
	if rand.Intn(100) < 5 {
		meleeAction.SetDamageFactor(3)
	}
	return nil
}

func returnDamage(context *actions.ItemContext, action actions.Action, engine actions.Engine) []actions.Action {
	factor := 0.1
	meleeAction, isMelee := action.(*actions.MeleeAttackAction)
	if !isMelee {
		return nil
	}
	mainDefender := meleeAction.GetTargetedDefender()
	if mainDefender != context.GetActor() {
		return nil
	}
	damageDealt := meleeAction.GetTotalDamageDealt()
	returnedDamage := max(1, int(float64(damageDealt)*factor))
	return actions.OneAction(actions.NewChangeRessourceAction(action.Cause(), meleeAction.GetAttacker(), stats.Health, -returnedDamage))
}

func fireBallOnRangedAttack(context *actions.ItemContext, action actions.Action, engine actions.Engine) []actions.Action {
	rangedAction, isRanged := action.(*actions.RangedAttackAction)
	if !isRanged {
		return nil
	}
	attacker := rangedAction.GetAttacker()
	if attacker != context.GetActor() {
		return nil
	}

	targetRules := NewRangedAttackTargetingRules(attacker, DijkstraAoE(engine.GetWorldState().GetMap(), 3), engine.GetWorldState().GetMap())
	rangedTargetRules := targetRules.(*RangedTargetingRules)

	targetPos := rangedAction.GetTargetPos()

	cause := actions.NewActorCause(actions.ActionNameRangedAttack, attacker)
	return actions.OneAction(actions.NewFireballAction(cause, rangedAction.GetAttacker(), 8, targetPos, rangedTargetRules.GetHitpositions(targetPos)))
}

// stat change on kill
func manaForKills(context *actions.ItemContext, action actions.Action, engine actions.Engine) []actions.Action {
	amount := 4
	return actions.OneAction(actions.NewChangeRessourceAction(action.Cause(), context.GetActor(), stats.Mana, amount))
}
