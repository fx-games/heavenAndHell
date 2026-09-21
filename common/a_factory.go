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

// d1HeroVoice is the voice set the hero speaks with. D1 and Hellfire have one woman's set (the Rogue's) and three
// men's (Warrior, Sorcerer, Monk; the Monk's is in hfmonk.mpq / hfvoice.mpq and falls silent without it), all with
// the same line numbers. So every heroine speaks with the Rogue's, and a man with his class's own, the Warrior's
// for the Rogue, the Bard and the Barbarian.
func (g *Factory) d1HeroVoice() string {
	if g.State.SelectedClass.IsFemale() {
		return "ROGUE"
	}
	switch g.State.GetPlayer().Hero().Class {
	case d1.Sorcerer:
		return "MAGE"
	case d1.Monk:
		return "MONK"
	}
	return "WARR"
}

// PlayD1HeroSound plays the hero's voice version of a hero line: n 8 is PS_WARR8, PS_ROGUE8 or PS_MAGE8.
func (g *Factory) PlayD1HeroSound(n int) {
	id := fmt.Sprintf("PS_%s%d", g.d1HeroVoice(), n)
	if id == "PS_WARR96" { // the warrior's catacombs line is PS_WARR96B
		id += "B"
	}
	g.playD1Sound(id)
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
