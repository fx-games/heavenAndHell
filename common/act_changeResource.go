package common

import (
	"math/rand"
	"rpg/stats"
	"strings"
	"time"
)

type ChangeResourceAction struct {
	BaseAction
	changeFactor         float64
	amount               float64
	actor                *Actor
	changedByAmount      float64
	resourceName         stats.Name
	shouldStartAnimation func() bool
	animations           []Animation
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

	receiver.GetStats().IncrementResourceBy(d.resourceName, d.changedByAmount)

	if d.changedByAmount < 0 { // this needs to be synched to the animation

		receiver.SetHurt()
	}

	d.animations = d.prepareAnimations(context)
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

func (d *ChangeResourceAction) GetAnimations() []Animation { // TODO: Not allowed to create new animations here..
	return d.animations
}

func (d *ChangeResourceAction) prepareAnimations(context ActionContext) []Animation {
	randomSource := rand.New(rand.NewSource(time.Now().UnixNano()))
	var animations []Animation
	amount := int(d.changedByAmount)
	if d.resourceName == stats.Health && amount != 0 {
		animation := NewDamageNumberAnimation(d.actor.MapPosition(), amount)
		animation.StartWhenTrue(d.shouldStartAnimation)
		animations = append(animations, animation)
		if amount < 0 {
			hitAnimation := NewHitAnimation(d.actor.MapPosition(), 0.2, 43, 2)
			hitAnimation.StartWhenTrue(d.shouldStartAnimation)
			hitAnimation.SetOnFinished(func() {
				freeNeighbor := context.GameState.GetMap().GetRandomNeighbor(randomSource, d.actor.MapPosition())
				roll := randomSource.Intn(9)
				if roll == 0 {
					freeNeighbor = d.actor.MapPosition()
				}
				context.GameState.AddBloodDecalAt(freeNeighbor)
				if rand.Intn(10) <= 8 && d.actor != context.GameState.GetPlayer() {
					d.playHitSound(context)
				}
			})
			animations = append(animations, hitAnimation)
		}
	}
	return animations
}

func (d *ChangeResourceAction) playHitSound(context ActionContext) {
	hitActor := d.actor
	hitCue := "human_hit"
	if strings.HasPrefix(hitActor.GetInternalName(), "skeleton") {
		hitCue = "skeleton_hit"
	} else if d.cause.Action() == ActionNameRangedAttack {
		hitCue = "missile_hit_human"
	} else if d.changedByAmount < -9 && !hitActor.IsAlive() {
		hitCue = "heavy_hit_human"
	}
	audio := context.AudioPlayer
	audio.PlayCue(hitCue)
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
