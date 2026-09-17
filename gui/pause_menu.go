package gui

import (
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"rpg/common"
	"rpg/util"
)

// openPauseMenu is ESC during the game. The game is turn-based, so nothing moves while a menu is open.
func (u *UIController) openPauseMenu() {
	u.OpenMenu([]util.MenuItem{
		{MainText: "Resume"},
		{MainText: "Video", Action: u.openVideoMenu},
		{MainText: "Audio", Action: u.openAudioMenu},
		{MainText: "Auto explore", Action: u.openAutoExploreMenu},
		{MainText: "Gameplay", Action: u.openGameplayMenu},
		{MainText: "Back to main menu", Action: u.context.BackToMainMenu},
		{MainText: "Quit game", Action: u.QuitGame},
	})
}

func (u *UIController) toggleFullscreen() {
	u.SetFullscreen(!ebiten.IsFullscreen())
	u.configuration.Fullscreen = u.isFullScreen
	u.configuration.SaveToRecFile(common.ConfigFile)
}

func (u *UIController) SetFullscreen(on bool) {
	u.isFullScreen = on
	ebiten.SetFullscreen(on)
	u.OnScreenSizeChanged()
}

// openVideoMenu holds display settings.
func (u *UIController) openVideoMenu() {
	state := "Off"
	if u.isFullScreen { // ebiten.IsFullscreen may lag a frame behind SetFullscreen
		state = "On"
	}
	u.OpenMenu([]util.MenuItem{
		{MainText: "Fullscreen: " + state, Action: func() {
			u.toggleFullscreen()
			u.openVideoMenu()
		}},
		{MainText: "Back", Action: u.openPauseMenu},
	})
}

// openAutoExploreMenu toggles auto-explore behavior, saved to config.rec.
func (u *UIController) openAutoExploreMenu() {
	u.OpenMenu(append(common.AutoExploreSettings(u.configuration, u.openAutoExploreMenu), util.MenuItem{MainText: "Back", Action: u.openPauseMenu}))
}

// openGameplayMenu holds miscellaneous gameplay toggles, saved to config.rec.
func (u *UIController) openGameplayMenu() {
	u.OpenMenu(append(common.GameplaySettings(u.configuration, u.openGameplayMenu), util.MenuItem{MainText: "Back", Action: u.openPauseMenu}))
}

// openAudioMenu: volume sliders (left/right change them by 10%, Enter steps up and wraps), saved to config.rec,
// and the music player.
func (u *UIController) openAudioMenu() {
	ap := u.context.AudioPlayer
	slider := func(name string, get func() float64, set func(float64)) util.MenuItem {
		label := func() string { return fmt.Sprintf("%s: < %d%% >", name, int(math.Round(get()*100))) }
		step := func(d float64) func() string {
			return func() string {
				set(math.Round((get()+d)*10) / 10)
				u.saveVolumes()
				return label()
			}
		}
		return util.MenuItem{MainText: label(), ActionLeft: step(-0.1), ActionRight: step(0.1), Action: func() {
			if get() >= 1 {
				set(0)
				u.saveVolumes()
			} else {
				step(0.1)()
			}
			u.openAudioMenu()
		}}
	}
	diabloAudioLabel := func() string {
		state := "Off"
		if u.configuration.DiabloAudioEnabled {
			state = "On"
		}
		return fmt.Sprintf("Diablo Audio (restart to apply): %s", state)
	}
	u.OpenMenu([]util.MenuItem{
		slider("Sound Effects", ap.GetSoundVolume, ap.SetSoundVolume),
		slider("Music", ap.GetMusicVolume, ap.SetMusicVolume),
		slider("Voice", ap.GetVoiceVolume, ap.SetVoiceVolume),
		{MainText: diabloAudioLabel(), Action: func() {
			u.configuration.DiabloAudioEnabled = !u.configuration.DiabloAudioEnabled
			u.configuration.SaveToRecFile(common.ConfigFile)
			u.openAudioMenu()
		}},
		{MainText: "Music Player", Action: u.openMusicPlayer},
		{MainText: "Back", Action: u.openPauseMenu},
	})
}

// openMusicPlayer lists the tracks in assets/ambience and assets/diablo_music; selecting one plays it.
func (u *UIController) openMusicPlayer() {
	ap := u.context.AudioPlayer
	var items []util.MenuItem
	for _, track := range ap.MusicTracks() {
		title := track
		if track == ap.CurrentMusic() {
			title = "> " + track + " <"
		}
		items = append(items, util.MenuItem{MainText: title, Action: func() {
			ap.PlayMusic(track)
			u.openMusicPlayer()
		}})
	}
	if len(items) == 0 {
		items = append(items, util.MenuItem{MainText: "No tracks yet: add .ogg files to assets/ambience", Action: u.openMusicPlayer})
	}
	u.OpenMenu(append(items, util.MenuItem{MainText: "Back", Action: u.openAudioMenu}))
}

func (u *UIController) saveVolumes() {
	ap := u.context.AudioPlayer
	u.configuration.SoundVolume, u.configuration.MusicVolume, u.configuration.VoiceVolume = ap.GetSoundVolume(), ap.GetMusicVolume(), ap.GetVoiceVolume()
	if err := u.configuration.SaveVolumes(common.ConfigFile); err != nil {
		println("ERR: could not save the volume settings: " + err.Error())
	}
}
