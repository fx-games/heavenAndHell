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
}

type SavedItem struct {
	Item d1.Item
	X, Y int
}

type LevelSnapshot struct {
	Monsters map[int]SavedMonster // by spawn index; missing = killed
	Items    []SavedItem          // D1 floor items
}

func SnapshotLevel(m *gridmap.GridMap[*Actor, *Item, Object]) *LevelSnapshot {
	s := &LevelSnapshot{Monsters: map[int]SavedMonster{}}
	for _, a := range m.Actors() {
		if mon := a.d1Monster; mon != nil && a.IsAlive() {
			p := a.MapPosition()
			s.Monsters[mon.SpawnIndex] = SavedMonster{HP: a.GetStats().GetResource(stats.Health).Value(), X: p.X, Y: p.Y}
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
	for _, a := range append([]*Actor(nil), m.Actors()...) {
		mon := a.d1Monster
		if mon == nil {
			continue
		}
		saved, alive := s.Monsters[mon.SpawnIndex]
		if !alive {
			m.RemoveActor(a)
			continue
		}
		SetResource(a.GetStats().GetResource(stats.Health), saved.HP)
		if p := (geometry.Point{X: saved.X, Y: saved.Y}); p != a.MapPosition() && m.CanPlaceActorHere(p) {
			m.MoveActor(a, p)
			a.SetDrawPosition(p.ToPointF())
		}
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
