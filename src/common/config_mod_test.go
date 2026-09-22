package common

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveModWritesIntoFirstRecord(t *testing.T) {
	f := filepath.Join(t.TempDir(), "config.rec")
	os.WriteFile(f, []byte("%rec: default\n\nSoundVolume: 0.5\n\n"), 0o644) // the shape of the real config.rec
	if err := SaveMod(f, "heavenandhell"); err != nil {
		t.Fatal(err)
	}
	if c := NewConfigurationFromRecFile(f); c.Mod != "heavenandhell" || c.SoundVolume != 0.5 {
		t.Fatalf("Mod = %q, SoundVolume = %v: the field split the record", c.Mod, c.SoundVolume)
	}
	SaveMod(f, "d1")
	if got := NewConfigurationFromRecFile(f).Mod; got != "d1" {
		t.Fatalf("Mod = %q after replace", got)
	}
}

// A mod's data/mod.rec fixes a setting over config.rec: D1 (1.09) never autosaves on a level change.
func TestModFixesAutosave(t *testing.T) {
	cfg := Configuration{AutosaveOnLevelChange: true}
	if cfg.AutosavesOnLevelChange() {
		t.Fatal("d1's mod.rec should turn the level-change autosave off")
	}
	if _, fixed := ModSetting("AutosaveOnLevelChange"); !fixed {
		t.Fatal("the setting should be locked for the player")
	}
}
