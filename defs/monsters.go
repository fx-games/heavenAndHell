package defs

import (
	"fmt"
	"image/color"
	"io/fs"
	"math/rand"
	"path"
	"strings"

	"rpg/recfile"
	"rpg/util"
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
	MinDLvl, MaxDLvl int
	Level            int
	Avail            int // 0 = never spawns at random (D1's MonstAvailTbl)
	OutOfDepthChance float64
	MinLevelForOoD   int
	GroupSpawn       []string

	// Combat.
	Health, Damage, Damage2 util.Dice
	Hit, AFNum              int
	Hit2, AFNum2            int
	AC                      int
	MagicRes, MagicRes2     int
	Treasure, Exp           int
	BaseSpeed               int
	Stats                   map[string]int

	// Behaviour.
	AI         string // D1 brain name (AI_*); empty for a mod that drives monsters by skills alone
	Class      string // MC_UNDEAD, MC_DEMON, MC_ANIMAL
	Flags, Int int
	AggroRange int
	Tags       []string

	TargetedSkills   []string
	UntargetedSkills []string
	Equipment        []string

	// Presentation.
	Family                            string // graphics folder, e.g. "Zombie"; variants share it
	ImgSize                           int    // graphics budget per level
	Frames, Rate                      [6]int // stand, walk, attack, hit, death, special
	AsciiIcon, UnicodeIcon, CP437Icon rune
	Color                             color.RGBA
	SpeechOnSight, SpeechOnHurt       []string
	DeathStringFormats                []string
}

// Available reports whether the monster turns up in random spawns.
func (m Monster) Available() bool { return m.Avail != 0 }

// CanAppearAt reports whether this monster may be rolled for dungeon level dlvl: at or below its own depth, or
// above it on an out-of-depth roll, and never past max_dlvl. The record carries all four knobs, so the rule
// lives with the record rather than with whichever generator happens to be placing monsters.
func (m Monster) CanAppearAt(randomSource *rand.Rand, dlvl int) bool {
	if m.MaxDLvl > 0 && dlvl > m.MaxDLvl {
		return false
	}
	if dlvl >= m.MinDLvl {
		return true
	}
	return dlvl >= m.MinLevelForOoD && randomSource.Float64() < m.OutOfDepthChance
}

// MinHP/MaxHP/MinDam/MaxDam are the bounds the combat code works in, from the dice the record carries.
func (m Monster) MinHP() int   { return m.Health.Min() }
func (m Monster) MaxHP() int   { return m.Health.Max() }
func (m Monster) MinDam() int  { return m.Damage.Min() }
func (m Monster) MaxDam() int  { return m.Damage.Max() }
func (m Monster) MinDam2() int { return m.Damage2.Min() }
func (m Monster) MaxDam2() int { return m.Damage2.Max() }

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
		if m.Level == 0 { // a record that names only its depth fights, drops and pays out at that depth, as items do
			m.Level = m.MinDLvl
		}
		out = append(out, m)
	}
	return out, nil
}

func monsterFromRecord(rec recfile.Record) (Monster, error) {
	m := Monster{Stats: map[string]int{}, AsciiIcon: -1, UnicodeIcon: -1, CP437Icon: -1}
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
		case "level":
			m.Level = f.AsInt()
		case "available":
			m.Avail = f.AsInt()
		case "out_of_depth_chance":
			m.OutOfDepthChance = f.AsFloat()
		case "min_level_for_ood":
			m.MinLevelForOoD = f.AsInt()
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
		case "afnum":
			m.AFNum = f.AsInt()
		case "afnum2":
			m.AFNum2 = f.AsInt()
		case "magic_res":
			m.MagicRes = f.AsInt()
		case "magic_res2":
			m.MagicRes2 = f.AsInt()
		case "treasure":
			m.Treasure = f.AsInt()
		case "base_speed":
			m.BaseSpeed = f.AsInt()
		case "ai":
			m.AI = f.Value
		case "class":
			m.Class = f.Value
		case "flags":
			m.Flags = f.AsInt()
		case "int":
			m.Int = f.AsInt()
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
		case "graphics_size":
			m.ImgSize = f.AsInt()
		case "frames":
			m.Frames = six(f.Value)
		case "rate":
			m.Rate = six(f.Value)
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

// six reads "11, 24, 12, 6, 16, 0" -- a fixed-length row of numbers that is one value, not six fields.
func six(v string) (out [6]int) {
	for i, p := range strings.Split(v, ",") {
		if i < 6 {
			out[i] = recfile.StrInt(strings.TrimSpace(p))
		}
	}
	return
}
