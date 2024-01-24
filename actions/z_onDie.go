package actions

import (
	"fmt"
	"math/rand"
	"rpg/common"
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

func NewOnDieHook(effectDescription string, afterHook func(itemContext *ItemContext, action Action, engine Engine) []Action) *OnDieHook {
	return &OnDieHook{
		ItemHook: ItemHook{
			BaseActionHook: BaseActionHook{
				hookedAction: ActionNameDie,
			},
			afterHook: afterHook,
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

func (d *OnKillHook) After(action Action, engine Engine) []Action {
	dieAction := action.(*DieAction)
	killer := dieAction.Cause().GetActor()
	victim := dieAction.GetActor()

	engine.GetWorldState().GetVisibility().RemoveActor(victim)
	if killer != nil && victim != nil {
		return d.onKill(engine, killer, victim)
	}
	return nil
}

func (d *OnKillHook) onKill(engine Engine, killer, victim *common.Actor) []Action {
	victimLevel := victim.GetStats().GetAttribute(stats.Level)
	victimXP := max(1, victim.GetStats().GetAttribute(stats.Experience))

	killerLevel := killer.GetStats().GetAttribute(stats.Level)

	multiplier := float64(victimLevel) / float64(killerLevel) //

	awardXP := int(float64(victimXP) * multiplier)

	awardGold := rand.Intn(victimXP)

	// should technically be its own action, if we wanted to hook this..
	killer.GetStats().IncrementBy(stats.Experience, awardXP)
	killer.GetStats().IncrementBy(stats.Gold, awardGold)

	if killer == engine.GetWorldState().GetPlayer() {
		engine.Log(fmt.Sprintf("You gain %d XP and %d gold", awardXP, awardGold))
		flags := engine.GetWorldState().GetFlags()

		flags.Increment(fmt.Sprintf("killed:%s", victim.GetInternalName()))

		currentMap := engine.GetWorldState().GetMap()
		livingEnemies := currentMap.GetFilteredActors(func(actor *common.Actor) bool {
			return actor.IsAlive() && !actor.IsVendor() && actor != killer
		})
		currentMapName := currentMap.GetName()
		if len(livingEnemies) == 0 {
			flags.SetTrue(fmt.Sprintf("all_clear:%s", currentMapName))
		}
	} else {
		engine.Log(fmt.Sprintf("%s gains %d XP and %d gold", killer.Name(), awardXP, awardGold))
	}

	return nil
}
