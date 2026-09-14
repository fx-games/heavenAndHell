// Package d1 holds Diablo 1's data tables (loaded verbatim from assets/data/d1) and its generation rules.
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

func readTable(fsys fs.FS, name string) ([]row, error) {
	f, err := fsys.Open(path.Join("assets", "data", "d1", name))
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
	"Infravision", "Phasing", "Mana Shield", "Fireball", "Guardian", "Chain Lightning", "Flame Wave", "Doom Serpents",
	"Blood Ritual", "Nova", "Invisibility", "Inferno", "Golem", "Blood Boil", "Teleport", "Apocalypse", "Etherealize",
	"Item Repair", "Staff Recharge", "Trap Disarm", "Elemental", "Charged Bolt", "Holy Bolt", "Resurrect",
	"Telekinesis", "Heal Other", "Blood Star", "Bone Spirit"}

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
	Spells         []SpellData // index = SPL_ value
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

func Load(fsys fs.FS) (*Data, error) {
	d := &Data{}
	tables := map[string][]row{}
	for _, n := range []string{"xl_monst", "xl_umon", "xl_item", "xl_ipre", "xl_isuf", "xl_uitem", "xl_spell"} {
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
	if len(d.Monsters) != 112 || len(d.Items) != 156 || len(d.Spells) != 37 {
		return nil, fmt.Errorf("d1 tables: unexpected sizes monsters=%d items=%d spells=%d", len(d.Monsters), len(d.Items), len(d.Spells))
	}
	return d, nil
}
