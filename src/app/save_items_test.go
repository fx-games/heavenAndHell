package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"rpg/src/common"
	"rpg/src/game"
	"rpg/src/geometry"
	"rpg/src/gridmap"
)

// A save must come back exactly: saving, loading and saving again writes the same file, and every item points at
// its mod's own base row again, not at a copy, so its icon, rules and name come from the loaded mod.
func TestSaveLoadRoundTripsItems(t *testing.T) {
	savedAt := regexp.MustCompile(`"SavedAt":"[^"]*"`)
	for _, mod := range []string{"d1", "heavenandhell"} {
		t.Run(mod, func(t *testing.T) {
			game.SaveDir = filepath.Join(t.TempDir(), "saves")
			g := newModGame(t, mod, &slotUI{DummyUI: NewDummyUI(), choose: 1})
			g.NewGame(g.GetGameData().ClassDefinitions[0], false, "Aidan")
			state, f := g.GetWorldState(), g.GetFactory()
			state.MapTransition(gridmap.Transition{TargetMap: levelAt(state, 1), TargetLocation: "ladder_up"})
			// something on the floor too, next to the hero
			p := state.GetPlayer().MapPosition()
			state.AddItemToMapWithDistribution(f.NewRolledItem(f.Generator.PlainItem(&f.Generator.D.Items[1])), p.Add(geometry.Point{X: 1}))

			// as after any turn: the hero's sight is up to date when the game saves
			g.GetActionBroker().Execute(common.NewUpdateFoVAction(common.AbstractCause("test"), state.GetPlayer()))

			read := func() string {
				t.Helper()
				g.SaveGame()
				b, err := os.ReadFile(filepath.Join(game.SaveDir, mod, "slot1.json"))
				if err != nil {
					t.Fatal(err)
				}
				return savedAt.ReplaceAllString(string(b), "")
			}
			first := read()
			if want := `"Build":"` + common.BuildHash() + `"`; !strings.Contains(first, want) || len(common.BuildHash()) != 8 {
				t.Errorf("the save doesn't name the binary that wrote it (want %s)", want)
			}
			if err := g.LoadGame(); err != nil {
				t.Fatalf("load: %v", err)
			}
			second := read()
			if second != first {
				i := 0
				for i < len(first) && i < len(second) && first[i] == second[i] {
					i++
				}
				lo := max(0, i-120)
				if k := strings.LastIndex(first[:i], `":`); k > 0 {
					t.Logf("in field %s", first[strings.LastIndex(first[:k], `"`):k+1])
				}
				t.Errorf("saving a loaded game writes a different file, from byte %d:\n  before %s\n  after  %s", i,
					first[lo:min(len(first), i+80)], second[lo:min(len(second), i+80)])
			}
			d := f.Generator.D
			items := append(state.GetPlayer().GetInventory().Items(), state.GetMap().Items()...)
			if len(items) < 2 {
				t.Fatalf("only %d items to check", len(items))
			}
			for _, it := range items {
				roll := it.Roll()
				if roll == nil {
					continue
				}
				if roll.Base == nil || roll.Base != d.ItemsByName[roll.BaseName()] {
					t.Errorf("%s: base not relinked to the mod's row after the load", it.Name())
				}
			}
		})
	}
}

// Starting the game with a save it can't load (here one written before items saved their base by name) skips
// that save: the start menu comes up on the newest save that does load, nothing panics, and the broken one is
// neither loaded nor offered.
func TestStartupSkipsASaveThatDoesNotLoad(t *testing.T) {
	game.SaveDir = filepath.Join(t.TempDir(), "saves")
	g := newModGame(t, "heavenandhell", &slotUI{DummyUI: NewDummyUI(), choose: 1})
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "Aidan") // a good save in slot 1
	g.SaveGame()
	old, err := os.ReadFile("src/app/testdata/old_save_hh.json") // TestMain runs from the repo root
	if err != nil {
		t.Fatal(err)
	}
	// slot 2 is the broken one, and the newest
	later, _ := time.Now().Add(time.Hour).MarshalJSON()
	old = regexp.MustCompile(`"SavedAt":"[^"]*"`).ReplaceAll(old, []byte(`"SavedAt":`+string(later)))
	if err := os.WriteFile(filepath.Join(game.SaveDir, "heavenandhell", "slot2.json"), old, 0o644); err != nil {
		t.Fatal(err)
	}

	started := newModGame(t, "heavenandhell", &slotUI{DummyUI: NewDummyUI(), choose: 1})
	started.ShowStartMenu()
	if got := started.SaveSlot(); got != 1 {
		t.Fatalf("startup continued slot %d, want the loadable slot 1", got)
	}
	if err := started.LoadSlot(2); err == nil {
		t.Fatal("the broken save loaded")
	}
}
