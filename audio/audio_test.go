package audio

import (
	"io/fs"
	"os"
	"testing"

	"rpg/files"
)

// modFiles is the file tree the game reads, on H&H: its audio is in git (D1's is bring-your-own).
func modFiles() fs.ReadDirFS {
	files.SetFileSystem(os.DirFS("../files").(files.FileSystem))
	files.SelectMod("heavenandhell")
	return files.GetFileSystem()
}

// An event with a missing file keeps its other files and plays a random loaded one of them at run time.
func TestRandomCueSkipsMissingFiles(t *testing.T) {
	ap := NewAudioPlayer(modFiles(), SampleRate44100)
	s := &Sound{
		Cue:          "test",
		Filenames:    []string{"assets/sfx/missing.ogg", "assets/sfx/melee_attack/melee_attack_1.ogg", "assets/sfx/melee_attack/melee_attack_2.ogg"},
		Samples:      make([][]byte, 3),
		IsRandomized: true,
	}
	if !s.Load(ap) {
		t.Fatal("cue with one missing file did not load")
	}
	seen := map[int]bool{}
	for i := 0; i < 200; i++ {
		b := s.Next()
		if len(b) == 0 {
			t.Fatal("picked a missing sample")
		}
		seen[len(b)] = true
	}
	if len(seen) < 2 {
		t.Fatalf("expected both loaded samples to be picked, got %d distinct", len(seen))
	}
}

func TestPannedScalesEachChannel(t *testing.T) {
	// one frame: left 1000, right -1000
	b := []byte{0xe8, 0x03, 0x18, 0xfc}
	out := panned(b, 0.5, 1)
	l, r := int16(uint16(out[0])|uint16(out[1])<<8), int16(uint16(out[2])|uint16(out[3])<<8)
	if l != 500 || r != -1000 {
		t.Fatalf("got %d, %d", l, r)
	}
	if &panned(b, 1, 1)[0] != &b[0] {
		t.Fatal("unpanned samples were copied")
	}
}

// music.rec holds the area themes levels.rec's music fields name, town wired up to the mod's music/town.ogg. A theme with
// no file assigned stays unregistered, so the current track keeps playing instead.
func TestAreaMusicRegistered(t *testing.T) {
	ap := NewAudioPlayer(modFiles(), SampleRate44100)
	ap.RegisterMusicEvents("assets/data/music.rec")
	if !slicesContains(ap.MusicTracks(), "town") {
		t.Fatalf("no town track, got %v", ap.MusicTracks())
	}
	if !ap.availableSounds["town"].IsLooping {
		t.Fatal("town music does not loop")
	}
	for _, theme := range []string{"cathedral", "catacombs", "caves", "hell"} {
		if _, ok := ap.availableSounds[theme]; ok {
			continue // someone assigned it a track, fine
		}
		if slicesContains(ap.MusicTracks(), theme) {
			t.Fatalf("%s is listed as a track but has no sound", theme)
		}
	}
}
