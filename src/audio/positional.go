package audio

import (
	"math"
	"time"
)

// PlayCueAt plays a cue attenuationDB quieter and panned: panDB > 0 quiets the left channel by that many dB, < 0 the
// right one (DirectSound's SetVolume and SetPan, which D1 plays its located sounds with). Like D1's snd_play_snd, a
// cue that started less than 80 ms ago doesn't start again.
func (ap *Player) PlayCueAt(cue string, attenuationDB, panDB float64) Handle {
	sound, ok := ap.availableSounds[cue]
	if ap.disablePlayback || !ok || time.Since(sound.lastStart) < 80*time.Millisecond {
		return &NullHandle{}
	}
	if !sound.IsLoaded() && !sound.Load(ap) {
		return &NullHandle{}
	}
	sound.lastStart = time.Now()
	left, right := 1.0, 1.0
	if panDB > 0 {
		left = dbToGain(-panDB)
	} else {
		right = dbToGain(panDB)
	}
	player := ap.context.NewPlayerFromBytes(panned(sound.Next(), left, right))
	active := ap.addActiveSound(sound, player, nil)
	active.gain = dbToGain(-attenuationDB)
	player.SetVolume(ap.volumeFor(sound) * active.gain)
	player.Play()
	return player
}

func dbToGain(db float64) float64 { return math.Pow(10, db/20) }

// panned scales the channels of 16-bit little-endian stereo samples (what the context plays); unpanned ones are
// shared, not copied.
func panned(b []byte, left, right float64) []byte {
	if left == 1 && right == 1 {
		return b
	}
	out := make([]byte, len(b))
	for i := 0; i+3 < len(b); i += 4 {
		for c, g := range [2]float64{left, right} {
			j := i + 2*c
			v := int16(uint16(b[j]) | uint16(b[j+1])<<8)
			v = int16(float64(v) * g)
			out[j], out[j+1] = byte(v), byte(uint16(v)>>8)
		}
	}
	return out
}
