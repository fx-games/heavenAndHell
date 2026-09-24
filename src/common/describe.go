package common

import (
	"slices"
	"strings"

	"rpg/src/defs"
	"rpg/src/geometry"
)

// SceneryAt names what the map shows at p, below any actor or item: an object (a searchable one, or decoration:
// a lamp, a hooked body), else what hangs on the cell (a wall torch), else the tile itself. Both UIs hover it.
func (s *State) SceneryAt(p geometry.Point) string {
	m := s.GetMap()
	if o, ok := m.TryGetObjectAt(p); ok {
		if name := o.Name(); name != "" {
			return name
		}
		if name := s.iconName(o.Icon(0)); name != "" {
			return name
		}
	}
	if icon, ok := m.GetDecal(p); ok {
		if name := s.iconName(icon); name != "" {
			return name
		}
	}
	return m.GetCell(p).TileType.DefinedDescription
}

// iconName is what a picture on the map stands for: the dungeon object drawn with it, else its tiles.rec
// record (blood and missile effects are not things one looks at). Icons two records share name the object first,
// then the record with the lowest id, so the answer doesn't hang on file order.
func (s *State) iconName(icon int32) string {
	if s.iconNames == nil {
		s.iconNames = map[int32]string{}
		ids := make([]string, 0, len(defs.TileRecords))
		for id := range defs.TileRecords {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		for _, id := range ids {
			if r := defs.TileRecords[id]; r.Name != "" && !strings.HasPrefix(id, "decal.") && !strings.HasPrefix(id, "effect.") {
				if _, taken := s.iconNames[r.Icon]; !taken {
					s.iconNames[r.Icon] = r.Name
				}
			}
		}
		if s.factory != nil && s.factory.Generator != nil {
			d := s.factory.Generator.D
			kinds := make([]int, 0, len(d.ObjectDisplays))
			for kind := range d.ObjectDisplays {
				kinds = append(kinds, kind)
			}
			slices.Sort(kinds)
			named := map[int32]bool{}
			for _, kind := range kinds {
				icon, ok := defs.IconOK(d.PropIcon(kind))
				if name := d.ObjectDisplays[kind].Name; ok && name != "" && !named[icon] {
					s.iconNames[icon], named[icon] = name, true
				}
			}
		}
	}
	return s.iconNames[icon]
}
