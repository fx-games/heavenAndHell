package common

import (
	"fmt"
	"math/rand"
	"rpg/audio"
	"rpg/d1"
	"rpg/defs"
	"rpg/stats"
	"rpg/util"
)

// Will turn definitions into actual game objects
type Factory struct {
	ActionBroker   *Broker
	RandomSource   *rand.Rand
	State          *State
	UI             func() UserInterface
	GetTimeKeeper  func() *TimeKeeper
	Msg            func(msg string)
	QueueAfterTurn func(func())
	Generator      *d1.Generator
	Audio          *audio.Player
	speechCue      string // the D1 speech cue playing, stopped by the next (sfx_STREAM)
	identifyTarget *Item  // the item picked for the Identify cast in progress
}

// playD1Sound ports PlaySFX for a D1 sound id (d1.SfxFiles). Speech replaces speech still playing, like D1's
// streamed sounds. Silent without Diablo's own sounds.
func (g *Factory) playD1Sound(id string) {
	if g.Audio == nil || !g.Audio.HasCue(id) {
		return
	}
	if g.speechCue != "" {
		g.Audio.Stop(g.speechCue) // Stop, not Close: keeps activeSounds in sync, avoiding a double-close in Update
	}
	g.Audio.PlayCue(id)
	g.speechCue = id
}

// d1HeroVoice is the voice set each class speaks with. The Monk has his own (hfmonk.mpq / hfvoice.mpq); the Bard
// and the Barbarian borrow the Rogue's and the Warrior's, which is what Hellfire itself does — EFFECTS.H defines
// PS_BARD1 as PS_ROGUE1 and so on. A missing file is simply not registered, so without the expansion's archives
// the Monk falls silent rather than breaking.
var d1HeroVoice = [d1.NumClasses]string{d1.Warrior: "WARR", d1.Rogue: "ROGUE", d1.Sorcerer: "MAGE", d1.Monk: "MONK", d1.Bard: "ROGUE", d1.Barbarian: "WARR"}

// playD1HeroSound plays the hero's class version of a hero line: n 8 is PS_WARR8, PS_ROGUE8 or PS_MAGE8.
func (g *Factory) playD1HeroSound(n int) {
	g.playD1Sound(fmt.Sprintf("PS_%s%d", d1HeroVoice[g.State.GetPlayer().Hero().Class], n))
}

func (g *Factory) D1() *d1.Generator { return g.Generator }

func (g *Factory) NewSign(inscription []string) *BumpObject {
	return NewBumpObject("a sign", defs.IconSign, func(actor *Actor, startAnimating func() bool) {
		g.UI().OpenTextWindow(util.ToLabelText(inscription))
	})
}

func (g *Factory) NewStash() *BumpObject {
	return NewBumpObject("your stash", defs.Icon("prop.chest1"), func(actor *Actor, startAnimating func() bool) {
		g.UI().OpenPlayerStashInTown()
	})
}

func (g *Factory) NewPoisonedEffect(turnsLeft int, damagePerRound int) *ParametrizedStatusEffect {
	return NewParametrizedStatusEffect(StatusDamageOverTime, func(a *Actor) {
		g.ActionBroker.Execute(NewChangeResourceAction(AbstractCause("poison"), a, stats.Health, float64(-damagePerRound)))
	}, turnsLeft)
}
