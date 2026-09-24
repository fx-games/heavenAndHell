package common

import (
	"fmt"
	"math/rand"
	"rpg/src/audio"
	"rpg/src/defs"
	"rpg/src/dungen"
	"rpg/src/geometry"
	"rpg/src/rules"
	"rpg/src/stats"
)

// Will turn definitions into actual game objects
type Factory struct {
	ActionBroker   *Broker
	RandomSource   *rand.Rand
	State          *State
	Show           func(UIRequest) // see ActionContext.Show
	GetTimeKeeper  func() *TimeKeeper
	Msg            func(msg string)
	QueueAfterTurn func(func())
	Generator      *rules.Generator
	Levels         *dungen.Levels           // the mod's level tables and which strategy builds each level
	Classes        []defs.LoadoutDefinition // the mod's class roster, in archetype order: each class's own skills
	Audio          *audio.Player
	speechCue      string         // the D1 speech cue playing, stopped by the next (sfx_STREAM)
	identifyTarget *Item          // the item picked for the Identify cast in progress
	talkAt         geometry.Point // where the NPC being talked to stands: drop_item and drop_unique land there
	talker         *Actor         // the quest boss an on_talker_* event is about
	talkerVisible  bool           // whether the hero sees it (D1's BFLAG_VISIBLE), for on_talker_turn
}

// playSound ports PlaySFX for a D1 sound id (rules.SfxFiles). Speech replaces speech still playing, like D1's
// streamed sounds. Silent without Diablo's own sounds.
func (g *Factory) playSound(id string) {
	if g.Audio == nil || !g.Audio.HasCue(id) {
		return
	}
	if g.speechCue != "" {
		g.Audio.Stop(g.speechCue) // Stop, not Close: keeps activeSounds in sync, avoiding a double-close in Update
	}
	g.Audio.PlayCue(id)
	g.speechCue = id
}

// heroVoice is the voice set the hero speaks with. D1 and Hellfire have one woman's set (the Rogue's) and three
// men's (Warrior, Sorcerer, Monk; the Monk's is in hfmonk.mpq / hfvoice.mpq and falls silent without it), all with
// the same line numbers. So every heroine speaks with the Rogue's, and a man with his class's own, the Warrior's
// for the Rogue, the Bard and the Barbarian.
func (g *Factory) heroVoice() string {
	class := g.Generator.D.Class(g.State.GetPlayer().Hero().Class)
	if g.State.SelectedClass.IsFemale() {
		return class.FemaleVoice
	}
	return class.Voice
}

// PlayHeroSound plays the hero's voice version of a hero line: n 8 is PS_WARR8, PS_ROGUE8 or PS_MAGE8.
func (g *Factory) PlayHeroSound(n int) {
	id := fmt.Sprintf("PS_%s%d", g.heroVoice(), n)
	if id == "PS_WARR96" { // the warrior's catacombs line is PS_WARR96B
		id += "B"
	}
	g.playSound(id)
}

func (g *Factory) Gen() *rules.Generator { return g.Generator }

func (g *Factory) NewSign(inscription []string) *BumpObject {
	return NewBumpObject("sign", "a sign", defs.IconSign, func(actor *Actor) {
		g.Show(ShowText{inscription})
	})
}

func (g *Factory) NewStash() *BumpObject {
	return NewBumpObject("stash", "your stash", defs.Icon("prop.chest"), func(actor *Actor) {
		g.Show(OpenStash{})
	})
}

func (g *Factory) NewPoisonedEffect(turnsLeft int, damagePerRound int) *ParametrizedStatusEffect {
	return NewParametrizedStatusEffect(StatusDamageOverTime, func(a *Actor) {
		if a.monster != nil && a.monster.Data.Has("poison_immune") {
			return
		}
		g.ActionBroker.Execute(NewChangeResourceAction(AbstractCause("poison"), a, stats.Health, float64(-damagePerRound)))
	}, turnsLeft)
}
