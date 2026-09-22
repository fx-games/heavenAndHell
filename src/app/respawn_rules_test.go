package app

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"rpg/src/common"
	"rpg/src/game"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/stats"
)

// levelState is everything a save must bring back about the current level: layout, monsters, items, doors, decals.
func levelState(state *common.State) string {
	m := state.GetMap()
	var b strings.Builder
	for y := 0; y < m.MapSize().Y; y++ {
		for x := 0; x < m.MapSize().X; x++ {
			p := geometry.Point{X: x, Y: y}
			fmt.Fprintf(&b, "%d,", m.GetCell(p).TileType.DefinedIcon)
			if d, ok := m.GetDecal(p); ok {
				fmt.Fprintf(&b, "d%d,", d)
			}
		}
	}
	var lines []string
	for _, a := range m.Actors() {
		if a != state.GetPlayer() {
			lines = append(lines, fmt.Sprintf("%s %v %d", a.GetInternalName(), a.MapPosition(), a.GetStats().GetResource(stats.Health).Value()))
		}
	}
	for _, it := range m.Items() {
		lines = append(lines, fmt.Sprintf("%s %v", it.Name(), it.MapPosition()))
	}
	for _, o := range m.Objects() {
		if d, ok := o.(*common.Door); ok {
			lines = append(lines, fmt.Sprintf("door %v %v", d.MapPosition(), d.IsClosed()))
		}
	}
	slices.Sort(lines)
	return b.String() + strings.Join(lines, "\n")
}

func monsters(state *common.State) (out []*common.Actor) {
	for _, a := range state.GetMap().Actors() {
		if a != state.GetPlayer() {
			out = append(out, a)
		}
	}
	return
}

func killAll(state *common.State) {
	for _, a := range monsters(state) {
		state.GetMap().RemoveActor(a)
	}
}

func modGame(t *testing.T, mod string, rules common.DungeonRules) (*game.GameInstance, *common.State) {
	t.Helper()
	game.SaveDir = filepath.Join(t.TempDir(), "saves")
	g := newModGame(t, mod, &slotUI{DummyUI: NewDummyUI(), choose: 1})
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "Aidan")
	state := g.GetWorldState()
	state.SetDungeonRules(rules)
	return g, state
}

var (
	toTown = gridmap.Transition{TargetMap: "town", TargetLocation: "to_cathedral"}
	down   = gridmap.Transition{TargetMap: "cathedral_1", TargetLocation: "ladder_up"}
)

// mod.rec's Respawn: kill a level's monsters, leave for town and come back.
func TestRespawnRules(t *testing.T) {
	for _, mod := range []string{"d1", "heavenandhell"} {
		for _, tc := range []struct {
			rules common.DungeonRules
			away  uint64 // turns spent in town
			want  func(before int) int
		}{
			{common.DungeonRules{Respawn: "disabled"}, 0, func(int) int { return 0 }},
			{common.DungeonRules{Respawn: "enabled"}, 0, func(b int) int { return b }},
			{common.DungeonRules{Respawn: "town_visits", RespawnCount: 2}, 0, func(int) int { return 2 }},
			{common.DungeonRules{Respawn: "timed", RespawnCount: 1, Turns: 1000}, 999, func(int) int { return 0 }},
			{common.DungeonRules{Respawn: "timed", RespawnCount: 1, Turns: 1000}, 3000, func(int) int { return 3 }},
		} {
			_, state := modGame(t, mod, tc.rules)
			state.MapTransition(down)
			before := len(monsters(state))
			if before < 4 {
				t.Fatalf("%s: level 1 has %d monsters", mod, before)
			}
			killAll(state)
			state.MapTransition(toTown)
			c := state.DungeonClock()
			c.Turn += tc.away
			state.SetDungeonClock(c)
			state.MapTransition(down)
			if got, want := len(monsters(state)), tc.want(before); got < want || got > want+8 { // a pack brings its companions
				t.Errorf("%s %+v: %d monsters back, want %d", mod, tc.rules, got, want)
			}
			for _, a := range monsters(state) {
				if a.IsUnique() {
					t.Errorf("%s %+v: unique %s respawned with RespawnUniques: none", mod, tc.rules, a.Name())
				}
			}
		}
	}
}

// A save taken anywhere loads back as exactly that level, also after respawns and on a level that was rebuilt.
func TestSaveLoadKeepsTheLevel(t *testing.T) {
	for _, mod := range []string{"d1", "heavenandhell"} {
		for _, rules := range []common.DungeonRules{{}, {Respawn: "enabled", Forget: false}, {Forget: true}} {
			g, state := modGame(t, mod, rules)
			state.MapTransition(down)
			for _, a := range monsters(state)[:len(monsters(state))/2] {
				state.GetMap().RemoveActor(a)
			}
			state.MapTransition(toTown)
			state.MapTransition(down) // respawned, or rebuilt as its second generation
			ms := monsters(state)
			state.GetMap().RemoveActor(ms[0]) // one dies
			if len(ms) > 1 && ms[1].GetStats().GetResource(stats.Health).Value() > 1 {
				ms[1].GetStats().GetResource(stats.Health).Decrease(1)
			}
			for _, o := range state.GetMap().Objects() {
				if d, ok := o.(*common.Door); ok {
					d.Open()
					break
				}
			}
			state.GetMap().AddDecal(state.GetPlayer().MapPosition(), 7)
			want := levelState(state)
			g.SaveGame()
			if err := g.LoadGame(); err != nil {
				t.Fatal(err)
			}
			state = g.GetWorldState()
			state.SetDungeonRules(rules)
			if got := levelState(state); got != want {
				t.Errorf("%s %+v: the loaded level differs from the saved one:%s", mod, rules, diff(want, got))
			}
		}
	}
}

// PersistLevels false builds a new level on each visit; PersistQuestLevels keeps a quest's level as it was.
func TestPersistLevels(t *testing.T) {
	_, state := modGame(t, "heavenandhell", common.DungeonRules{Forget: true, Respawn: "enabled"})
	state.MapTransition(down)
	first := levelState(state)
	state.MapTransition(toTown)
	state.MapTransition(down)
	if levelState(state) == first {
		t.Error("PersistLevels false kept the level")
	}

	quest := gridmap.Transition{TargetMap: "catacombs_4", TargetLocation: "to_town"}
	state.MapTransition(quest)
	killAll(state)
	kept := levelState(state)
	state.MapTransition(gridmap.Transition{TargetMap: "town", TargetLocation: "to_catacombs"})
	state.MapTransition(quest)
	if levelState(state) != kept {
		t.Error("a quest level was rebuilt or respawned")
	}
}

// diff names the first place two levelStates part.
func diff(a, b string) string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	if al[0] != bl[0] {
		i := 0
		for i < len(al[0]) && i < len(bl[0]) && al[0][i] == bl[0][i] {
			i++
		}
		return fmt.Sprintf(" layout at char %d", i)
	}
	var out []string
	for _, l := range al[1:] {
		if !slices.Contains(bl, l) {
			out = append(out, "\n  lost: "+l)
		}
	}
	for _, l := range bl[1:] {
		if !slices.Contains(al, l) {
			out = append(out, "\n  new: "+l)
		}
	}
	return strings.Join(out, "")
}
