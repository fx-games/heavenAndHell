package common

import (
	"slices"
	"strings"

	"rpg/gridmap"
	"rpg/ldtk"
)

// Quests add and remove townspeople by flag, so the change survives the map being rebuilt from its drawing (a
// load, a new visit): "npc_removed:<map>:<npc>" and "npc_added:<map>:<npc>:<location>". An added NPC is drawn
// somewhere in the mod's maps.ldtk; add_npc copies that entity.

func (f *Factory) addNPC(npc, mapName, location string) {
	flags := f.State.GetFlags()
	flags.SetFalse("npc_removed:" + mapName + ":" + npc)
	flags.SetTrue("npc_added:" + mapName + ":" + npc + ":" + location)
	f.State.applyNPCChanges(f.State.Map(mapName))
}

func (f *Factory) removeNPC(npc, mapName string) {
	flags := f.State.GetFlags()
	for key := range flags.AllEntries() {
		if strings.HasPrefix(key, "npc_added:"+mapName+":"+npc+":") {
			flags.SetFalse(key)
		}
	}
	flags.SetTrue("npc_removed:" + mapName + ":" + npc)
	f.State.applyNPCChanges(f.State.Map(mapName))
}

// applyNPCChanges brings a built map's townspeople in line with the flags. A map not built yet gets them when
// the hero arrives.
func (s *State) applyNPCChanges(m *gridmap.GridMap[*Actor, *Item, Object]) {
	if m == nil {
		return
	}
	npcOn := func(name string) *Actor {
		for _, a := range m.Actors() {
			if a.d1Monster == nil && a.GetInternalName() == name {
				return a
			}
		}
		return nil
	}
	var added []string
	for key := range s.GetFlags().AllEntries() {
		if rest, ok := strings.CutPrefix(key, "npc_added:"+m.GetName()+":"); ok {
			added = append(added, rest)
		} else if npc, ok := strings.CutPrefix(key, "npc_removed:"+m.GetName()+":"); ok {
			if a := npcOn(npc); a != nil {
				m.RemoveActor(a)
			}
		}
	}
	slices.Sort(added) // flags are a map: placement order must not be its iteration order
	for _, entry := range added {
		npc, location, _ := strings.Cut(entry, ":")
		if npcOn(npc) != nil {
			continue
		}
		entity := npcEntity(s.ldtkProject, npc)
		if entity == nil {
			s.factory.Msg("add_npc: maps.ldtk draws no NPC " + npc)
			continue
		}
		if cells := freeCellsNear(m, m.GetNamedLocation(location), 1, m.CanPlaceActorHere); len(cells) > 0 {
			addActorFromEntity(s.ldtkProject, m, entity, cells[0])
		}
	}
}

// npcEntity finds the NPC with that internal name wherever the mod's maps.ldtk draws it.
func npcEntity(p *ldtk.Project, npc string) *ldtk.Entity {
	for _, level := range p.Levels {
		for _, layer := range level.Layers {
			for _, e := range layer.Entities {
				if name := e.PropertyByIdentifier("InternalName"); e.Identifier == "NPC" && !name.IsNull() && name.AsString() == npc {
					return e
				}
			}
		}
	}
	return nil
}
