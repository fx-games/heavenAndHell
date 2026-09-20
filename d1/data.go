// Package d1 holds Diablo 1's data tables (loaded verbatim from mods/d1/data) and its generation rules.
// Enum columns stay strings (e.g. "IPL_TOHIT", "AI_ZOMBIE"); only bitfields are numbers.
package d1

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"
)

type row map[string]string

func (r row) s(k string) string { return r[k] }
func (r row) i(k string) int {
	v, err := strconv.Atoi(strings.TrimSpace(r[k]))
	if err != nil {
		return 0 // "NULL" and friends
	}
	return v
}
func (r row) arr(k string) (out [6]int) {
	for n, p := range strings.Split(r[k], ",") {
		if n < 6 {
			out[n], _ = strconv.Atoi(strings.TrimSpace(p))
		}
	}
	return
}

// dataPath locates one of this mod's data files. Diablo 1 is a mod like any other: its tables, texts and level
// data live under files/mods/d1/data, where they shadow assets/data for a game that selects this mod. Package d1
// reads them from their real home rather than through the overlay, so a caller with a plain fs.FS rooted at
// files/ (every test does this) gets the same bytes as the running game.
func dataPath(elem ...string) string {
	return path.Join(append([]string{"mods", "d1", "data"}, elem...)...)
}

func readTable(fsys fs.FS, name string) ([]row, error) {
	f, err := fsys.Open(dataPath(name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cr := csv.NewReader(f)
	cr.Comma = '\t'
	cr.LazyQuotes = true
	cr.FieldsPerRecord = -1
	recs, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	header := recs[0]
	for i, h := range header {
		h = strings.TrimPrefix(h, "#")
		if p := strings.IndexByte(h, '('); p >= 0 {
			h = h[:p]
		}
		header[i] = h
	}
	var rows []row
	for _, rec := range recs[1:] {
		r := row{}
		for i, v := range rec {
			if i < len(header) {
				r[header[i]] = v
			}
		}
		rows = append(rows, r)
	}
	return rows, nil
}

type MonsterData struct {
	ID                                    string // MT_ name
	Family                                string // graphics folder, e.g. "Zombie"; variants share it
	ImgSize                               int    // mType: graphics budget per level
	Frames, Rate                          [6]int // stand, walk, attack, hit, death, special
	Name                                  string
	MinDLvl, MaxDLvl, Level, MinHP, MaxHP int
	AI                                    string
	Flags, Int                            int
	Hit, AFNum, MinDam, MaxDam            int
	Hit2, AFNum2, MinDam2, MaxDam2        int
	AC                                    int
	Class                                 string // MC_UNDEAD, MC_DEMON, MC_ANIMAL
	MagicRes, MagicRes2, Treasure, Exp    int
	Available                             bool // MonstAvailTbl != 0
}

// Monster flag bits (_mFlags).
const (
	MFlagHidden      = 0x01
	MFlagTargetsMon  = 0x10
	MFlagGolem       = 0x20
	MFlagKnockback   = 0x80
	MFlagSearch      = 0x100
	MFlagCanOpenDoor = 0x200
	MFlagNoLifeSteal = 0x1000
)

// Monster resistance bits (mMagicRes).
const (
	ResistMagic     = 0x01
	ResistFire      = 0x02
	ResistLightning = 0x04
	ImmuneMagic     = 0x08
	ImmuneFire      = 0x10
	ImmuneLightning = 0x20
)

type UniqueMonsterData struct {
	Type                          string // MT_ name of base monster
	Name, Trn                     string
	Level, MaxHP                  int
	AI                            string
	Int, MinDam, MaxDam, MagicRes int
	UnqAttr, UnqVar1, UnqVar2     int
	TalkMsg                       string
}

// hellfireItemOrder lays the item table out as Hellfire's AllItemsList: the table file is D1 1.09's with Hellfire's
// rows in D1's empty slots 35-47 and its new rows appended, but Hellfire inserts its oils after the Potion of Full
// Rejuvenation and the Scroll of Vision after the Scroll of Healing, puts the Bovine Plate in Lightforge's slot and
// ends with the runes. Every roll that walks the table (RndItem, RndTypeItems, the stores) depends on this order.
// The Sorcerer's staff is Hellfire's Short Staff of Mana, the unused Map of the Stars its Cathedral Map, and
// DevilutionX's Arena Potion isn't in Hellfire's table.
func hellfireItemOrder(d1Items []ItemData) []ItemData {
	const (
		oils    = D1ItemCount     // Blacksmith Oil .. Oil, 4 rows
		vision  = D1ItemCount + 4 // Scroll of Vision
		runes   = D1ItemCount + 5 // the five placeable traps (Hellfire's runes), 5 rows
		bovine  = D1ItemCount + 10
		fullRej = 82 // Potion of Full Rejuvenation
		healing = 87 // Scroll of Healing
	)
	var out []ItemData
	out = append(out, d1Items[:fullRej+1]...)
	out = append(out, d1Items[oils:oils+4]...)
	out = append(out, d1Items[fullRej+1:healing+1]...)
	out = append(out, d1Items[vision])
	out = append(out, d1Items[healing+1:D1ItemCount]...)
	out = append(out, d1Items[runes:runes+5]...)
	out[32] = d1Items[bovine]
	out[5].Name, out[5].Spell = "Short Staff of Mana", "SPL_MANA"
	return out
}

// D1ItemCount is how many entries D1 1.09's table has; everything past it is Hellfire's (see AGENTS.md).
// Hellfire fills D1's thirteen empty slots 35-47 in place, so every D1 index below it stays where it was.
const D1ItemCount = 156

type ItemData struct {
	Drop, Class, Loc             string // IDROP_, ICLASS_, ILOC_
	Curs, Type                   int    // cursor graphic, ITYPE_ value
	UniqueBase                   string // UITYPE_ name uniques refer to
	Name, ShortName              string
	MinMLvl, Durability          int
	MinDam, MaxDam, MinAC, MaxAC int
	MinStr, MinMag, MinDex       int
	Flags, MiscID, Spell         string
	Value                        int
}

// ITYPE_ values.
const (
	ITypeMisc = iota
	ITypeSword
	ITypeAxe
	ITypeBow
	ITypeMace
	ITypeShield
	ITypeLArmor
	ITypeHelm
	ITypeMArmor
	ITypeHArmor
	ITypeStaff
	ITypeGold
	ITypeRing
	ITypeAmulet
	ITypeFood
)

// ICURS_ graphics the slain hero's rewards are rolled until (CreateMagicArmor/CreateMagicWeapon).
const (
	ICursLongWarBow   = 119
	ICursBreastPlate  = 153
	ICursBastardSword = 57
	ICursBattleAxe    = 101
	ICursWarStaff     = 124
	ICursGreatSword   = 134
)

// PLT_ affix item-type bits.
const (
	PLTMisc  = 0x1
	PLTBow   = 0x10
	PLTStaff = 0x100
	PLTWeap  = 0x1000
	PLTShld  = 0x10000
	PLTArmo  = 0x100000
)

var pltBits = map[string]int{"PLT_MISC": PLTMisc, "PLT_BOW": PLTBow, "PLT_STAFF": PLTStaff, "PLT_WEAP": PLTWeap, "PLT_SHLD": PLTShld, "PLT_ARMO": PLTArmo}

// D1PrefixCount, D1SuffixCount and D1UniqueItemCount are how many D1 1.09 has; the rest are Hellfire's, appended.
const (
	D1PrefixCount     = 83
	D1SuffixCount     = 95
	D1UniqueItemCount = 90
)

type Affix struct {
	Name, Power             string
	P1, P2, MinLvl, IType   int
	GOE                     int
	Double, Ok              bool
	MinVal, MaxVal, MultVal int
}

type Power struct {
	Power  string
	P1, P2 int
}

type UniqueItemData struct {
	Name, Base           string
	MinLvl, NumPL, Value int
	Powers               [6]Power
}

type SpellData struct {
	ID, Type, Name                          string
	Mana, BookLvl, StaffLvl, MinInt         int
	StaffMin, StaffMax, BookCost, StaffCost int
	ManaAdj, MinMana                        int
	Town                                    bool
}

// D1's spelldata names; the TSV only carries some of them.
var spellNames = []string{"", "Firebolt", "Healing", "Lightning", "Flash", "Identify", "Fire Wall", "Town Portal", "Stone Curse",
	"Vision", "Phasing", "Mana Shield", "Fireball", "Guardian", "Chain Lightning", "Flame Wave", "Doom Serpents",
	"Blood Ritual", "Nova", "Invisibility", "Inferno", "Golem", "Rage", "Teleport", "Apocalypse", "Etherealize",
	"Item Repair", "Staff Recharge", "Trap Disarm", "Elemental", "Charged Bolt", "Holy Bolt", "Resurrect",
	"Telekinesis", "Heal Other", "Blood Star", "Bone Spirit", "Mana", "the Magi", "the Jester", "Lightning Wall",
	"Immolation", "Warp", "Reflect", "Berserk", "Ring of Fire", "Vision", "Fire Trap", "Lightning Trap", "Nova Trap",
	"Immolation Trap", "Stone Trap"}

// SpellDescriptions holds one short item-tooltip sentence per spell, indexed like spellNames.
var SpellDescriptions = []string{"",
	"Hurls a bolt of fire at the target.",
	"Restores some of your life.",
	"Fires a crackling line of lightning at the target.",
	"Unleashes a burst of light that damages everything within 1 square of you.",
	"Reveals the magical properties of an item.",
	"Raises a wall of flame that burns anything passing through.",
	"Opens a portal back to town and returns you to where you were.",
	"Turns a monster to stone for a short while.",
	"Shows every living thing on the level and lets you see them through walls.",
	"Teleports you to a random nearby spot.",
	"Uses your mana to absorb damage instead of life.",
	"Launches an exploding ball of fire at the target.",
	"Summons a fire-breathing guardian that attacks nearby monsters.",
	"Sends lightning arcing into every monster in sight.",
	"Sends a wave of flame rolling toward the target.",
	"Summons serpents that hunt your enemies.",
	"Sacrifices your own blood for power.",
	"Blasts lightning in a ring all around you.",
	"Makes you unseen by monsters.",
	"Breathes a short stream of fire at the target.",
	"Summons a golem that fights at your side.",
	"Doubles your fury for a while; the hangover costs as much again.",
	"Instantly moves you to the chosen spot.",
	"Rains fire on every monster in sight.",
	"Makes you ethereal so physical attacks pass through you.",
	"Repairs an item at the cost of some of its durability.",
	"Restores a staff's charges at the cost of some of its maximum.",
	"Disarms a trapped door or chest.",
	"Launches a fireball that seeks out the nearest monster.",
	"Releases sparks of lightning that wander toward monsters.",
	"Fires a holy bolt that is deadly to the undead.",
	"Brings a dead ally back to life.",
	"Lets you pick up items and trigger objects from afar.",
	"Restores life to another player.",
	"Fires a star of blood that drains your life when mana runs out.",
	"Sends a spirit that tears away a third of a monster's life.",
	"Restores some mana.",
	"Restores all mana.",
	"Casts a random spell: firebolt, fireball, fire wall, guardian, chain lightning, town portal, teleport, apocalypse or stone curse.",
	"Raises a wall of lightning.",
	"Hurls fireballs in every direction.",
	"Teleports you to the nearest stairs.",
	"Turns part of the damage of blows back on the attacker.",
	"Drives a monster berserk, turning it on its own kind.",
	"Surrounds you with a ring of fire walls.",
	"Shows every living thing on the level and lets you see them through walls.",
	"Sets a trap that explodes in fire when stepped on.",
	"Sets a trap that raises a wall of lightning when stepped on.",
	"Sets a trap that releases a lightning nova when stepped on.",
	"Sets a trap that releases fireballs in every direction when stepped on.",
	"Sets a trap that turns to stone whoever steps on it."}

// D1MonsterCount is how many entries D1 1.09's table has; everything past it is Hellfire's, appended.
const D1MonsterCount = 112

// monstdat.cpp MonstAvailTbl: 0 = never spawns at random.
var monstAvail = [112]byte{
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 0, 2, 2, 2, 2, 0, 2, 2, 2, 2, 1, 1,
	1, 1, 2, 2, 2, 2, 2, 2, 2, 2, 0, 0, 2, 2, 2, 2, 0, 0, 0, 0,
	2, 2, 2, 2, 2, 2, 2, 2, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2,
	0, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 0, 0, 0,
	0, 2, 2, 2, 2, 2, 2, 2, 2, 0, 0, 0,
}

type Data struct {
	Monsters       []MonsterData
	UniqueMonsters []UniqueMonsterData
	Items          []ItemData
	Prefixes       []Affix
	Suffixes       []Affix
	UniqueItems    []UniqueItemData
	Spells         []SpellData         // index = SPL_ value
	Objects        []ObjData           // index = OBJ_ value
	Gossip         map[string][]string // town NPC internal_name -> TEXT_* ids of its gossip, greeting first (mods/d1/data/gossip)
	Texts          map[string]string   // TEXT_* id -> gossip, quest and book text, paragraphs split by "\n\n"
	TextSfx        map[string]string   // TEXT_* id -> the speech D1 plays with it (alltext's sfxnr, see SfxFiles)
	levels         levelData           // set pieces and tile tables (mods/d1/data/levels)
}

// gossipFiles maps a town NPC's internal name (its common.VendorType) to its gossip file's base
// name in mods/d1/data/gossip, ported from Diablo's own Source/Data/Text/Gossip/*.txt.
var gossipFiles = map[string]string{
	"griswold": "griswold", "adria": "adria", "pepin": "pepin", "wirt": "wirt", "cain": "story", "ogden": "ogden",
	"farnham": "farnham", "gillian": "gillian",
}

// parseTexts reads a D1 text file (Source/Data/Text): blocks of a "[TEXT_*]" header, a quoted line and a few
// numeric/sfx fields, blank-line separated. D1's "\n \n" paragraph breaks become "\n\n".
func parseTexts(data string) (ids, texts, sfx []string) {
	for _, block := range strings.Split(data, "\n\n") {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		if len(lines) < 2 || !strings.HasPrefix(lines[0], "[TEXT_") {
			continue
		}
		quoted := strings.TrimSuffix(strings.TrimSpace(strings.Trim(strings.TrimSpace(lines[1]), "\"")), "|")
		var paragraphs []string
		for _, par := range strings.Split(quoted, `\n \n`) {
			if par = strings.Join(strings.Fields(strings.ReplaceAll(par, `\n`, " ")), " "); par != "" {
				paragraphs = append(paragraphs, par)
			}
		}
		ids, texts = append(ids, strings.Trim(lines[0], "[]")), append(texts, strings.Join(paragraphs, "\n\n"))
		sfx = append(sfx, strings.TrimSpace(lines[len(lines)-1]))
	}
	return
}

// loadTexts loads every gossip and quest text by its TEXT_* id, and each NPC's gossip lines (missing files are
// skipped).
func loadTexts(fsys fs.FS) (texts, sfx map[string]string, gossip map[string][]string) {
	texts, sfx, gossip = map[string]string{}, map[string]string{}, map[string][]string{}
	for _, dir := range []string{"gossip", "text"} {
		files, _ := fs.ReadDir(fsys, dataPath(dir))
		for _, f := range files {
			data, err := fs.ReadFile(fsys, dataPath(dir, f.Name()))
			if err != nil {
				continue
			}
			ids, lines, sounds := parseTexts(string(data))
			for i, id := range ids {
				texts[id], sfx[id] = lines[i], sounds[i]
			}
			for npc, file := range gossipFiles {
				if dir == "gossip" && file+".txt" == f.Name() {
					gossip[npc] = ids
				}
			}
		}
	}
	return
}

func (d *Data) MonsterByID(id string) int {
	for i, m := range d.Monsters {
		if m.ID == id {
			return i
		}
	}
	return -1
}

// family extracts "Zombie" from `Monsters\\Zombie\\Zombie%c.CL2`.
func family(graphic string) string {
	parts := strings.FieldsFunc(graphic, func(r rune) bool { return r == '\\' })
	if len(parts) < 2 {
		return graphic
	}
	return parts[1]
}

// Load reads the tables.
func Load(fsys fs.FS) (*Data, error) {
	d, err := load(fsys)
	if d != nil {
		d.Items = hellfireItemOrder(d.Items)
		d.Spells[SplNova].BookLvl, d.Spells[SplApocalypse].BookLvl = 14, 19 // Hellfire's books of Nova and Apocalypse
	}
	return d, err
}

func load(fsys fs.FS) (*Data, error) {
	d := &Data{}
	tables := map[string][]row{}
	for _, n := range []string{"xl_monst", "xl_umon", "xl_item", "xl_ipre", "xl_isuf", "xl_uitem", "xl_spell", "xl_obj"} {
		rows, err := readTable(fsys, n+".txt")
		if err != nil {
			return nil, err
		}
		tables[n] = rows
	}
	for n, r := range tables["xl_monst"] {
		d.Monsters = append(d.Monsters, MonsterData{
			ID: r.s(""), Family: family(r.s("GraphicType")), ImgSize: r.i("mType"), Frames: r.arr("Frames"), Rate: r.arr("Rate"), Name: r.s("mName"),
			MinDLvl: r.i("mMinDLvl"), MaxDLvl: r.i("mMaxDLvl"), Level: r.i("mLevel"), MinHP: r.i("mMinHP"), MaxHP: r.i("mMaxHP"),
			AI: r.s("mAi"), Flags: r.i("mFlags"), Int: r.i("mInt"),
			Hit: r.i("mHit"), AFNum: r.i("mAFNum"), MinDam: r.i("mMinDamage"), MaxDam: r.i("mMaxDamage"),
			Hit2: r.i("mHit2"), AFNum2: r.i("mAFNum2"), MinDam2: r.i("mMinDamage2"), MaxDam2: r.i("mMaxDamage2"),
			AC: r.i("mArmorClass"), Class: r.s("mMonstClass"), MagicRes: r.i("mMagicRes"), MagicRes2: r.i("mMagicRes2"),
			Treasure: r.i("mTreasure"), Exp: r.i("mExp"), Available: n < len(monstAvail) && monstAvail[n] != 0,
		})
	}
	for _, r := range tables["xl_umon"] {
		if r.s("mtype") == "-1" {
			break
		}
		d.UniqueMonsters = append(d.UniqueMonsters, UniqueMonsterData{
			Type: r.s("mtype"), Name: r.s("mName"), Trn: r.s("mTrnName"), Level: r.i("mlevel"), MaxHP: r.i("mmaxhp"),
			AI: r.s("mAi"), Int: r.i("mint"), MinDam: r.i("mMinDamage"), MaxDam: r.i("mMaxDamage"), MagicRes: r.i("mMagicRes"),
			UnqAttr: r.i("mUnqAttr"), UnqVar1: r.i("mUnqVar1"), UnqVar2: r.i("mUnqVar2"), TalkMsg: r.s("mtalkmsg"),
		})
	}
	for _, r := range tables["xl_item"] {
		if r.s("iLoc") == "ILOC_INVALID" {
			break
		}
		d.Items = append(d.Items, ItemData{
			Drop: r.s("iRnd"), Class: r.s("iClass"), Loc: r.s("iLoc"), Curs: r.i("iCurs"), Type: r.i("itype"),
			UniqueBase: r.s("iItemId"), Name: r.s("iName"), ShortName: r.s("iSName"), MinMLvl: r.i("iMinMLvl"),
			Durability: r.i("iDurability"), MinDam: r.i("iMinDam"), MaxDam: r.i("iMaxDam"), MinAC: r.i("iMinAC"), MaxAC: r.i("iMaxAC"),
			MinStr: r.i("iMinStr"), MinMag: r.i("iMinMag"), MinDex: r.i("iMinDex"),
			Flags: r.s("iFlags"), MiscID: r.s("iMiscId"), Spell: r.s("iSpell"), Value: r.i("iValue"),
		})
	}
	affixes := func(rows []row) (out []Affix) {
		for _, r := range rows {
			if r.s("PLPower") == "IPL_INVALID" {
				break
			}
			itype := 0
			for _, p := range strings.Split(r.s("PLIType"), "|") {
				itype |= pltBits[p]
			}
			out = append(out, Affix{
				Name: r.s("PLName"), Power: r.s("PLPower"), P1: r.i("PLParam1"), P2: r.i("PLParam2"), MinLvl: r.i("PLMinLvl"),
				IType: itype, GOE: r.i("PLGOE"), Double: r.i("PLDouble") != 0, Ok: r.i("PLOk") != 0,
				MinVal: r.i("PLMinVal"), MaxVal: r.i("PLMaxVal"), MultVal: r.i("PLMultVal"),
			})
		}
		return
	}
	d.Prefixes = affixes(tables["xl_ipre"])
	d.Suffixes = affixes(tables["xl_isuf"])
	for _, r := range tables["xl_uitem"] {
		if r.s("UIItemId") == "UITYPE_INVALID" {
			break
		}
		u := UniqueItemData{Name: r.s("UIName"), Base: r.s("UIItemId"), MinLvl: r.i("UIMinLvl"), NumPL: r.i("UINumPL"), Value: r.i("UIValue")}
		for p := 0; p < 6; p++ {
			u.Powers[p] = Power{r.s(fmt.Sprintf("UIPower%d", p+1)), r.i(fmt.Sprintf("UIParam%d", 2*p+1)), r.i(fmt.Sprintf("UIParam%d", 2*p+2))}
		}
		d.UniqueItems = append(d.UniqueItems, u)
	}
	for n, r := range tables["xl_spell"] {
		name := ""
		if n < len(spellNames) {
			name = spellNames[n]
		}
		d.Spells = append(d.Spells, SpellData{
			ID: r.s("sName"), Type: r.s("sType"), Name: name, Mana: r.i("sManaCost"),
			BookLvl: r.i("sBookLvl"), StaffLvl: r.i("sStaffLvl"), MinInt: r.i("sMinInt"),
			StaffMin: r.i("sStaffMin"), StaffMax: r.i("sStaffMax"), BookCost: r.i("sBookCost"), StaffCost: r.i("sStaffCost"),
			Town: r.i("sTownSpell") != 0, ManaAdj: r.i("sManaAdj"), MinMana: r.i("sMinMana"),
		})
	}
	for _, r := range tables["xl_obj"] {
		if r.s("") == "-1" {
			break
		}
		d.Objects = append(d.Objects, ObjData{
			Name: r.s(""), Load: r.i("oload"), MinLvl: r.i("ominlvl"), MaxLvl: r.i("omaxlvl"), LvlType: r.i("olvltype"),
			Theme: r.i("otheme"), AnimFlag: r.i("oAnimFlag"), AnimDelay: r.i("oAnimDelay"), AnimLen: r.i("oAnimLen"),
			Solid: r.i("oSolidFlag") != 0, Missile: r.i("oMissFlag") != 0, Light: r.i("oLightFlag") != 0,
			Break: r.i("oBreak"), SelFlag: r.i("oSelFlag"), Trap: r.i("oTrapFlag") != 0,
		})
	}
	if len(d.Monsters) != len(monstAvail) || len(d.Items) < D1ItemCount || len(d.Spells) != NumSpells {
		return nil, fmt.Errorf("d1 tables: unexpected sizes monsters=%d items=%d spells=%d", len(d.Monsters), len(d.Items), len(d.Spells))
	}
	d.Texts, d.TextSfx, d.Gossip = loadTexts(fsys)
	levels, err := loadLevelData(fsys)
	if err != nil {
		return nil, err
	}
	d.levels = levels
	if err := loadRecTables(fsys); err != nil {
		return nil, err
	}
	return d, nil
}

// loadRecTables fills the package-level tables that used to be Go literals. They stay package-level because the
// code reading them (ItemSnds and friends) is called from all over and has no Data to hand.
func loadRecTables(fsys fs.FS) error {
	var err error
	if oils, err = loadOils(fsys); err != nil {
		return err
	}
	if itemCAnimTbl, itemDropSnds, itemInvSnds, err = loadItemSounds(fsys); err != nil {
		return err
	}
	if SfxFiles, err = loadSfxFiles(fsys); err != nil {
		return err
	}
	if err = loadObjectTables(fsys); err != nil {
		return err
	}
	if err = loadHeroTables(fsys); err != nil {
		return err
	}
	if err = loadSpellTables(fsys); err != nil {
		return err
	}
	if propertyDefs, err = loadPropertyDefs(fsys); err != nil {
		return err
	}
	if err = loadStoreTables(fsys); err != nil {
		return err
	}
	return nil
}
