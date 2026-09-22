package common

import (
	"strings"

	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/util"
)

// Presenter turns an event into the animations that show it. Present is the game's; a headless run could use one
// that shows nothing.
type Presenter func(b *Broker, ev Event) []Animation

// Present is the tile and terminal UIs' presentation of the rules' events.
func Present(b *Broker, ev Event) []Animation {
	switch ev := ev.(type) {
	case MeleeSwing:
		ticks := util.SecondsToTicks(0.2)
		return OneAnimation(&MeleeAttackAnimation{actor: ev.Attacker, targetPos: ev.To, origin: ev.From, ticksNeeded: ticks, ticksLeft: ticks})
	case Moved:
		if ev.Quiet || !sees(b, ev.ByPlayer, ev.From, ev.To) {
			ev.Actor.SetDrawPosition(ev.To.ToPointF())
			return nil
		}
		return OneAnimation(&MovementAnimation{actor: ev.Actor, path: []geometry.Point{ev.From, ev.To},
			ticksNeeded: util.SecondsToTicks(ev.Delay), isForPlayer: ev.ByPlayer})
	case Swapped:
		if ev.Quiet || !sees(b, ev.ByPlayer, ev.One.MapPosition(), ev.Two.MapPosition()) {
			ev.One.SetDrawPosition(ev.One.MapPosition().ToPointF())
			ev.Two.SetDrawPosition(ev.Two.MapPosition().ToPointF())
			return nil
		}
		return OneAnimation(NewSwapAnimation(ev.One, ev.Two, ev.Delay, ev.ByPlayer))
	case Missile:
		flight := NewProjectilePathAnimation(ev.By, ev.Path, ev.SecPerTile, ev.Icon, 2)
		if ev.ArriveSfx != "" {
			end := ev.Path[len(ev.Path)-1]
			flight.SetOnFinish(func() { playD1SfxAt(b.context, ev.ArriveSfx, end) })
		}
		return OneAnimation(flight)
	case Flash:
		return OneAnimation(NewHitAnimation(ev.At, ev.Seconds, ev.Icon, 2))
	case Said:
		b.AddAnimation(NewSpeechAnimation(ev.Actor, []string{ev.Line})) // up at once, like the line in the log
		return nil
	case KnockedBack:
		if ev.Quiet || ev.From == ev.To {
			ev.Actor.SetDrawPosition(ev.To.ToPointF())
			return nil
		}
		push := &MovementAnimation{actor: ev.Actor, ticksNeeded: util.SecondsToTicks(ev.Delay * geometry.Distance(ev.From, ev.To))}
		push.SetSimplePath(ev.From, ev.To)
		return OneAnimation(push)
	case Shot:
		icon := defs.IconEffectArrow
		if weapon := ev.Attacker.GetEquipment().GetWeapon(RangedAttack); weapon != nil && weapon.GetWeaponStats().ProjectileIcon > -1 {
			icon = weapon.GetWeaponStats().ProjectileIcon // D1 monsters shoot without a weapon item
		}
		return OneAnimation(NewProjectileDirectionAnimation(ev.Attacker, ev.From, ev.To, 0.02, icon, 1))
	case Thrown:
		ev.Item.SetDrawnToScreen(false) // it lands when the animation does
		flight := NewProjectileDirectionAnimation(ev.Thrower, ev.From, ev.To, 0.04, 41, 1)
		flight.SetIcon(ev.Item.Icon(0))
		flight.SetOnFinish(func() { ev.Item.SetDrawnToScreen(true) })
		return OneAnimation(flight)
	case Fireball:
		return OneAnimation(NewFireballAnimation(ev.Caster, ev.To, ev.Icon, ev.Frames, ev.Hits))
	case HealthChanged:
		return presentHealthChanged(b, ev)
	}
	return nil
}

// sees is whether the player sees something happen at a or b: always when it is their own doing.
func sees(b *Broker, byPlayer bool, a, c geometry.Point) bool {
	player := b.context.GameState.GetPlayer()
	return byPlayer || player.CanSee(a) || player.CanSee(c)
}

// presentHealthChanged is the damage number, and for a hit the flash, the blood and the hit sound. A blow whose
// missile is still in the air, or that landed on an actor the player is still watching move, is held back until
// it arrives; until then the actor keeps its old health, tint and, if it died, its feet.
func presentHealthChanged(b *Broker, ev HealthChanged) []Animation {
	context, actor := b.context, ev.Actor
	show := b.whenShown(ev.At, actor)
	if ev.Amount < 0 && show != nil && !actor.VisualsPending() {
		actor.shownHealth = ev.Before
		actor.visualGate = show // tint and death wait for the blow too
	}
	number := NewDamageNumberAnimation(ev.At, int(ev.Amount))
	number.StartWhenTrue(show)
	animations := []Animation{number}
	if ev.Amount >= 0 {
		return animations
	}
	if actor.hero != nil && actor == context.GameState.GetPlayer() {
		whenVisible(b, actor, func() { d1HeroPainSound(context, actor) })
	}
	hit := NewHitAnimation(ev.At, 0.2, 43, 2)
	hit.StartWhenTrue(show)
	playsHitSound := b.fx.Intn(10) <= 8 // how it sounds is nobody's business but the presenter's
	hit.SetOnFinished(func() {
		context.GameState.AddBloodDecalAt(ev.Blood)
		if playsHitSound && actor != context.GameState.GetPlayer() && (actor.IsAlive() || actor.d1Monster == nil) {
			playHitSound(context, ev)
		}
	})
	if show != nil {
		actor.moveHold = hit
	}
	return append(animations, hit)
}

func playHitSound(context ActionContext, ev HealthChanged) {
	hitActor := ev.Actor
	if playCueAt(context, hitActor.MonsterSoundCue("hit"), hitActor.MapPosition()) {
		return
	}
	hitCue := "human_hit"
	if strings.HasPrefix(hitActor.GetInternalName(), "skeleton") {
		hitCue = "skeleton_hit"
	} else if ev.Cause == ActionNameRangedAttack {
		hitCue = "missile_hit_human"
	} else if ev.Amount < -9 && !hitActor.IsAlive() {
		hitCue = "heavy_hit_human"
	}
	context.AudioPlayer.PlayCue(hitCue)
}
