package actions

import (
	"fmt"
	"math/rand"
	"rpg/common"
	"rpg/stats"
	"strings"
	"time"
)

type ChangeRessourceAction struct {
	BaseAction
	changeFactor         float64
	amount               int
	actor                *common.Actor
	changedByAmount      int
	ressourceName        stats.Name
	shouldStartAnimation func() bool
	animations           []common.Animation
}

func NewChangeRessourceAction(cause Cause, actor *common.Actor, ressourceName stats.Name, amount int) *ChangeRessourceAction {
	return &ChangeRessourceAction{
		BaseAction:    BaseAction{cause: cause, name: ActionNameChangeRessource},
		actor:         actor,
		amount:        amount,
		changeFactor:  1.0,
		ressourceName: ressourceName,
	}
}
func (d *ChangeRessourceAction) Execute(engine Engine) []Action {
	receiver := d.actor
	d.changedByAmount = int(float64(d.amount) * d.changeFactor)

	receiver.GetStats().IncrementRessourceBy(d.ressourceName, d.changedByAmount)

	if d.changedByAmount < 0 { // this needs to be synched to the animation

		receiver.SetHurt()
	}

	d.animations = d.prepareAnimations(engine)
	println(fmt.Sprintf("(%s - %s) Changed %s by %d", d.cause.Action(), d.Name(), receiver.Name(), d.changedByAmount))
	return nil
}

func (d *ChangeRessourceAction) GetChangeFactor() float64 {
	return d.changeFactor
}

func (d *ChangeRessourceAction) SetChangeFactor(factor float64) {
	d.changeFactor = factor
}
func (d *ChangeRessourceAction) GetActor() *common.Actor {
	return d.actor
}

func (d *ChangeRessourceAction) GetAnimations() []common.Animation { // TODO: Not allowed to create new animations here..
	return d.animations
}

func (d *ChangeRessourceAction) prepareAnimations(engine Engine) []common.Animation {
	randomSource := rand.New(rand.NewSource(time.Now().UnixNano()))
	var animations []common.Animation
	if d.ressourceName == stats.Health {
		animation := NewDamageNumberAnimation(d.actor.MapPosition(), d.changedByAmount)
		animation.StartWhenTrue(d.shouldStartAnimation)
		animations = append(animations, animation)
		if d.changedByAmount < 0 {
			hitAnimation := NewHitAnimation(d.actor.MapPosition(), 0.2, 43, 2)
			hitAnimation.StartWhenTrue(d.shouldStartAnimation)
			hitAnimation.SetOnFinished(func() {
				freeNeighbor := engine.GetWorldState().GetMap().GetRandomNeighbor(randomSource, d.actor.MapPosition())
				roll := randomSource.Intn(9)
				if roll == 0 {
					freeNeighbor = d.actor.MapPosition()
				}
				engine.AddBloodDecalAt(freeNeighbor)
				if rand.Intn(10) <= 8 && d.actor != engine.GetWorldState().GetPlayer() {
					d.playHitSound(engine)
				}
			})
			animations = append(animations, hitAnimation)
		}
	}
	return animations
}

func (d *ChangeRessourceAction) playHitSound(engine Engine) {
	hitActor := d.actor
	hitCue := "human_hit"
	if strings.HasPrefix(hitActor.GetInternalName(), "skeleton") {
		hitCue = "skeleton_hit"
	} else if d.cause.Action() == ActionNameRangedAttack {
		hitCue = "missile_hit_human"
	} else if d.changedByAmount < -9 && !hitActor.IsAlive() {
		hitCue = "heavy_hit_human"
	}
	audio := engine.GetAudio()
	audio.PlayCue(hitCue)
}

func (d *ChangeRessourceAction) GetRessourceName() stats.Name {
	return d.ressourceName
}

func (d *ChangeRessourceAction) GetChangeAmount() int {
	return d.amount
}

func (d *ChangeRessourceAction) SetAnimationStartPredicate(shouldStartAnimation func() bool) {
	d.shouldStartAnimation = shouldStartAnimation
}
