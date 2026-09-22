package common

import (
	"math/rand"
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
		return presentHealthChanged(b.context, ev)
	}
	return nil
}

// sees is whether the player sees something happen at a or b: always when it is their own doing.
func sees(b *Broker, byPlayer bool, a, c geometry.Point) bool {
	player := b.context.GameState.GetPlayer()
	return byPlayer || player.CanSee(a) || player.CanSee(c)
}

// presentHealthChanged is the damage number, and for a hit the flash, the blood and the hit sound.
func presentHealthChanged(context ActionContext, ev HealthChanged) []Animation {
	// seeded from the game's dice rather than the clock: a run replayed from its seed has to roll the same
	randomSource := rand.New(rand.NewSource(rng.Int63()))
	animations := []Animation{}
	number := NewDamageNumberAnimation(ev.At, int(ev.Amount))
	number.StartWhenTrue(ev.ShowWhen)
	animations = append(animations, number)
	if ev.Amount >= 0 {
		return animations
	}
	hit := NewHitAnimation(ev.At, 0.2, 43, 2)
	hit.StartWhenTrue(ev.ShowWhen)
	// rolled here, not in the callback: an animation finishes when the frame says so, and a global die rolled
	// from there shifts every later roll in the game depending on when it ran
	playsHitSound := rng.Intn(10) <= 8
	actor := ev.Actor
	hit.SetOnFinished(func() {
		freeNeighbor := context.GameState.GetMap().GetRandomNeighbor(randomSource, actor.MapPosition())
		if randomSource.Intn(9) == 0 {
			freeNeighbor = actor.MapPosition()
		}
		context.GameState.AddBloodDecalAt(freeNeighbor)
		if playsHitSound && actor != context.GameState.GetPlayer() && (actor.IsAlive() || actor.d1Monster == nil) {
			playHitSound(context, ev)
		}
	})
	if ev.ShowWhen != nil {
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
