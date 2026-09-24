package app

import (
	"strings"
	"testing"

	"rpg/src/common"
	"rpg/src/gridmap"
	"rpg/src/stats"
)

// H&H borrows D1's monster families (their brains included) for its dungeon's variety: they must spawn on its
// levels and take turns under their D1 brains without the D1 mod loaded.
func TestHHBorrowedFamiliesSpawnAndAct(t *testing.T) {
	if testing.Short() {
		t.Skip("walks sixteen levels")
	}
	g := newHeavenAndHellGame(t, NewDummyUI())
	g.SeedRandom(1)
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	state := g.GetWorldState()
	player := state.GetPlayer()
	player.Hero().MaxHPBase += 100000 << 6 // stand among them without dying
	common.SyncHero(player)
	seen, hurt := map[string]bool{}, map[string]bool{}
	hp := player.GetStats().GetResource(stats.Health)
	for dlvl := 1; dlvl <= 16; dlvl++ {
		state.MapTransition(gridmap.Transition{TargetMap: levelAt(state, dlvl), TargetLocation: "ladder_up"})
		m := state.GetMap()
		for _, a := range m.Actors() {
			mon := a.Monster()
			if mon == nil || mon.Data.AI == "" || seen[mon.Data.ID] || !a.IsAlive() {
				continue
			}
			seen[mon.Data.ID] = true
			free := m.GetFreeCardinalNeighbors(a.MapPosition())
			if len(free) == 0 {
				continue
			}
			m.MoveActor(player, free[0])
			g.GetActionBroker().Execute(common.NewUpdateFoVAction(common.AbstractCause("test"), player))
			for turn := 0; turn < 30 && a.IsAlive() && !hurt[mon.Data.ID]; turn++ {
				hp.SetToMax()
				g.GetActions().EndTurn(10)
				for i := 0; i < 50; i++ {
					g.GetActionBroker().Tick()
				}
				hurt[mon.Data.ID] = !hp.IsAtMax()
			}
		}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	if len(seen) < 20 {
		t.Errorf("only %d borrowed monster types spawned in sixteen levels: %s", len(seen), strings.Join(ids, " "))
	}
	var harmless []string
	for id := range seen {
		if !hurt[id] {
			harmless = append(harmless, id)
		}
	}
	t.Logf("%d types spawned; never struck in 30 turns: %s", len(seen), strings.Join(harmless, " "))
	if len(harmless) > len(seen)/3 { // a stone saint sleeps, an inquisitor blinks away: some may not strike in 30 turns
		t.Errorf("%d of %d borrowed types never hurt a hero standing next to them for 30 turns: %s", len(harmless), len(seen), strings.Join(harmless, " "))
	}
}
