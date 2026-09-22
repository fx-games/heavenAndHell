package common

import (
	"rpg/src/stats"
)

type ChangeResourceAction struct {
	BaseAction
	changeFactor         float64
	amount               float64
	actor                *Actor
	changedByAmount      float64
	resourceName         stats.Name
	shouldStartAnimation func() bool
}

func NewChangeResourceAction(cause Cause, actor *Actor, resourceName stats.Name, amount float64) *ChangeResourceAction {
	return &ChangeResourceAction{
		BaseAction:   BaseAction{cause: cause, name: ActionNameChangeResource},
		actor:        actor,
		amount:       amount,
		changeFactor: 1.0,
		resourceName: resourceName,
	}
}
func (d *ChangeResourceAction) Execute(context ActionContext) []Action {
	receiver := d.actor
	d.changedByAmount = float64(d.amount) * d.changeFactor
	if d.resourceName == stats.Health && d.changedByAmount < 0 && d.shouldStartAnimation != nil && !receiver.VisualsPending() {
		receiver.shownHealth = receiver.GetStats().GetResource(stats.Health).Value()
	}

	hp := receiver.GetStats().GetResource(stats.Health)
	wasOK := hp.Value()*4 >= hp.GetMax()
	receiver.GetStats().IncrementResourceBy(d.resourceName, d.changedByAmount)
	if d.resourceName == stats.Health && wasOK && hp.Value()*4 < hp.GetMax() && receiver.IsAlive() && receiver == context.GameState.GetPlayer() {
		PlayUICue("low_health") // a quarter of the hero's life left
	}

	if d.changedByAmount < 0 {
		if d.shouldStartAnimation != nil {
			receiver.visualGate = d.shouldStartAnimation // tint and death wait for the projectile too
		}
		receiver.SetHurt()
		if d.resourceName == stats.Health && receiver.IsAlive() {
			d1HitRecovery(context.GameState.factory, receiver, int(-d.changedByAmount), d.cause.GetActor())
		}
		if d.resourceName == stats.Health && receiver.hero != nil && receiver == context.GameState.GetPlayer() {
			whenVisible(context.ActionBroker, receiver, func() { d1HeroPainSound(context, receiver) })
		}
	}

	if d.resourceName == stats.Health && int(d.changedByAmount) != 0 {
		context.ActionBroker.Emit(HealthChanged{Actor: receiver, At: receiver.MapPosition(), Amount: d.changedByAmount, Cause: d.cause.Action(), ShowWhen: d.shouldStartAnimation})
	}
	return nil
}

func (d *ChangeResourceAction) GetChangeFactor() float64 {
	return d.changeFactor
}

func (d *ChangeResourceAction) SetChangeFactor(factor float64) {
	d.changeFactor = factor
}
func (d *ChangeResourceAction) GetActor() *Actor {
	return d.actor
}

func (d *ChangeResourceAction) GetResourceName() stats.Name {
	return d.resourceName
}

func (d *ChangeResourceAction) GetChangeAmount() float64 {
	return d.amount
}

func (d *ChangeResourceAction) SetAnimationStartPredicate(shouldStartAnimation func() bool) {
	d.shouldStartAnimation = shouldStartAnimation
}
