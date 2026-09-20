package audio

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"path"
	"slices"
	"strings"
	"time"

	"rpg/recfile"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
)

func Clamp(value float64, minVal float64, maxVal float64) float64 {
	return min(max(value, minVal), maxVal)
}

type Cue struct {
	AudioCues    []string
	IsRandom     bool
	CurrentIndex int
}

func (c *Cue) Next() string {
	if c.IsRandom {
		randomIndex := rand.Intn(len(c.AudioCues))
		return c.AudioCues[randomIndex]
	}
	if c.CurrentIndex >= len(c.AudioCues) {
		c.CurrentIndex = 0
	}
	c.CurrentIndex++
	return c.AudioCues[c.CurrentIndex]
}

type Player struct {
	context         *audio.Context
	availableSounds map[string]*Sound
	activeSounds    map[string][]*ActiveSound
	queuedSounds    []string
	loopVolume      float64
	sfxVolume       float64
	voiceVolume     float64
	musicTracks     []string // cues registered by RegisterMusic / RegisterMusicEvents, in registration order
	currentMusic    string
	masterVolume    float64
	files           fs.ReadDirFS
	disablePlayback bool
}

func (ap *Player) SetDisablePlayback(disable bool) {
	ap.disablePlayback = disable
}
func (ap *Player) GetMasterVolume() float64 {
	return ap.masterVolume
}

func (ap *Player) GetMusicVolume() float64 {
	return ap.loopVolume
}

func (ap *Player) GetSoundVolume() float64 {
	return ap.sfxVolume
}

func (ap *Player) GetVoiceVolume() float64 {
	return ap.voiceVolume
}

func (ap *Player) SetVoiceVolume(volume float64) {
	ap.voiceVolume = Clamp(volume, 0, 1)
	ap.updateVolume()
}

// volumeFor is the effective volume of a sound's category: music loops, voice lines or sound effects.
func (ap *Player) volumeFor(s *Sound) float64 {
	switch {
	case s.IsLooping:
		return ap.effectiveMusicVolume()
	case s.IsVoice:
		return ap.voiceVolume * ap.masterVolume
	}
	return ap.effectiveSoundVolume()
}

func (ap *Player) effectiveMusicVolume() float64 {
	return ap.loopVolume * ap.masterVolume
}

func (ap *Player) effectiveSoundVolume() float64 {
	return ap.sfxVolume * ap.masterVolume
}

func (ap *Player) SetMasterVolume(volume float64) {
	ap.masterVolume = Clamp(volume, 0, 1)
	ap.updateVolume()
}

func (ap *Player) SetMusicVolume(volume float64) {
	ap.loopVolume = Clamp(volume, 0, 1)
	ap.updateVolume()
}

func (ap *Player) SetSoundVolume(volume float64) {
	ap.sfxVolume = Clamp(volume, 0, 1)
	ap.updateVolume()
}

func (ap *Player) updateVolume() {
	for cue, activeSounds := range ap.activeSounds {
		sound := ap.availableSounds[cue]
		for _, activeSound := range activeSounds {
			activeSound.player.SetVolume(ap.volumeFor(sound) * activeSound.gain)
		}
	}
}

type Sound struct {
	Cue          string
	Filenames    []string
	Samples      [][]byte
	IsLooping    bool
	IsVoice      bool
	IsRandomized bool
	currentIndex int
	lastStart    time.Time // PlayCueAt's 80 ms guard
}

func (s *Sound) Unload() {
	s.Samples = make([][]byte, len(s.Filenames))
}

// IsLoaded reports whether at least one of the cue's files decoded; the others may be missing.
func (s *Sound) IsLoaded() bool {
	return slices.ContainsFunc(s.Samples, func(b []byte) bool { return len(b) > 0 })
}

func (s *Sound) Load(ap *Player) bool {
	for i, filename := range s.Filenames {
		print(fmt.Sprintf("Loading %s...", filename))
		s.Samples[i] = ap.LoadOGG(filename)
		println("done")
	}
	if !s.IsLoaded() {
		println("ERR: Sound cue not found: " + s.Cue)
		return false
	}
	return true
}

// Next picks the sample to play: a random loaded one for randomized cues, skipping files that failed to load.
func (s *Sound) Next() []byte {
	if s.IsRandomized {
		var loaded [][]byte
		for _, b := range s.Samples {
			if len(b) > 0 {
				loaded = append(loaded, b)
			}
		}
		return loaded[rand.Intn(len(loaded))]
	}
	sample := s.Samples[s.currentIndex]
	s.currentIndex = (s.currentIndex + 1) % len(s.Samples)
	return sample
}

func (s *Sound) nextFilename() string {
	if s.IsRandomized {
		return s.Filenames[rand.Intn(len(s.Filenames))]
	}
	filename := s.Filenames[s.currentIndex]
	s.currentIndex = (s.currentIndex + 1) % len(s.Filenames)
	return filename
}

func (s *Sound) OpenStream(ap *Player) *audio.Player {
	return ap.OpenOGG(s.nextFilename())
}

type ActiveSound struct {
	player   *audio.Player
	callback func()
	gain     float64 // on top of the category volume: PlayCueAt's distance
}

type NullHandle struct{}

func (h *NullHandle) Close() error    { return nil }
func (h *NullHandle) IsPlaying() bool { return false }

type SampleRate int

const (
	SampleRate22050 SampleRate = 22050
	SampleRate44100 SampleRate = 44100
)

// audioContext is ebiten's single audio context: creating a second one panics, so a process that builds more
// than one game instance (every test past the first) shares this one.
var audioContext *audio.Context

func NewAudioPlayer(fileSystem fs.ReadDirFS, sampleRate SampleRate) *Player {
	if audioContext == nil {
		audioContext = audio.NewContext(int(sampleRate))
	}
	return &Player{
		files:           fileSystem,
		context:         audioContext,
		availableSounds: make(map[string]*Sound),
		activeSounds:    make(map[string][]*ActiveSound),
		sfxVolume:       1,
		loopVolume:      1,
		voiceVolume:     1,
		masterVolume:    1,
	}
}
func (ap *Player) UnloadAll() {
	for _, sound := range ap.availableSounds {
		sound.Unload()
	}
	ap.activeSounds = make(map[string][]*ActiveSound)
}

func (ap *Player) StopAll() {
	for cue, sounds := range ap.activeSounds {
		for _, sound := range sounds {
			sound.player.Close()
		}
		delete(ap.activeSounds, cue)
	}
}

// LoadOGG loads an OGG file from the given filename into memory.
func (ap *Player) LoadOGG(filename string) []byte {
	file, err := ap.files.Open(filename)
	if err != nil {
		return nil
	}
	defer file.Close()
	stream, err := vorbis.DecodeWithSampleRate(ap.context.SampleRate(), file)
	if err != nil {
		return nil
	}
	buffer, _ := io.ReadAll(stream)
	return buffer
}

// OpenOGG opens an OGG file for streaming, resampled to the context's rate. The file is read into memory first:
// the player keeps reading the stream after this returns.
func (ap *Player) OpenOGG(filename string) *audio.Player {
	data, err := fs.ReadFile(ap.files, filename)
	if err != nil {
		return nil
	}
	stream, decodeErr := vorbis.DecodeWithSampleRate(ap.context.SampleRate(), bytes.NewReader(data))
	if decodeErr != nil {
		return nil
	}
	player, err := ap.context.NewPlayer(stream)
	if err != nil {
		return nil
	}
	return player
}

func (ap *Player) Stop(cue string) {
	if _, ok := ap.activeSounds[cue]; ok {
		for _, sound := range ap.activeSounds[cue] {
			sound.player.Close()
		}
		delete(ap.activeSounds, cue)
	}
}

type Handle interface {
	Close() error
	IsPlaying() bool
}

func (ap *Player) StartLoop(cue string) Handle {
	if ap.disablePlayback {
		return &NullHandle{}
	}
	if sound, ok := ap.availableSounds[cue]; ok {
		if !sound.IsLoaded() && !sound.Load(ap) {
			return &NullHandle{}
		}
		sound.IsLooping = true
		return ap.startPlaying(sound, ap.effectiveMusicVolume(), nil)
	} else {
		println("ERR: Sound cue not found: " + cue)
	}
	return &NullHandle{}
}

func (ap *Player) StartLoopStream(cue string) Handle {
	if ap.disablePlayback {
		return &NullHandle{}
	}
	if sound, ok := ap.availableSounds[cue]; ok {
		sound.IsLooping = true
		player := sound.OpenStream(ap)
		if player == nil {
			println("ERR: Could not open music: " + cue)
			return &NullHandle{}
		}
		player.Rewind()
		player.SetVolume(ap.effectiveMusicVolume())
		player.Play()
		ap.addActiveSound(sound, player, nil)
		return player
	} else {
		println("ERR: Sound cue not found: " + cue)
	}
	return &NullHandle{}
}

func (ap *Player) startPlaying(sound *Sound, volume float64, callback func()) *audio.Player {
	player := ap.context.NewPlayerFromBytes(sound.Next())
	player.Rewind()
	player.SetVolume(volume)
	player.Play()
	ap.addActiveSound(sound, player, callback)
	return player
}
func (ap *Player) addActiveSound(sound *Sound, player *audio.Player, callback func()) *ActiveSound {
	active := &ActiveSound{player: player, callback: callback, gain: 1}
	ap.activeSounds[sound.Cue] = append(ap.activeSounds[sound.Cue], active)
	return active
}
func (ap *Player) PlayCue(cue string) Handle {
	if ap.disablePlayback {
		return &NullHandle{}
	}
	if sound, ok := ap.availableSounds[cue]; ok {
		if !sound.IsLoaded() && !sound.Load(ap) {
			return &NullHandle{}
		}
		return ap.startPlaying(sound, ap.volumeFor(sound), nil)
	} else {
		println("ERR: Sound cue not found: " + cue)
	}
	return &NullHandle{}
}

func (ap *Player) PlayCueWhenFree(cue string) {
	if ap.disablePlayback {
		return
	}
	if !ap.IsSoundEffectPlaying() {
		ap.PlayCue(cue)
	}
	ap.queuedSounds = append(ap.queuedSounds, cue)
}

func (ap *Player) PlayCueWithCallback(cue string, callback func()) Handle {
	if ap.disablePlayback {
		return &NullHandle{}
	}
	if sound, ok := ap.availableSounds[cue]; ok {
		if !sound.IsLoaded() && !sound.Load(ap) {
			return &NullHandle{}
		}
		return ap.startPlaying(sound, ap.volumeFor(sound), callback)
	} else {
		println("ERR: Sound cue not found: " + cue)
	}
	return &NullHandle{}
}

func (ap *Player) IsCuePlaying(cue string) bool {
	if sounds, ok := ap.activeSounds[cue]; ok {
		return len(sounds) > 0
	}
	return false
}

func (ap *Player) Update() {
	if ap.disablePlayback {
		return
	}
	isSoundEffectPlaying := false
	for cue, sounds := range ap.activeSounds {
		soundDef := ap.availableSounds[cue]
		for i := len(sounds) - 1; i >= 0; i-- {
			sound := sounds[i]
			if !sound.player.IsPlaying() {
				if soundDef.IsLooping {
					sound.player.Rewind()
					sound.player.Play()
					continue
				}
				sound.player.Close()
				if sound.callback != nil {
					sound.callback()
				}
				ap.activeSounds[cue] = append(ap.activeSounds[cue][:i], ap.activeSounds[cue][i+1:]...)
			} else if !soundDef.IsLooping {
				isSoundEffectPlaying = true
			}
		}
	}
	if !isSoundEffectPlaying && len(ap.queuedSounds) > 0 {
		cue := ap.queuedSounds[0]
		ap.queuedSounds = ap.queuedSounds[1:]
		ap.PlayCue(cue)
	}
}
func (ap *Player) PreLoadCuesIntoMemory(soundCues []string) {
	for _, soundCue := range soundCues {
		if sound, ok := ap.availableSounds[soundCue]; ok {
			if !sound.IsLoaded() {
				println(fmt.Sprintf("Loading sound cue '%s'", soundCue))
				sound.Load(ap)
			}
		}
	}
}
func (ap *Player) UnloadCues(soundCues []string) {
	for _, soundCue := range soundCues {
		if sound, ok := ap.availableSounds[soundCue]; ok {
			if sound.IsLoaded() {
				println(fmt.Sprintf("Unloading sound cue '%s'", soundCue))
				sound.Unload()
			}
		}
	}
}

// HasCue reports whether a sound cue is registered, so callers can fall back to a generic cue when a
// monster-specific one (from RegisterSoundEvents) isn't available.
func (ap *Player) HasCue(cue string) bool {
	_, ok := ap.availableSounds[cue]
	return ok
}

func (ap *Player) IsSoundEffectPlaying() bool {
	for cue, sounds := range ap.activeSounds {
		for _, sound := range sounds {
			soundDef := ap.availableSounds[cue]
			if sound.player.IsPlaying() && !soundDef.IsLooping {
				return true
			}
		}
	}
	return false
}

// RegisterMusic registers every .ogg in a directory as a background music track named after its file.
func (ap *Player) RegisterMusic(directory string) {
	ap.registerDirectory(directory, func(s *Sound) {
		s.IsLooping = true
		if !slicesContains(ap.musicTracks, s.Cue) {
			ap.musicTracks = append(ap.musicTracks, s.Cue)
		}
	})
}

// RegisterVoices registers every .ogg in a directory as a voice line, played with PlayCue at the voice volume.
func (ap *Player) RegisterVoices(directory string) {
	ap.registerDirectory(directory, func(s *Sound) { s.IsVoice = true })
}

// RegisterVoice registers one .ogg file as a voice line under cue, if the file exists.
func (ap *Player) RegisterVoice(cue, filename string) {
	if _, err := fs.Stat(ap.files, filename); err == nil {
		ap.availableSounds[cue] = &Sound{Cue: cue, Filenames: []string{filename}, Samples: make([][]byte, 1), IsVoice: true}
	}
}

func (ap *Player) registerDirectory(directory string, setup func(s *Sound)) {
	entries, _ := ap.files.ReadDir(directory)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".ogg") {
			continue
		}
		s := &Sound{Cue: strings.TrimSuffix(entry.Name(), ".ogg"), Filenames: []string{path.Join(directory, entry.Name())}, Samples: make([][]byte, 1)}
		setup(s)
		ap.availableSounds[s.Cue] = s
	}
}

func slicesContains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (ap *Player) MusicTracks() []string { return ap.musicTracks }
func (ap *Player) CurrentMusic() string  { return ap.currentMusic }

// PlayMusic switches the background music to a registered track and reports whether it knows the track.
func (ap *Player) PlayMusic(cue string) bool {
	if !slicesContains(ap.musicTracks, cue) || ap.disablePlayback {
		return false
	}
	if cue == ap.currentMusic && ap.IsCuePlaying(cue) {
		return true
	}
	ap.Stop(ap.currentMusic)
	ap.currentMusic = cue
	ap.StartLoopStream(cue)
	return true
}

// RegisterSoundEvents registers every event in a sounds rec file (assets/data/sounds.rec, edited with
// ./remap_sounds.sh) as a randomized cue named after its internal_name, one "sound:" field per file relative to
// assets/. Missing files are skipped when a sample is picked at play time (see Sound.Next); an event left without
// files isn't registered at all, so HasCue fallbacks still work.
func (ap *Player) RegisterSoundEvents(recFile string) {
	ap.registerEvents(recFile, func(s *Sound) { s.IsRandomized = true })
}

// RegisterVoiceEvents is RegisterSoundEvents for assets/data/voices.rec (./remap_voices.sh): D1's speech ids
// (_sfx_id, e.g. TSFX_BMAID1 or PS_WARR8), played at the voice volume.
func (ap *Player) RegisterVoiceEvents(recFile string) {
	ap.registerEvents(recFile, func(s *Sound) { s.IsRandomized, s.IsVoice = true, true })
}

// RegisterMusicEvents is RegisterSoundEvents for assets/data/music.rec (./remap_music.sh): the area themes
// (town, cathedral, catacombs, caves, hell), looped and listed in the Music Player.
func (ap *Player) RegisterMusicEvents(recFile string) {
	ap.registerEvents(recFile, func(s *Sound) {
		s.IsLooping = true
		if !slicesContains(ap.musicTracks, s.Cue) {
			ap.musicTracks = append(ap.musicTracks, s.Cue)
		}
	})
}

func (ap *Player) registerEvents(recFile string, setup func(s *Sound)) {
	f, err := ap.files.Open(recFile)
	if err != nil {
		println("ERR: Sound events not found: " + recFile)
		return
	}
	defer f.Close()
	for _, rec := range recfile.Read(f) {
		var cue string
		var filenames []string
		for _, field := range rec {
			switch field.Name {
			case "internal_name":
				cue = field.Value
			case "sound":
				filenames = append(filenames, path.Join("assets", field.Value))
			}
		}
		if len(filenames) == 0 { // an event nobody has assigned a file to yet is simply silent
			continue
		}
		s := &Sound{Cue: cue, Filenames: filenames, Samples: make([][]byte, len(filenames))}
		setup(s)
		ap.availableSounds[cue] = s
	}
}
