package gui

import (
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"rpg/common"
	"rpg/util"
)

// openPauseMenu is ESC during the game. The game is turn-based, so nothing moves while a menu is open.
// Saving and loading are left out of a permadeath game, which has one life and one autosave by definition.
func (u *UIController) openPauseMenu() {
	items := []util.MenuItem{{MainText: "Resume"}}
	if !u.context.IsPermadeath() {
		items = append(items,
			util.MenuItem{MainText: "Save game", Action: func() { u.context.OpenSaveMenu(func() {}) }},
			util.MenuItem{MainText: "Load game", Action: func() { u.context.OpenLoadMenu(func() {}) }})
	}
	quit := "Quit game"
	if u.context.QuitSaves != nil && u.context.QuitSaves() {
		quit = "Save & Quit"
	}
	u.OpenMenu(append(items, []util.MenuItem{
		{MainText: "Video", Action: u.openVideoMenu},
		{MainText: "Audio", Action: u.openAudioMenu},
		{MainText: "Auto explore", Action: u.openAutoExploreMenu},
		{MainText: "Back to main menu", Action: u.context.BackToMainMenu},
		{MainText: quit, Action: u.QuitGame},
	}...))
}

// reopen rebuilds the menu on top in place, so its labels show the new state without stacking a second copy.
func (u *UIController) reopen(open func()) {
	u.PopWidget()
	open()
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
			u.reopen(u.openVideoMenu)
		}},
		{MainText: "Back", Action: u.PopWidget},
	})
}

// openAutoExploreMenu toggles auto-explore behavior, saved to config.rec.
func (u *UIController) openAutoExploreMenu() {
	u.OpenMenu(append(common.AutoExploreSettings(u.configuration, func() { u.reopen(u.openAutoExploreMenu) }), util.MenuItem{MainText: "Back", Action: u.PopWidget}))
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
			u.reopen(u.openAudioMenu)
		}}
	}
	u.OpenMenu([]util.MenuItem{
		slider("Sound Effects", ap.GetSoundVolume, ap.SetSoundVolume),
		slider("Music", ap.GetMusicVolume, ap.SetMusicVolume),
		slider("Voice", ap.GetVoiceVolume, ap.SetVoiceVolume),
		{MainText: "Music Player", Action: u.openMusicPlayer},
		{MainText: "Back", Action: u.PopWidget},
	})
}

// openMusicPlayer lists the tracks in assets/ambience, assets/music and music.rec; selecting one plays it.
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
			u.reopen(u.openMusicPlayer)
		}})
	}
	if len(items) == 0 {
		items = append(items, util.MenuItem{MainText: "No tracks yet: add .ogg files to assets/ambience", Action: func() { u.reopen(u.openMusicPlayer) }})
	}
	u.OpenMenu(append(items, util.MenuItem{MainText: "Back", Action: u.PopWidget}))
}

func (u *UIController) saveVolumes() {
	ap := u.context.AudioPlayer
	u.configuration.SoundVolume, u.configuration.MusicVolume, u.configuration.VoiceVolume = ap.GetSoundVolume(), ap.GetMusicVolume(), ap.GetVoiceVolume()
	if err := u.configuration.SaveVolumes(common.ConfigFile); err != nil {
		println("ERR: could not save the volume settings: " + err.Error())
	}
}
