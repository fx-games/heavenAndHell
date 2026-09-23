package defs

import (
	"fmt"
	"image/color"
	"io/fs"
	"path"
	"slices"
	"strings"

	"rpg/src/recfile"
	"rpg/src/util"
)

// Monster is the unified monster record every mod supplies in assets/data/enemies.rec. It is the union of what
// the two games need, not a lowest common denominator: Diablo's brains and bitfields sit next to Heaven&Hell's
// dice, skills and speech, and a mod fills in the fields its game uses. The engine reads one schema whichever
// mod supplied it -- there is no second loader and no per-mod branch.
//
// Health and Damage are util.Dice because the two schemas met there: D1 stores a uniform min..max range and
// H&H stores XdY, and "4-7" is exactly 1d4+3, so one field carries both losslessly (util.ParseDice reads
// "4-7", "1d3+1" and a bare "5" alike). Min()/Max() give D1 back its original numbers.
type Monster struct {
	ID   string // internal name: D1's MT_* enum, or H&H's "rat"
	Name string
	Icon string // icon ID in the mod's icons/actors.rec, e.g. "monster.nzombie"

	// Where it spawns.
	MinDLvl, MaxDLvl int     // the depths it spawns on; MaxDLvl 0: no limit
	Level            int     // monster_level: to-hit, spell hit chance, experience, drops; MinDLvl if unset
	RandomSpawn      bool    // false: only quests, rooms, side levels and group_spawn place it
	SpawnCost        int     // its share of a level's SpawnBudget (D1: graphics memory); 0 = free, always in
	WanderChance     float64 // >0: never in the type draw; a level its depth allows gets a single one at these odds
	GroupSpawn       []string

	// Combat.
	Health, Damage, Damage2 util.Dice
	Hit, Hit2               int
	AC                      int
	MagicRes, MagicRes2     int
	Drops                   string // "none", a unique item's id, or "" for the usual roll
	Exp                     int
	// AUT a step and an attack take (default 10, one turn) and a hard hit costs it (0: none)
	SpeedMove, SpeedAttack, SpeedRecovery int
	Stats                                 map[string]int
	FullHP                                bool // health is the monster's real hit points, not D1's doubled single-player table

	// Behaviour.
	AI           string // D1 brain name (AI_*); empty for a mod that drives monsters by skills alone
	Class        string // MC_UNDEAD, MC_DEMON, MC_ANIMAL
	AILevel      int    // D1's mInt, 0-3
	AggroRange   int
	SpecialOneIn int    // the Fallen's howl: 1 in N per standing turn
	FleeDistance int    // how far it runs when a comrade dies near it (M_FallenFear); 0: it does not
	Tint         string // "R G B" its icon is tinted with; empty: untinted
	FixedHealth  int    // its hit points whatever Health rolls; 0: the roll
	SightSfx     string // sfx.rec id it says when it first sees the hero
	Tags         []string

	TargetedSkills   []string
	UntargetedSkills []string
	Equipment        []string

	// Presentation.
	Family                            string // graphics folder, e.g. "Zombie"; variants share it
	AsciiIcon, UnicodeIcon, CP437Icon rune
	Color                             color.RGBA
	SpeechOnSight, SpeechOnHurt       []string
	DeathStringFormats                []string
}

// Has reports whether the record carries tag.
func (m Monster) Has(tag string) bool { return slices.Contains(m.Tags, tag) }

// SpawnBudget is what a level's monster types may cost together (D1's 4000 of graphics memory).
const SpawnBudget = 4000

// PickSpawnTypes ports GetLevelMTypes' draw: up to 16 of the candidate records, picked at random while their
// spawn_cost still fits the budget. It returns indices into ms.
func PickSpawnTypes(ms []Monster, candidates []int, rnd func(int) int) []int {
	list := slices.Clone(candidates)
	var types []int
	budget := SpawnBudget
	for len(list) > 0 && len(types) < 16 {
		for i := 0; i < len(list); { // swap-remove, as D1 does: the order decides which type rnd picks
			if ms[list[i]].SpawnCost > budget {
				list[i] = list[len(list)-1]
				list = list[:len(list)-1]
			} else {
				i++
			}
		}
		if len(list) == 0 {
			break
		}
		rv := rnd(len(list))
		types = append(types, list[rv])
		budget -= ms[list[rv]].SpawnCost
		list[rv] = list[len(list)-1]
		list = list[:len(list)-1]
	}
	return types
}

// SpawnsAt reports whether dlvl..max_dlvl takes in dungeon level dlvl.
func (m Monster) SpawnsAt(dlvl int) bool {
	return dlvl >= m.MinDLvl && (m.MaxDLvl == 0 || dlvl <= m.MaxDLvl)
}

// MinHP/MaxHP/MinDam/MaxDam are the bounds the combat code works in, from the dice the record carries.
func (m Monster) MinHP() int   { return m.Health.Min() }
func (m Monster) MaxHP() int   { return m.Health.Max() }
func (m Monster) MinDam() int  { return m.Damage.Min() }
func (m Monster) MaxDam() int  { return m.Damage.Max() }
func (m Monster) MinDam2() int { return m.Damage2.Min() }
func (m Monster) MaxDam2() int { return m.Damage2.Max() }

// fillCombatModel gives a record without a D1 brain (H&H's) the to-hit D1's combat reads and its health as
// stated. The to-hit is a design choice of the port, tunable in play: five per point of dexterity.
func fillCombatModel(m *Monster) {
	// defaults near D1's median monster (fallen: 14/16/6), so a swing costs about what the hero's does
	if m.SpeedMove <= 0 {
		m.SpeedMove = 12
	}
	if m.SpeedAttack <= 0 {
		m.SpeedAttack = 16
	}
	if m.Level == 0 { // a record that names only its depth fights, drops and pays out at that depth, as items do
		m.Level = m.MinDLvl
	}
	if m.AI != "" {
		return
	}
	if m.SpeedRecovery <= 0 { // without one, the hero's hardest blow never makes it flinch
		m.SpeedRecovery = 5
	}
	m.FullHP = true
	if m.Hit == 0 {
		m.Hit = 5 * m.Stats["dexterity"]
	}
}

// ReadMonsters loads the active mod's monsters, in file order -- order is content, not incidental: D1 indexes
// its monsters by table position in saves and spawn tables.
func ReadMonsters(fsys fs.FS) ([]Monster, error) {
	f, err := fsys.Open(path.Join("assets", "data", "enemies.rec"))
	if err != nil {
		return nil, fmt.Errorf("enemies.rec: %w", err)
	}
	defer f.Close()
	var out []Monster
	for i, rec := range recfile.Read(f) {
		m, err := monsterFromRecord(rec)
		if err != nil {
			return nil, fmt.Errorf("enemies.rec record %d: %w", i+1, err)
		}
		fillCombatModel(&m)
		out = append(out, m)
	}
	return out, nil
}

func monsterFromRecord(rec recfile.Record) (Monster, error) {
	m := Monster{Stats: map[string]int{}, RandomSpawn: true, AsciiIcon: -1, UnicodeIcon: -1, CP437Icon: -1}
	for _, f := range rec {
		if stat, ok := strings.CutPrefix(f.Name, "stats_"); ok {
			m.Stats[stat] = f.AsInt()
			continue
		}
		switch f.Name {
		case "internal_name":
			m.ID = f.Value
		case "name":
			m.Name = f.Value
		case "icon":
			m.Icon = f.Value
		case "dlvl":
			m.MinDLvl = f.AsInt()
		case "max_dlvl":
			m.MaxDLvl = f.AsInt()
		case "monster_level":
			m.Level = f.AsInt()
		case "wander_chance":
			m.WanderChance = f.AsFloat()
		case "random_spawn":
			m.RandomSpawn = f.Value != "false"
		case "spawn_cost":
			m.SpawnCost = f.AsInt()
		case "group_spawn":
			m.GroupSpawn = append(m.GroupSpawn, f.Value)
		case "health":
			m.Health = util.ParseDice(f.Value)
		case "damage":
			m.Damage = util.ParseDice(f.Value)
		case "damage2":
			m.Damage2 = util.ParseDice(f.Value)
		case "hit":
			m.Hit = f.AsInt()
		case "hit2":
			m.Hit2 = f.AsInt()
		case "magic_res":
			m.MagicRes = f.AsInt()
		case "magic_res2":
			m.MagicRes2 = f.AsInt()
		case "drops":
			m.Drops = f.Value
		case "ai":
			m.AI = f.Value
		case "class":
			m.Class = f.Value
		case "ai_level":
			m.AILevel = f.AsInt()
		case "aggro_range":
			m.AggroRange = f.AsInt()
		case "tags":
			// "unique | no_corpse" is one field carrying several tags, which is how H&H's content writes them.
			for _, t := range strings.Split(f.Value, "|") {
				if t = strings.TrimSpace(t); t != "" {
					m.Tags = append(m.Tags, t)
				}
			}
		case "targeted_skill":
			m.TargetedSkills = append(m.TargetedSkills, f.Value)
		case "untargeted_skill":
			m.UntargetedSkills = append(m.UntargetedSkills, f.Value)
		case "equipment":
			m.Equipment = append(m.Equipment, f.Value)
		case "graphics_family":
			m.Family = f.Value
		case "speed_move":
			m.SpeedMove = f.AsInt()
		case "speed_attack":
			m.SpeedAttack = f.AsInt()
		case "speed_recovery":
			m.SpeedRecovery = f.AsInt()
		case "special_one_in":
			m.SpecialOneIn = f.AsInt()
		case "fixed_health":
			m.FixedHealth = f.AsInt()
		case "sight_sfx":
			m.SightSfx = f.Value
		case "tint":
			m.Tint = f.Value
		case "flee_distance":
			m.FleeDistance = f.AsInt()
		case "ascii":
			m.AsciiIcon = firstRune(f.Value)
		case "unicode":
			m.UnicodeIcon = firstRune(f.Value)
		case "cp437":
			m.CP437Icon = rune(util.UnicodeToCP437Byte(firstRune(f.Value)))
		case "color":
			m.Color = util.GetColorFromRGBString(f.Value)
		case "speech_on_sight":
			m.SpeechOnSight = append(m.SpeechOnSight, f.Value)
		case "speech_on_hurt":
			m.SpeechOnHurt = append(m.SpeechOnHurt, f.Value)
		case "death_string":
			m.DeathStringFormats = append(m.DeathStringFormats, f.Value)
		}
	}
	if m.ID == "" {
		return m, fmt.Errorf("no internal_name")
	}
	// Stats carry the two numbers D1 keeps in columns of its own, so a mod writes them once, in one place.
	m.AC, m.Exp = m.Stats["armor_class"], m.Stats["experience"]
	return m, nil
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return -1
}
