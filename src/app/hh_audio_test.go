package app

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Heaven & Hell never plays Diablo's audio (AGENTS.md, Audio): its recs point only into its own folders, and none
// of its files is a copy of one in D1's diablo_sfx/ or diablo_music/ (checked when those are on disk).
func TestHHUsesNoDiabloAudio(t *testing.T) {
	root := "files/mods/" // TestMain runs from the repo root
	for _, rec := range []string{"sounds.rec", "music.rec", "voices.rec"} {
		b, err := os.ReadFile(root + "heavenandhell/data/" + rec)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			p, ok := strings.CutPrefix(line, "sound: ")
			if !ok {
				continue
			}
			if dir, _, _ := strings.Cut(p, "/"); !map[string]bool{"sfx": true, "ang_sfx": true, "music": true, "ambience": true, "voice": true}[dir] || strings.Contains(p, "..") {
				t.Errorf("%s: %q is outside H&H's own audio folders", rec, p)
			}
		}
	}
	hashes := map[[32]byte]string{}
	for _, dir := range []string{"diablo_sfx", "diablo_music"} {
		filepath.WalkDir(root+"d1/"+dir, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				if b, err := os.ReadFile(p); err == nil {
					hashes[sha256.Sum256(b)] = p
				}
			}
			return nil
		})
	}
	if len(hashes) == 0 {
		t.Skip("no Diablo audio on disk to compare against")
	}
	filepath.WalkDir(root+"heavenandhell", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".ogg") && !strings.HasSuffix(p, ".wav") {
			return nil
		}
		if b, err := os.ReadFile(p); err == nil {
			if orig, dup := hashes[sha256.Sum256(b)]; dup {
				t.Errorf("%s is Diablo's %s", p, orig)
			}
		}
		return nil
	})
}
