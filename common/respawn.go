package common

import (
	"hash/fnv"
	"math/rand"
	"slices"
	"strconv"

	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
)

// mod.rec's dungeon rules, applied as the hero leaves and enters a dungeon level:
//
//	Respawn             disabled (default): dead is dead. enabled: the level is back at the population its
//	                    generator gave it on every entry. timed: RespawnCount monsters per RespawnTurns turns spent
//	                    elsewhere. town_visits: RespawnCount per town visit since the hero left the level.
//	RespawnUniques      none (default): respawns are rolled from the mod's ordinary monsters. random: a unique of
//	                    the depth may be rolled too. same: the level's own dead uniques come back first.
//	PersistLevels       false: a dungeon level is forgotten when left and built anew on the next visit (Angband).
//	                    Default true
//	PersistQuestLevels  true (default): a level a quest names (or a set piece) is never rebuilt nor respawned,
//	                    whatever the rules above say; false: it follows them like any other.
//
// Respawned monsters are rolled afresh, never copies of the dead: at the spots the generator put its own, from
// the mod's table for the depth. Everything here is saved (DungeonClock, LevelSnapshot.Spots), so a load neither
// resets a timer nor changes what a level rebuilds as.

// DungeonRules are the fields above.
type DungeonRules struct {
	Respawn, Uniques    string
	RespawnCount, Turns int
	Forget              bool // PersistLevels: false
	QuestLevelsFollow   bool // PersistQuestLevels: false
}

// ReadDungeonRules reads the mod's DungeonRules.
func ReadDungeonRules() DungeonRules {
	num := func(name string) int { v, _ := ModSetting(name); n, _ := strconv.Atoi(v); return n }
	r := DungeonRules{RespawnCount: num("RespawnCount"), Turns: num("RespawnTurns")}
	r.Respawn, _ = ModSetting("Respawn")
	r.Uniques, _ = ModSetting("RespawnUniques")
	v, _ := ModSetting("PersistLevels")
	r.Forget = v == "false"
	v, _ = ModSetting("PersistQuestLevels")
	r.QuestLevelsFollow = v == "false"
	return r
}

// SetDungeonRules picks the mod's DungeonRules for this game.
func (s *State) SetDungeonRules(r DungeonRules) { s.dungeonRules = r }

// LevelClock is when the hero left a level: the turn and how many town visits had been made by then.
type LevelClock struct {
	Turn       uint64
	TownVisits int
}

// DungeonClock is what the rules count with, as a save keeps it.
type DungeonClock struct {
	Turn        uint64
	TownVisits  int
	LastSeen    map[string]LevelClock
	Generations map[string]int // how often each level was forgotten and rebuilt; picks its seed
}

func (s *State) DungeonClock() DungeonClock {
	return DungeonClock{s.timekeeper.turnCounter, s.townVisits, s.lastSeen, s.mapGen.generations}
}

func (s *State) SetDungeonClock(c DungeonClock) {
	s.timekeeper.turnCounter, s.townVisits, s.lastSeen, s.mapGen.generations = c.Turn, c.TownVisits, c.LastSeen, c.Generations
}

// roster is where a level's generator put its monsters, and its uniques, for respawns.
type roster struct {
	spots   []geometry.Point
	uniques []uniqueSpawn
}

type uniqueSpawn struct {
	at   geometry.Point
	make func() *Actor
	is   func(*Actor) bool // this unique, alive or restored from a save
}

func (g *MapGenerator) roster(id string) *roster {
	if g.rosters == nil {
		g.rosters = map[string]*roster{}
	}
	if g.rosters[id] == nil {
		g.rosters[id] = &roster{}
	}
	return g.rosters[id]
}

// addSpawn records a monster the generator just put on m; fresh and is only matter for a unique.
func (g *MapGenerator) addSpawn(m *gridmap.GridMap[*Actor, *Item, Object], a *Actor, fresh func() *Actor, is func(*Actor) bool) {
	r := g.roster(m.GetName())
	r.spots = append(r.spots, a.MapPosition())
	if a.IsUnique() && fresh != nil {
		r.uniques = append(r.uniques, uniqueSpawn{a.MapPosition(), fresh, is})
	}
}

// sameName matches a monster by its record, for uniques that are one per record (H&H's).
func sameName(id string) func(*Actor) bool {
	return func(a *Actor) bool { return a.IsUnique() && a.GetInternalName() == id }
}

// levelRand is the random source level id is built from: the same for the same game seed and generation, so a
// load rebuilds exactly the layout that was saved, and a new one each time the level is forgotten.
func (g *MapGenerator) levelRand(id string) *rand.Rand {
	h := fnv.New64a()
	h.Write([]byte(id))
	return rand.New(rand.NewSource(g.seed ^ int64(h.Sum64()) ^ int64(uint64(g.generations[id])*0x9E3779B97F4A7C15)))
}

// d1LevelSeeds is D1's seed table for level id: glSeedTbl as D1 rolled it, the level's depth re-rolled once it
// has been forgotten.
func (g *MapGenerator) d1LevelSeeds(id string, depth int) [d1.NumLevels]int32 {
	seeds := g.d1Seeds
	if g.generations[id] > 0 {
		seeds[depth] = g.levelRand(id).Int31()
	}
	return seeds
}

// keepsLevel: level id is exempt from the rules (a town, or a quest level the mod keeps).
func (s *State) keepsLevel(id string) bool {
	l := s.Level(id)
	if l.Town {
		return true
	}
	if s.dungeonRules.QuestLevelsFollow {
		return false
	}
	if _, set := l.Config.(d1.SetLevelParams); set {
		return true
	}
	return slices.ContainsFunc(s.QuestDefs(), func(q defs.Quest) bool { return q.Level == id })
}

// leaveLevel stops level id's clock, or forgets the level when the mod doesn't keep them.
func (s *State) leaveLevel(id string) {
	if s.keepsLevel(id) {
		return
	}
	if s.dungeonRules.Forget {
		g := s.mapGen
		delete(s.maps, id)
		delete(g.snapshots, id)
		delete(g.rosters, id)
		delete(g.d1Levels, id)
		if g.generations == nil {
			g.generations = map[string]int{}
		}
		g.generations[id]++
		return
	}
	if s.lastSeen == nil {
		s.lastSeen = map[string]LevelClock{}
	}
	s.lastSeen[id] = LevelClock{s.timekeeper.turnCounter, s.townVisits}
}

// enterLevel brings back the monsters the mod's Respawn says are due on m since the hero last left it.
func (s *State) enterLevel(m *gridmap.GridMap[*Actor, *Item, Object]) {
	id := m.GetName()
	if s.Level(id).Town {
		s.townVisits++
	}
	last, seen := s.lastSeen[id]
	if !seen || s.keepsLevel(id) {
		return
	}
	delete(s.lastSeen, id)
	switch r := s.dungeonRules; r.Respawn {
	case "enabled":
		s.respawn(m, -1)
	case "timed":
		if r.Turns > 0 {
			s.respawn(m, r.RespawnCount*int((s.timekeeper.turnCounter-last.Turn)/uint64(r.Turns)))
		}
	case "town_visits":
		s.respawn(m, r.RespawnCount*(s.townVisits-last.TownVisits))
	}
}

// respawn rolls up to n monsters (all that are missing when negative) onto m, never past the population its
// generator gave it.
func (s *State) respawn(m *gridmap.GridMap[*Actor, *Item, Object], n int) {
	g, r := s.mapGen, s.mapGen.roster(m.GetName())
	var alive []*Actor
	for _, a := range m.Actors() {
		if a.d1Monster != nil && !a.d1Monster.Golem && a.IsAlive() {
			alive = append(alive, a)
		}
	}
	due := len(r.spots) - len(alive)
	if n >= 0 {
		due = min(due, n)
	}
	place := func(a *Actor, at geometry.Point) bool {
		if !m.CanPlaceActorHere(at) {
			return false
		}
		a.SetDrawPosition(at.ToPointF())
		m.AddActor(a, at)
		alive = append(alive, a)
		due--
		return true
	}
	if s.dungeonRules.Uniques == "same" {
		for _, u := range r.uniques {
			if due > 0 && !slices.ContainsFunc(alive, u.is) {
				if a := u.make(); place(a, u.at) && g.d1Levels[m.GetName()] != nil {
					s.factory.initD1Talk(a)
				}
			}
		}
	}
	depth := s.Level(m.GetName()).Depth
	for _, i := range g.randomSource.Perm(len(r.spots)) {
		if due <= 0 {
			return
		}
		a, companions := s.factory.rollEnemy(depth, s.dungeonRules.Uniques == "random", func(id string) bool {
			return slices.ContainsFunc(alive, sameName(id))
		})
		if a == nil {
			continue
		}
		a.d1Monster.SpawnIndex = -1 // saved in LevelSnapshot.Spawned
		if !place(a, r.spots[i]) {
			continue
		}
		for _, c := range g.spawnDungenCompanions(m, r.spots[i], companions[:min(len(companions), due)]) {
			c.d1Monster.SpawnIndex = -1
			alive = append(alive, c)
			due--
		}
	}
}
