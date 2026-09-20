package d1

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"testing"
)

// goldenFile pins what CreateLevel builds. It is generated from DIABDAT.MPQ and must keep matching after the level
// data moved into assets/data/d1/levels: a one-tile encoding slip shows up here as a changed hash.
const goldenFile = "testdata/levels.golden"

func hashLevel(l *Level) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d %d %d %v %v %v %v\n", l.Dlvl, l.Type, l.SetLevel, l.View, l.SetPiece, l.DiabloQuads, l.Triggers)
	fmt.Fprintf(h, "%v %v\n", l.Dungeon, l.PDungeon)
	fmt.Fprintf(h, "%v %v %v\n", l.Piece, l.Trans, l.Flags)
	fmt.Fprintf(h, "%v\n", l.SOL)
	for _, m := range l.Monsters {
		if m == nil {
			io.WriteString(h, "nil\n")
			continue
		}
		fmt.Fprintf(h, "%+v\n", *m)
	}
	for _, o := range l.Objects {
		fmt.Fprintf(h, "%+v\n", *o)
	}
	for _, it := range l.Items {
		fmt.Fprintf(h, "%d %d %d %+v\n", it.X, it.Y, it.SelFlag, *it.Item)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// goldenLevels builds the same levels TestCreateLevels does and returns "game/dlvl/setlevel hash" lines.
func goldenLevels(t *testing.T, d *Data) []string {
	t.Helper()
	var out []string
	for game := int32(1); game <= 6; game++ {
		seeds := LevelSeeds(game * 7919)
		q := InitQuests(seeds)
		gnr := NewGenerator(d, game)
		type lv struct{ dlvl, sl int }
		levels := make([]lv, 0, 20)
		for dlvl := 1; dlvl <= 16; dlvl++ {
			levels = append(levels, lv{dlvl, SLNone})
		}
		levels = append(levels, lv{3, SLSkelKing}, lv{6, SLBoneChamb}, lv{2, SLPoisonWater}, lv{15, SLVileBetrayer})
		for _, l := range levels {
			lvl, err := gnr.CreateLevel(&q, seeds, l.dlvl, l.sl, EntryMain)
			if err != nil {
				t.Fatalf("game %d level %d/%d: %v", game, l.dlvl, l.sl, err)
			}
			out = append(out, fmt.Sprintf("%d/%d/%d %s", game, l.dlvl, l.sl, hashLevel(lvl)))
		}
	}
	sort.Strings(out)
	return out
}

func TestGoldenLevels(t *testing.T) {
	d, err := Load(os.DirFS("../files"))
	if err != nil {
		t.Fatal(err)
	}
	got := goldenLevels(t, d)
	want, err := os.ReadFile(goldenFile)
	if os.IsNotExist(err) {
		if os.Getenv("D1_WRITE_GOLDEN") == "" {
			t.Skipf("%s missing; run with D1_WRITE_GOLDEN=1 to create it", goldenFile)
		}
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenFile, []byte(strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s (%d levels)", goldenFile, len(got))
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	wantLines := strings.Split(strings.TrimSpace(string(want)), "\n")
	if len(got) != len(wantLines) {
		t.Fatalf("%d levels, golden has %d", len(got), len(wantLines))
	}
	for i := range got {
		if got[i] != wantLines[i] {
			t.Errorf("level %s changed:\n got %s\nwant %s", strings.Fields(got[i])[0], got[i], wantLines[i])
		}
	}
}
