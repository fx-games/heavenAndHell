package main

import (
	"rpg/src/cmd/internal/mpq"

	"encoding/binary"
	"testing"

	"rpg/src/d1"
)

// TestMPQ reads each level type's tile tables and checks they agree: every megatile points at pieces the
// .SOL and .MIN describe, which fails if decompression or decryption is off.
func TestMPQ(t *testing.T) {
	a, err := mpq.DiabDat()
	if err != nil {
		t.Skip(err)
	}
	for lt, dir := range []string{`Levels\L1Data\L1`, `Levels\L2Data\L2`, `Levels\L3Data\L3`, `Levels\L4Data\L4`} {
		til, err1 := a.ReadFile(dir + ".TIL")
		sol, err2 := a.ReadFile(dir + ".SOL")
		min, err3 := a.ReadFile(dir + ".MIN")
		if err1 != nil || err2 != nil || err3 != nil {
			t.Fatal(err1, err2, err3)
		}
		blocks := 20
		if lt == 3 {
			blocks = 32
		}
		if len(min) != len(sol)*blocks {
			t.Errorf("%s: %d pieces in .SOL, .MIN is %d bytes", dir, len(sol), len(min))
		}
		for i := 0; i+2 <= len(til); i += 2 {
			if p := int(binary.LittleEndian.Uint16(til[i:])); p >= len(sol) {
				t.Fatalf("%s.TIL: piece %d of %d", dir, p, len(sol))
			}
		}
	}
	b, err := a.ReadFile(`Levels\L1Data\SklKng2.DUN`)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := binary.LittleEndian.Uint16(b), binary.LittleEndian.Uint16(b[2:]); w != 37 || h != 25 {
		t.Errorf("SklKng2.DUN: %dx%d", w, h)
	}
}

// TestImportRoundTrip re-runs the import into a scratch directory. ImportLevels parses back everything it writes
// and compares it to the MPQ's bytes, so this fails if the text encoding ever stops being lossless.
func TestImportRoundTrip(t *testing.T) {
	a, err := mpq.DiabDat()
	if err != nil {
		t.Skip(err)
	}
	if err := d1.ImportLevels(a.ReadFile, t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
