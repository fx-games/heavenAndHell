package common

import (
	"math/rand"
	"rpg/stats"
	"strings"
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
	// seeded from the game's dice rather than the clock: a run replayed from its seed has to roll the same
	randomSource := rand.New(rand.NewSource(rand.Int63()))
	var animations []Animation
	amount := int(d.changedByAmount)
	if d.resourceName == stats.Health && amount != 0 {
		animation := NewDamageNumberAnimation(d.actor.MapPosition(), amount)
		animation.StartWhenTrue(d.shouldStartAnimation)
		animations = append(animations, animation)
		if amount < 0 {
			hitAnimation := NewHitAnimation(d.actor.MapPosition(), 0.2, 43, 2)
			hitAnimation.StartWhenTrue(d.shouldStartAnimation)
			// rolled here, not in the callback: an animation finishes when the frame says so, and a global
			// die rolled from there shifts every later roll in the game depending on when it ran
			playsHitSound := rand.Intn(10) <= 8
			hitAnimation.SetOnFinished(func() {
				freeNeighbor := context.GameState.GetMap().GetRandomNeighbor(randomSource, d.actor.MapPosition())
				roll := randomSource.Intn(9)
				if roll == 0 {
					freeNeighbor = d.actor.MapPosition()
				}
				context.GameState.AddBloodDecalAt(freeNeighbor)
				if playsHitSound && d.actor != context.GameState.GetPlayer() && (d.actor.IsAlive() || d.actor.d1Monster == nil) {
					d.playHitSound(context)
				}
			})
			if d.shouldStartAnimation != nil {
				d.actor.moveHold = hitAnimation
			}
			animations = append(animations, hitAnimation)
		}
	}
	return animations
}

func (d *ChangeResourceAction) playHitSound(context ActionContext) {
	hitActor := d.actor
	if playCueAt(context, hitActor.MonsterSoundCue("hit"), hitActor.MapPosition()) {
		return
	}
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
