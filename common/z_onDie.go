package common

import (
	"fmt"
	"math/rand"

	"rpg/stats"
)

type OnDieHook struct {
	ItemHook
	effectDescription string
}

func (d *OnDieHook) ToString() string {
	return fmt.Sprintf("On Kill: %s", d.effectDescription)
}

func (d *OnDieHook) Name() string {
	return "OnDieHook"
}

func NewOnDieHook(effectDescription string, afterHook func(itemContext *ItemContext, action Action, engine ActionContext) []Action) *OnDieHook {
	return &OnDieHook{
		ItemHook: ItemHook{
			HookedAction: ActionNameDie,
			AfterHook:    afterHook,
		},
		effectDescription: effectDescription,
	}
}

type OnKillHook struct {
	BaseActionHook
}

func (d *OnKillHook) IsBefore() bool {
	return false
}

func (d *OnKillHook) IsAfter() bool {
	return true
}

func NewMainOnKillHook() *OnKillHook {
	return &OnKillHook{
		BaseActionHook: BaseActionHook{
			hookedAction: ActionNameDie,
		},
	}
}

func (d *OnKillHook) After(action Action, context ActionContext) []Action {
	dieAction := action.(*DieAction)
	killer := dieAction.Cause().GetActor()
	victim := dieAction.GetActor()

	state := context.GameState
	state.GetVisibility().RemoveActor(victim)
	if killer != nil && victim != nil {
		return d.onKill(context, killer, victim)
	}
	return nil
}

func (d *OnKillHook) onKill(context ActionContext, killer, victim *Actor) []Action {
	msg := context.Msg
	victimLevel := victim.GetStats().GetAttribute(stats.Level)
	victimXP := max(1, victim.GetStats().GetAttribute(stats.Experience))

	killerLevel := killer.GetStats().GetAttribute(stats.Level)

	multiplier := float64(victimLevel) / float64(killerLevel) //

	awardXP := int(float64(victimXP) * multiplier)

	awardGold := rand.Intn(victimXP) + victimXP + rand.Intn(awardXP) + awardXP

	// should technically be its own action, if we wanted to hook this..
	killer.GetStats().IncrementBy(stats.Experience, awardXP)
	killer.GetStats().IncrementBy(stats.Gold, awardGold)
	state := context.GameState
	if killer == state.GetPlayer() {
		msg(fmt.Sprintf("You gain %d XP and %d gold", awardXP, awardGold))
		flags := state.GetFlags()

		flags.Increment(fmt.Sprintf("killed:%s", victim.GetInternalName()))

		currentMap := state.GetMap()
		livingEnemies := currentMap.GetFilteredActors(func(actor *Actor) bool {
			return actor.IsAlive() && !actor.IsVendor() && actor != killer
		})
		currentMapName := currentMap.GetName()
		if len(livingEnemies) == 0 {
			flags.SetTrue(fmt.Sprintf("all_clear:%s", currentMapName))
		}
	} else {
		msg(fmt.Sprintf("%s gains %d XP and %d gold", killer.Name(), awardXP, awardGold))
	}

	return nil
}
