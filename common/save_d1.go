package common

import (
	"maps"
	"rpg/d1"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
)

// Decision 11: a level is rebuilt from its seed, then this snapshot restores what changed.
type SavedMonster struct {
	HP, X, Y int
	Type     int             // only for monsters that objects spawned (not part of the generated level) and corpses
	State    *D1MonsterState // nil in saves from before AI state was kept
	Status   map[string]int  `json:",omitempty"` // status effects (Stone Curse, ...) and their turns left
}

type SavedItem struct {
	Item d1.Item
	X, Y int
}

type LevelSnapshot struct {
	Monsters map[int]SavedMonster // by spawn index; missing = killed
	Items    []SavedItem          // D1 floor items
	Used     []int                // indices of used objects (opened chests, broken barrels, ...)
	Disarmed []int                // indices of objects whose trap the rogue disarmed
	Traps    []int                // indices of wall traps that fired or were disarmed
	Doors    []int                // indices of open doors
	Spawned  []SavedMonster       // skeletons from sarcophagi and barrels
	Corpses  []SavedMonster       // dead monsters, which scavengers feed on
	Explored []byte               // explored cells, one bit per cell row by row
	// Width is the row stride of Explored, i.e. the width of the map it was taken from. Saves from before it
	// existed used d1.LevelSize, which is what a map a D1 generator built is - but any other map (a mod's own
	// level) wrote its rows at the wrong stride and lost every cell past the first LevelSize² of them.
	Width int `json:",omitempty"`
	// RespawnMonsters makes the level forget its dead the next time it is built: Gillian's donation sets it on
	// every explored level that isn't loaded right now (State.RespawnD1Monsters).
	RespawnMonsters bool
}

func SnapshotLevel(state *State, m *gridmap.GridMap[*Actor, *Item, Object]) *LevelSnapshot {
	s := &LevelSnapshot{Monsters: map[int]SavedMonster{}}
	if lvl := state.mapGen.d1Levels[m.GetName()]; lvl != nil {
		s.Traps = d1SprungTraps(lvl)
	}
	for _, a := range m.Actors() {
		if mon := a.d1Monster; mon != nil && a.IsAlive() && !mon.Golem {
			p := a.MapPosition()
			// at least 1 HP: regeneration leaves monsters alive below 1
			state := mon.D1MonsterState
			saved := SavedMonster{HP: max(a.GetStats().GetResource(stats.Health).Value(), 1), X: p.X, Y: p.Y, Type: mon.Type, State: &state, Status: maps.Clone(a.statusEffects.AllEntries())}
			if mon.SpawnIndex < 0 {
				s.Spawned = append(s.Spawned, saved)
			} else {
				s.Monsters[mon.SpawnIndex] = saved
			}
		}
	}
	for _, a := range m.DownedActors() {
		if mon := a.d1Monster; mon != nil {
			p := a.MapPosition()
			s.Corpses = append(s.Corpses, SavedMonster{X: p.X, Y: p.Y, Type: mon.Type})
		}
	}
	size := m.MapSize()
	s.Width = size.X
	s.Explored = make([]byte, (size.X*size.Y+7)/8)
	for i := range size.X * size.Y {
		if m.IsExplored(geometry.Point{X: i % size.X, Y: i / size.X}) {
			s.Explored[i/8] |= 1 << (i % 8)
		}
	}
	for _, obj := range m.Objects() {
		if o, ok := obj.(*D1Object); ok && !o.O.Active {
			s.Used = append(s.Used, o.O.Index)
		}
	}
	for _, obj := range m.Objects() {
		switch o := obj.(type) {
		case *D1Object:
			if o.Disarmed {
				s.Disarmed = append(s.Disarmed, o.O.Index)
			}
		case *Door:
			if o.D1 != nil && o.Disarmed {
				s.Disarmed = append(s.Disarmed, o.D1.Index)
			}
			if o.D1 != nil && o.isOpen {
				s.Doors = append(s.Doors, o.D1.Index)
			}
		}
	}
	for _, it := range m.Items() {
		if it.d1 != nil {
			p := it.MapPosition()
			s.Items = append(s.Items, SavedItem{Item: *it.d1, X: p.X, Y: p.Y})
		}
	}
	return s
}

// SetResource sets a resource's current value, clamped to its range.
func SetResource(r *stats.Resource, value int) {
	r.SetToMax()
	r.Decrease(r.GetMax() - value)
}

func (s *LevelSnapshot) apply(f *Factory, m *gridmap.GridMap[*Actor, *Item, Object]) {
	used := map[int]bool{}
	for _, i := range s.Used {
		used[i] = true
	}
	disarmed := map[int]bool{}
	for _, i := range s.Disarmed {
		disarmed[i] = true
	}
	sprung := map[int]bool{}
	for _, i := range s.Traps {
		sprung[i] = true
	}
	if lvl := f.State.mapGen.d1Levels[m.GetName()]; lvl != nil {
		for _, t := range lvl.Objects {
			if (t.Kind == d1.ObjTRAPL || t.Kind == d1.ObjTRAPR) && sprung[t.Index] {
				t.Var4 = 1
				for _, o := range lvl.Objects { // Obj_Trap cleared the target's flag when the trap fired
					if o.X == t.Var1 && o.Y == t.Var2 {
						o.TrapFlag = false
					}
				}
			}
			if used[t.Index] && t.Kind >= d1.ObjTCHEST1 && t.Kind <= d1.ObjTCHEST3 { // opening set its trap off
				t.TrapFlag = false
			}
		}
	}
	open := map[int]bool{}
	for _, i := range s.Doors {
		open[i] = true
	}
	for _, obj := range m.Objects() {
		switch o := obj.(type) {
		case *D1Object:
			o.O.Active = !used[o.O.Index]
			if disarmed[o.O.Index] {
				o.Disarmed = true
				f.State.mapGen.disarmD1Object(m, o.O)
			}
		case *Door:
			if o.D1 != nil && disarmed[o.D1.Index] {
				o.Disarmed = true
				f.State.mapGen.disarmD1Object(m, o.D1)
			}
			if o.D1 != nil && open[o.D1.Index] {
				o.isOpen = true // its wall traps are restored above, so opening it doesn't fire them again
			}
		}
	}
	f.State.applyPatches(m)
	if s.RespawnMonsters { // the level keeps the monsters the generator just placed, and buries its corpses
		s.RespawnMonsters = false
		s.Monsters, s.Spawned, s.Corpses = map[int]SavedMonster{}, nil, nil
	} else {
		// take the monsters off the map first, so none finds its saved cell taken by one that hasn't moved yet
		var restore []*Actor
		var at []SavedMonster
		for _, a := range append([]*Actor(nil), m.Actors()...) {
			mon := a.d1Monster
			if mon == nil {
				continue
			}
			m.RemoveActor(a)
			if saved, alive := s.Monsters[mon.SpawnIndex]; alive {
				restore, at = append(restore, a), append(at, saved)
			}
		}
		for _, saved := range s.Spawned {
			a := f.NewD1Monster(saved.Type)
			a.d1Monster.SpawnIndex = -1
			restore, at = append(restore, a), append(at, saved)
		}
		for i, a := range restore {
			p := geometry.Point{X: at[i].X, Y: at[i].Y}
			// the cell may have been taken since the level was saved; a monster that finds nowhere at all is
			// dropped rather than left half on the map (gridmap.AddActor)
			if !canPlaceD1Monster(m, p) || m.IsActorAt(p) {
				near := freeCellsNear(m, p, 1, m.CanPlaceActorHere)
				if len(near) == 0 {
					continue
				}
				p = near[0]
			}
			SetResource(a.GetStats().GetResource(stats.Health), at[i].HP)
			if at[i].State != nil {
				a.d1Monster.D1MonsterState = *at[i].State
			}
			maps.Copy(a.statusEffects.AllEntries(), at[i].Status)
			a.SetDrawPosition(p.ToPointF())
			m.AddActor(a, p)
		}
		for _, c := range s.Corpses {
			a := f.NewD1Monster(c.Type)
			SetResource(a.GetStats().GetResource(stats.Health), 0)
			a.SetDrawPosition(geometry.Point{X: c.X, Y: c.Y}.ToPointF())
			a.SetMapPosition(geometry.Point{X: c.X, Y: c.Y})
			m.AddDownedActor(a, geometry.Point{X: c.X, Y: c.Y})
		}
	}
	// A snapshot is only ever applied to the level it was taken from, so its stride is this map's width; older
	// saves have none and were all written by a D1 generator, whose levels are LevelSize wide.
	width := s.Width
	if width == 0 {
		width = d1.LevelSize
	}
	size := m.MapSize()
	for i := range min(len(s.Explored)*8, width*size.Y) {
		if p := (geometry.Point{X: i % width, Y: i / width}); p.X < size.X && s.Explored[i/8]&(1<<(i%8)) != 0 {
			m.SetExplored(p)
		}
	}
	for _, it := range append([]*Item(nil), m.Items()...) {
		if it.d1 != nil {
			m.RemoveItem(it)
		}
	}
	for _, si := range s.Items {
		item := si.Item
		p := geometry.Point{X: si.X, Y: si.Y}
		if !m.CanPlaceItemHere(p) { // the level may have changed since the save (new terrain), or two items share a cell
			if near := freeCellsNear(m, p, 1, m.CanPlaceItemHere); len(near) > 0 {
				p = near[0]
			}
		}
		m.AddItem(f.NewD1Item(&item), p)
	}
}
