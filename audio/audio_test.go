package audio

import (
	"io/fs"
	"os"
	"testing"
)

// An event with a missing file keeps its other files and plays a random loaded one of them at run time.
func TestRandomCueSkipsMissingFiles(t *testing.T) {
	ap := NewAudioPlayer(os.DirFS("../files").(fs.ReadDirFS), SampleRate44100)
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
