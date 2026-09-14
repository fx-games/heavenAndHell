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
	Type     int // only for monsters that objects spawned (not part of the generated level)
}

type SavedItem struct {
	Item d1.Item
	X, Y int
}

type LevelSnapshot struct {
	Monsters map[int]SavedMonster // by spawn index; missing = killed
	Items    []SavedItem          // D1 floor items
	Used     []int                // indices of used objects (opened chests, broken barrels, ...)
	Spawned  []SavedMonster       // skeletons from sarcophagi and barrels
}

func SnapshotLevel(m *gridmap.GridMap[*Actor, *Item, Object]) *LevelSnapshot {
	s := &LevelSnapshot{Monsters: map[int]SavedMonster{}}
	for _, a := range m.Actors() {
		if mon := a.d1Monster; mon != nil && a.IsAlive() && !mon.Golem {
			p := a.MapPosition()
			// at least 1 HP: regeneration leaves monsters alive below 1
			saved := SavedMonster{HP: max(a.GetStats().GetResource(stats.Health).Value(), 1), X: p.X, Y: p.Y, Type: mon.Type}
			if mon.SpawnIndex < 0 {
				s.Spawned = append(s.Spawned, saved)
			} else {
				s.Monsters[mon.SpawnIndex] = saved
			}
		}
	}
	for _, obj := range m.Objects() {
		if o, ok := obj.(*D1Object); ok && !o.O.Active {
			s.Used = append(s.Used, o.O.Index)
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
	for _, obj := range m.Objects() {
		if o, ok := obj.(*D1Object); ok {
			o.O.Active = !used[o.O.Index]
		}
	}
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
		a.SetDrawPosition(p.ToPointF())
		m.AddActor(a, p)
	}
	for _, it := range append([]*Item(nil), m.Items()...) {
		if it.d1 != nil {
			m.RemoveItem(it)
		}
	}
	for _, si := range s.Items {
		item := si.Item
		m.AddItem(f.NewD1Item(&item), geometry.Point{X: si.X, Y: si.Y})
	}
}
