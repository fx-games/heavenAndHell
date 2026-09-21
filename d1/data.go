// Package d1 holds Diablo 1's data tables (loaded verbatim from assets/data) and its generation rules.
// Enum columns stay strings (e.g. "IPL_TOHIT", "AI_ZOMBIE"); only bitfields are numbers.
package d1

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"

	"rpg/defs"
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

// dataPath locates one of the active mod's data files. Diablo 1 is a mod like any other: the engine never names
// it, so every read goes to assets/data and the overlay in package files resolves it to mods/<active>/data.
func dataPath(elem ...string) string {
	return path.Join(append([]string{"assets", "data"}, elem...)...)
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

// MonsterData is the unified monster record (defs.Monster), not a D1-shaped one. Diablo is a mod like any
// other: its monsters come out of mods/d1/data/enemies.rec through the same defs.ReadMonsters every mod uses.
type MonsterData = defs.Monster

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

// ItemData is the unified item record (defs.Item). Diablo's items come out of mods/d1/data/items.rec
// through the same defs.ReadItems every mod uses.
type ItemData = defs.Item

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

// spellNames are D1's spelldata names and SpellDescriptions one short item-tooltip sentence each, both indexed
// by spell slot. Filled by Load from the slot section of assets/data/spells.rec.
var (
	spellNames        []string
	SpellDescriptions []string
)

// D1MonsterCount is how many entries D1 1.09's table has; everything past it is Hellfire's, appended.
const D1MonsterCount = 112

type Data struct {
	Monsters       []MonsterData
	UniqueMonsters []UniqueMonsterData
	Items          []ItemData
	Prefixes       []Affix
	Suffixes       []Affix
	UniqueItems    []UniqueItemData
	Spells         []SpellData         // index = SPL_ value
	Objects        []ObjData           // index = OBJ_ value
	Gossip         map[string][]string // town NPC internal_name -> TEXT_* ids of its gossip, greeting first (assets/data/gossip)
	QuestDefs      []defs.Quest        // the mod's quests, in quest-id order (assets/data/quests.rec)
	QuestTriggers  []defs.Trigger      // its triggers, conditions already compiled
	Texts          map[string]string   // TEXT_* id -> gossip, quest and book text, paragraphs split by "\n\n"
	TextSfx        map[string]string   // TEXT_* id -> the speech D1 plays with it (alltext's sfxnr, see SfxFiles)
	levels         levelData           // set pieces and tile tables (assets/data/levels)
}

// gossipFiles maps a town NPC's internal name (its common.VendorType) to its gossip file's base
// name in assets/data/gossip, ported from Diablo's own Source/Data/Text/Gossip/*.txt.
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
	var err error
	tables := map[string][]row{}
	for _, n := range []string{"xl_umon", "xl_ipre", "xl_isuf", "xl_uitem", "xl_spell", "xl_obj"} {
		rows, err := readTable(fsys, n+".txt")
		if err != nil {
			return nil, err
		}
		tables[n] = rows
	}
	if d.Monsters, err = defs.ReadMonsters(fsys); err != nil {
		return nil, err
	}
	if d.QuestDefs, d.QuestTriggers, err = defs.ReadQuests(fsys); err != nil {
		return nil, err
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
	if d.Items, err = defs.ReadItems(fsys); err != nil {
		return nil, err
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
	if len(d.Monsters) != D1MonsterCount || len(d.Items) < D1ItemCount || len(d.Spells) != NumSpells {
		return nil, fmt.Errorf("d1 tables: unexpected sizes monsters=%d items=%d spells=%d", len(d.Monsters), len(d.Items), len(d.Spells))
	}
	d.Texts, d.TextSfx, d.Gossip = loadTexts(fsys)
	levels, err := loadLevelData(fsys)
	if err != nil {
		return nil, err
	}
	d.levels = levels
	if err := loadRecTables(fsys, d); err != nil {
		return nil, err
	}
	return d, nil
}

// loadRecTables fills the package-level tables that used to be Go literals. They stay package-level because the
// code reading them (ItemSnds and friends) is called from all over and has no Data to hand.
func loadRecTables(fsys fs.FS, d *Data) error {
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
	if err = loadObjectTables(fsys, d); err != nil {
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
	if err = loadMonsterTables(fsys); err != nil {
		return err
	}
	return nil
}
