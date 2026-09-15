package common

import (
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
	Spawned  []SavedMonster       // skeletons from sarcophagi and barrels
	Corpses  []SavedMonster       // dead monsters, which scavengers feed on
	Explored []byte               // explored cells, one bit per cell row by row
}

func SnapshotLevel(m *gridmap.GridMap[*Actor, *Item, Object]) *LevelSnapshot {
	s := &LevelSnapshot{Monsters: map[int]SavedMonster{}}
	for _, a := range m.Actors() {
		if mon := a.d1Monster; mon != nil && a.IsAlive() && !mon.Golem {
			p := a.MapPosition()
			// at least 1 HP: regeneration leaves monsters alive below 1
			state := mon.D1MonsterState
			saved := SavedMonster{HP: max(a.GetStats().GetResource(stats.Health).Value(), 1), X: p.X, Y: p.Y, Type: mon.Type, State: &state}
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
	s.Explored = make([]byte, (d1.LevelSize*d1.LevelSize+7)/8)
	for i := range d1.LevelSize * d1.LevelSize {
		if m.IsExplored(geometry.Point{X: i % d1.LevelSize, Y: i / d1.LevelSize}) {
			s.Explored[i/8] |= 1 << (i % 8)
		}
	}
	for _, obj := range m.Objects() {
		if o, ok := obj.(*D1Object); ok && !o.O.Active {
			s.Used = append(s.Used, o.O.Index)
		}
	}
	for _, obj := range m.Objects() {
		if o, ok := obj.(*D1Object); ok && o.Disarmed {
			s.Disarmed = append(s.Disarmed, o.O.Index)
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
	for _, obj := range m.Objects() {
		if o, ok := obj.(*D1Object); ok {
			o.O.Active = !used[o.O.Index]
			if disarmed[o.O.Index] {
				f.State.mapGen.disarmD1Object(m, o)
			}
		}
	}
	replayD1MapChanges(f.State, m)
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
		if !m.CanPlaceActorHere(p) {
			if near := freeCellsNear(m, p, 1, m.CanPlaceActorHere); len(near) > 0 {
				p = near[0]
			}
		}
		SetResource(a.GetStats().GetResource(stats.Health), at[i].HP)
		if at[i].State != nil {
			a.d1Monster.D1MonsterState = *at[i].State
		}
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
	for i := range min(len(s.Explored)*8, d1.LevelSize*d1.LevelSize) {
		if s.Explored[i/8]&(1<<(i%8)) != 0 {
			m.SetExplored(geometry.Point{X: i % d1.LevelSize, Y: i / d1.LevelSize})
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
