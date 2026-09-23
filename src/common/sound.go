package common

import (
	"rpg/src/audio"
	"rpg/src/geometry"
	"rpg/src/rules"
)

// soundPosition ports calc_snd_position: a sound at p, heard from the listener, is 0.64 dB quieter per cell of
// distance (down to snd_play_snd's -16 dB floor) and panned 2.56 dB per cell to the side, and is not heard at all
// 25 cells to the side or 100 away. D1 pans by x-y, the horizontal of its isometric screen; ours is x.
func soundPosition(listener, p geometry.Point) (attenuationDB, panDB float64, heard bool) {
	d := p.Sub(listener)
	pan := d.X * 256
	vol := max(abs(d.X), abs(d.Y)) * 64
	if abs(pan) > 6400 || vol >= 6400 {
		return 0, 0, false
	}
	return float64(min(vol, 1600)) / 100, float64(pan) / 100, true
}

func abs(x int) int { return max(x, -x) }

// playCueAt plays a sound cue as heard by the player from p (PlaySfxLoc). It reports whether the cue exists, so
// callers can fall back to a generic one.
func playCueAt(ctx ActionContext, cue string, p geometry.Point) bool {
	return playCueFrom(ctx.AudioPlayer, ctx.GameState.GetPlayer(), cue, p)
}

// PlaySoundAt is playCueAt outside an action.
func (g *Factory) PlaySoundAt(cue string, p geometry.Point) bool {
	return playCueFrom(g.Audio, g.State.GetPlayer(), cue, p)
}

// PlayMonsterSound plays a monster's sound of a category (attack, hit, death, special) where it stands
// (PlayEffect); false when it has none.
func (g *Factory) PlayMonsterSound(a *Actor, category string) bool {
	return g.PlaySoundAt(a.MonsterSoundCue(category), a.MapPosition())
}

func playCueFrom(ap *audio.Player, listener *Actor, cue string, p geometry.Point) bool {
	if ap == nil || cue == "" || !ap.HasCue(cue) {
		return false
	}
	if listener != nil {
		if att, pan, ok := soundPosition(listener.MapPosition(), p); ok {
			ap.PlayCueAt(cue, att, pan)
		}
	}
	return true
}

// playSfxAt plays a D1 sound effect id (_sfx_id, e.g. IS_CAST2) from p: sounds.rec's event sfx.<id>, whose
// files are that id's and those of its RndSFX variants. "" is silent.
func playSfxAt(ctx ActionContext, id string, p geometry.Point) bool {
	return playCueAt(ctx, sfxCue(id), p)
}

// PlaySfx is playSfxAt outside an action.
func (g *Factory) PlaySfx(id string, p geometry.Point) bool {
	return g.PlaySoundAt(sfxCue(id), p)
}

// sfxCue is the sounds.rec event of a D1 sound effect id.
func sfxCue(id string) string {
	if id == "" {
		return ""
	}
	return "sfx." + id
}

// heroPainSound is the hero's grunt when hurt (StartPlrHit, PS_WARR69 and kin) or cry when killed
// (StartPlayerKill, PS_WARR71 and kin), in their voice (heroVoice). The Warrior dies with PS_DEAD, as in 1.09.
func heroPainSound(ctx ActionContext, hero *Actor) {
	voice := ctx.GameState.factory.heroVoice()
	id := "PS_" + voice + "69"
	if !hero.IsAlive() {
		id = "PS_" + voice + "71"
		if death := hero.hero.Rules().Class(hero.hero.Class).DeathSound; death != "" {
			id = death
		}
	}
	playSfxAt(ctx, id, hero.MapPosition())
}

// itemSfx is a D1 item's sound landing on the floor (drop) or going into the pack (inv); "" for other items.
func itemSfx(item *Item) (drop, inv string) {
	if item == nil || item.roll == nil {
		return "", ""
	}
	return item.data.ItemSnds(item.roll.Curs)
}

// identifyCue plays what identifying it turned up: a unique, a curse (it's worth less than it looked), or magic.
func identifyCue(it *rules.Item) {
	switch {
	case it.Magical == 2:
		PlayUICue("identify_art")
	case it.IValue < it.Value: // ponytail: price as the curse test; read the affixes if it misjudges
		PlayUICue("identify_bad")
	default:
		PlayUICue("identify_ego")
	}
}

// UICues is the running game's audio player, for the engine's non-positional cues from code that holds no
// ActionContext (shop transactions). NewGameInstance sets it.
var UICues *audio.Player

// PlayUICue plays one of the engine's cues (store_enter, vendor_buy, identify_art, ...) if the mod wired it up:
// each mod's sounds.rec decides what, if anything, it sounds like.
func PlayUICue(cue string) {
	if UICues != nil && UICues.HasCue(cue) {
		UICues.PlayCue(cue)
	}
}
