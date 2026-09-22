package common

import (
	"rpg/src/stats"
)

type ChangeResourceAction struct {
	BaseAction
	changeFactor    float64
	amount          float64
	actor           *Actor
	changedByAmount float64
	resourceName    stats.Name
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
	hp := receiver.GetStats().GetResource(stats.Health)
	before := hp.Value()
	wasOK := hp.Value()*4 >= hp.GetMax()
	receiver.GetStats().IncrementResourceBy(d.resourceName, d.changedByAmount)
	if d.resourceName == stats.Health && wasOK && hp.Value()*4 < hp.GetMax() && receiver.IsAlive() && receiver == context.GameState.GetPlayer() {
		PlayUICue("low_health") // a quarter of the hero's life left
	}

	if d.changedByAmount < 0 {
		receiver.SetHurt()
		if d.resourceName == stats.Health && receiver.IsAlive() {
			d1HitRecovery(context.GameState.factory, receiver, int(-d.changedByAmount), d.cause.GetActor())
		}
	}

	if d.resourceName == stats.Health && int(d.changedByAmount) != 0 {
		ev := HealthChanged{Actor: receiver, At: receiver.MapPosition(), Amount: d.changedByAmount, Before: before,
			Cause: d.cause.Action()}
		if d.changedByAmount < 0 { // where the blood lands is rolled with the game's dice: it is saved with the map
			ev.Blood = context.GameState.GetMap().GetRandomNeighbor(rng, receiver.MapPosition())
			if rng.Intn(9) == 0 {
				ev.Blood = receiver.MapPosition()
			}
		}
		context.ActionBroker.Emit(ev)
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
