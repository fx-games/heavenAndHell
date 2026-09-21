package common

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveModWritesIntoFirstRecord(t *testing.T) {
	f := filepath.Join(t.TempDir(), "config.rec")
	os.WriteFile(f, []byte("SoundVolume: 0.5\n\n"), 0o644)
	if err := SaveMod(f, "heavenandhell"); err != nil {
		t.Fatal(err)
	}
	if got := NewConfigurationFromRecFile(f).Mod; got != "heavenandhell" {
		t.Fatalf("Mod = %q", got)
	}
	SaveMod(f, "d1")
	if got := NewConfigurationFromRecFile(f).Mod; got != "d1" {
		t.Fatalf("Mod = %q after replace", got)
	}
}
