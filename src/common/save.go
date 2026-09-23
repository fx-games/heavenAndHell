package common

import (
	"cmp"
	"maps"
	"rpg/src/dungen"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/rules"
	"rpg/src/stats"
	"slices"
)

// Decision 11: a level is rebuilt from its seed, then this snapshot restores what changed.
type SavedMonster struct {
	HP, X, Y int
	MaxHP    int            `json:",omitempty"` // rolled when the monster was made; 0 in older saves: the new roll's
	Type     int            // only for monsters that objects spawned (not part of the generated level) and corpses
	State    *MonsterState  // nil in saves from before AI state was kept
	Status   map[string]int `json:",omitempty"` // status effects (Stone Curse, ...) and their turns left
	ID       uint64         `json:",omitempty"` // Actor.ID; 0 in saves from before ids
	ShieldBy uint64         `json:",omitempty"` // the ID of whoever cast its Shield
}

type SavedItem struct {
	Item rules.Item
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
	// existed used dungen.LevelSize, which is what a map a D1 generator built is - but any other map (a mod's own
	// level) wrote its rows at the wrong stride and lost every cell past the first LevelSize² of them.
	Width int `json:",omitempty"`
	// RespawnMonsters makes the level forget its dead the next time it is built: Gillian's donation sets it on
	// every explored level that isn't loaded right now (State.RespawnMonsters).
	RespawnMonsters bool
	// Spots are where the level's generator put its monsters (respawn.go): a dungen level rolls them with the
	// factory's own random source, which a rebuild can't replay.
	Spots     []geometry.Point `json:",omitempty"`
	OpenDoors []geometry.Point `json:",omitempty"` // open doors that aren't D1's (those are in Doors)
	Decals    []SavedDecal     `json:",omitempty"` // blood and the like
}

type SavedDecal struct {
	X, Y int
	Icon int32
}

func SnapshotLevel(state *State, m *gridmap.GridMap[*Actor, *Item, Object]) *LevelSnapshot {
	s := &LevelSnapshot{Monsters: map[int]SavedMonster{}}
	if lvl := state.mapGen.generated[m.GetName()]; lvl != nil {
		s.Traps = sprungTraps(lvl)
	}
	for _, a := range m.Actors() {
		if mon := a.monster; mon != nil && a.IsAlive() && !mon.Golem {
			p := a.MapPosition()
			// at least 1 HP: regeneration leaves monsters alive below 1
			state := mon.MonsterState
			saved := SavedMonster{HP: max(a.GetStats().GetResource(stats.Health).Value(), 1), MaxHP: a.GetStats().GetAttribute(stats.MaxHealth), X: p.X, Y: p.Y, Type: mon.Type, State: &state, Status: maps.Clone(a.statusEffects.AllEntries()), ID: a.id, ShieldBy: a.shieldedBy}
			if mon.SpawnIndex < 0 {
				s.Spawned = append(s.Spawned, saved)
			} else {
				s.Monsters[mon.SpawnIndex] = saved
			}
		}
	}
	for _, a := range m.DownedActors() {
		if mon := a.monster; mon != nil {
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
		if o, ok := obj.(*Prop); ok && !o.O.Active {
			s.Used = append(s.Used, o.O.Index)
		}
	}
	for _, obj := range m.Objects() {
		switch o := obj.(type) {
		case *Prop:
			if o.Disarmed {
				s.Disarmed = append(s.Disarmed, o.O.Index)
			}
		case *Door:
			if o.Prop != nil && o.Disarmed {
				s.Disarmed = append(s.Disarmed, o.Prop.Index)
			}
			if o.Prop != nil && o.isOpen {
				s.Doors = append(s.Doors, o.Prop.Index)
			}
			if o.Prop == nil && o.isOpen {
				s.OpenDoors = append(s.OpenDoors, o.position)
			}
		}
	}
	if r := state.mapGen.rosters[m.GetName()]; r != nil {
		s.Spots = r.spots
	}
	for p, icon := range m.Decals() {
		s.Decals = append(s.Decals, SavedDecal{p.X, p.Y, icon})
	}
	slices.SortFunc(s.Decals, func(a, b SavedDecal) int { return cmp.Or(cmp.Compare(a.Y, b.Y), cmp.Compare(a.X, b.X)) })
	for _, it := range m.Items() {
		if it.roll != nil {
			p := it.MapPosition()
			s.Items = append(s.Items, SavedItem{Item: *it.roll, X: p.X, Y: p.Y})
		}
	}
	return s
}

// SetResource sets a resource's current value, clamped to its range.
func SetResource(r *stats.Resource, value int) {
	r.SetToMax()
	r.Decrease(r.GetMax() - value)
}

// RestoreExplored makes the map's explored cells exactly the snapshot's: what arriving on the level lit up
// before the hero stood where the save left them is forgotten again.
func (s *LevelSnapshot) RestoreExplored(m *gridmap.GridMap[*Actor, *Item, Object]) {
	// A snapshot is only ever applied to the level it was taken from, so its stride is this map's width; older
	// saves have none and were all written by a D1 generator, whose levels are LevelSize wide.
	width := s.Width
	if width == 0 {
		width = dungen.LevelSize
	}
	m.ForgetExplored()
	size := m.MapSize()
	for i := range min(len(s.Explored)*8, width*size.Y) {
		if p := (geometry.Point{X: i % width, Y: i / width}); p.X < size.X && s.Explored[i/8]&(1<<(i%8)) != 0 {
			m.SetExplored(p)
		}
	}
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
	if lvl := f.State.mapGen.generated[m.GetName()]; lvl != nil {
		for _, t := range lvl.Objects {
			if (t.Kind == rules.ObjTRAPL || t.Kind == rules.ObjTRAPR) && sprung[t.Index] {
				t.Var4 = 1
				for _, o := range lvl.Objects { // Obj_Trap cleared the target's flag when the trap fired
					if o.X == t.Var1 && o.Y == t.Var2 {
						o.TrapFlag = false
					}
				}
			}
			if used[t.Index] && t.Kind >= rules.ObjTCHEST1 && t.Kind <= rules.ObjTCHEST3 { // opening set its trap off
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
		case *Prop:
			o.O.Active = !used[o.O.Index]
			if disarmed[o.O.Index] {
				o.Disarmed = true
				f.State.mapGen.disarmObject(m, o.O)
			}
		case *Door:
			if o.Prop != nil && disarmed[o.Prop.Index] {
				o.Disarmed = true
				f.State.mapGen.disarmObject(m, o.Prop)
			}
			if o.Prop != nil && open[o.Prop.Index] {
				o.isOpen = true // its wall traps are restored above, so opening it doesn't fire them again
			}
			if o.Prop == nil && slices.Contains(s.OpenDoors, o.position) {
				o.isOpen = true
			}
		}
	}
	f.State.applyPatches(m)
	if s.Spots != nil {
		f.State.mapGen.roster(m.GetName()).spots = s.Spots
	}
	for _, d := range s.Decals {
		m.AddDecal(geometry.Point{X: d.X, Y: d.Y}, d.Icon)
	}
	if s.RespawnMonsters { // the level keeps the monsters the generator just placed, and buries its corpses
		s.RespawnMonsters = false
		s.Monsters, s.Spawned, s.Corpses = map[int]SavedMonster{}, nil, nil
	} else {
		// take the monsters off the map first, so none finds its saved cell taken by one that hasn't moved yet
		var restore []*Actor
		var at []SavedMonster
		for _, a := range append([]*Actor(nil), m.Actors()...) {
			mon := a.monster
			if mon == nil {
				continue
			}
			m.RemoveActor(a)
			if saved, alive := s.Monsters[mon.SpawnIndex]; alive {
				restore, at = append(restore, a), append(at, saved)
			}
		}
		for _, saved := range s.Spawned {
			a := f.NewMonster(saved.Type)
			a.monster.SpawnIndex = -1
			restore, at = append(restore, a), append(at, saved)
		}
		for i, a := range restore {
			p := geometry.Point{X: at[i].X, Y: at[i].Y}
			// the cell may have been taken since the level was saved; a monster that finds nowhere at all is
			// dropped rather than left half on the map (gridmap.AddActor)
			if !canPlaceMonster(m, p) || m.IsActorAt(p) {
				near := freeCellsNear(m, p, 1, m.CanPlaceActorHere)
				if len(near) == 0 {
					continue
				}
				p = near[0]
			}
			if at[i].MaxHP > 0 {
				a.GetStats().SetAttribute(stats.MaxHealth, at[i].MaxHP)
			}
			SetResource(a.GetStats().GetResource(stats.Health), at[i].HP)
			if at[i].State != nil {
				a.monster.MonsterState = *at[i].State
			}
			maps.Copy(a.statusEffects.AllEntries(), at[i].Status)
			a.SetID(at[i].ID)
			a.shieldedBy = at[i].ShieldBy
			a.SetDrawPosition(p.ToPointF())
			m.AddActor(a, p)
		}
		DropOrphanShields(m, f.State.GetPlayer())
		for _, c := range s.Corpses {
			a := f.NewMonster(c.Type)
			SetResource(a.GetStats().GetResource(stats.Health), 0)
			a.SetDrawPosition(geometry.Point{X: c.X, Y: c.Y}.ToPointF())
			a.SetMapPosition(geometry.Point{X: c.X, Y: c.Y})
			m.AddDownedActor(a, geometry.Point{X: c.X, Y: c.Y})
		}
	}
	s.RestoreExplored(m)
	for _, it := range append([]*Item(nil), m.Items()...) {
		if it.roll != nil {
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
		m.AddItem(f.NewRolledItem(&item), p)
	}
}
