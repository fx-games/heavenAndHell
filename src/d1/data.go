// Package d1 holds Diablo 1's data tables (loaded verbatim from assets/data) and its generation rules.
// Enum columns stay strings (e.g. "IPL_TOHIT", "AI_ZOMBIE"); only bitfields are numbers.
package d1

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strconv"
	"strings"

	"rpg/src/defs"
	"rpg/src/recfile"
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
	// Lines starting with "##" document the table and are skipped; a single "#" is part of D1's own header row.
	raw, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	var kept []string
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "##") {
			kept = append(kept, line)
		}
	}
	cr := csv.NewReader(strings.NewReader(strings.Join(kept, "\n")))
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
	// This shuffle is Hellfire's own table layout, by fixed row number. A mod whose items.rec is not Diablo's
	// table has no such rows and keeps the order it wrote them in, which is the order its own content depends on.
	if len(d1Items) <= bovine {
		return d1Items
	}
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

// Vendor is the unified store record (defs.Vendor). Diablo's stores come out of mods/d1/data/stores.rec
// through the same defs.ReadVendors every mod uses.
type Vendor = defs.Vendor

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
	ItemIdx        map[string]int    // item internal_name -> index into Items
	Vendors        map[string]Vendor // store internal_name -> its record (assets/data/stores.rec)
	VendorList     []Vendor          // the same records in file order, which is the order an NPC's menu and stock take

	// The mod's named skills and item/shrine actions (effects/targeted.rec, effects/untargeted.rec): what a
	// class's skill list or an item's action names, with its display name, icon and default arguments.
	TargetedDefs   map[string]defs.TargetedActionDefinition
	UntargetedDefs map[string]defs.UntargetedActionDefinition
	Prefixes       []Affix
	Suffixes       []Affix
	UniqueItems    []UniqueItemData
	Spells         []SpellData         // index = SPL_ value
	SpellCount     int                 // the mod's MAX_SPELLS: its spell table's rows, before Spells is padded to NumSpells
	Objects        []ObjData           // index = OBJ_ value
	Gossip         map[string][]string // town NPC internal_name -> TEXT_* ids of its gossip, greeting first (texts.rec)
	QuestDefs      []defs.Quest        // the mod's quests, in quest-id order (assets/data/quests.rec)
	QuestTriggers  []defs.Trigger      // its triggers, conditions already compiled
	Texts          map[string]string   // TEXT_* id -> gossip, quest and book text, paragraphs split by "\n\n"
	TextSfx        map[string]string   // TEXT_* id -> the speech D1 plays with it (alltext's sfxnr, see SfxFiles)
	levels         levelData           // set pieces and tile tables (assets/data/levels)
}

// loadTextRec reads texts.rec: every quest, book and gossip text by its TEXT_* id, with its speech cue, and each
// town NPC's gossip ids in file order (greeting first). A mod without the file has nothing to say.
func loadTextRec(fsys fs.FS) (texts, sfx map[string]string, gossip map[string][]string, err error) {
	texts, sfx, gossip = map[string]string{}, map[string]string{}, map[string][]string{}
	f, err := fsys.Open(dataPath("texts.rec"))
	if errors.Is(err, fs.ErrNotExist) {
		return texts, sfx, gossip, nil
	}
	if err != nil {
		return nil, nil, nil, err
	}
	defer f.Close()
	for i, r := range recfile.ReadMulti(f)["text"] {
		m := r.ToMap("")
		id := m.GetStringOrDefault("id", "")
		if id == "" {
			return nil, nil, nil, fmt.Errorf("texts.rec: text %d has no id", i+1)
		}
		texts[id] = m.GetStringOrDefault("text", "")
		if speech := m.GetStringOrDefault("sfx", ""); speech != "" { // a text may be silent
			sfx[id] = speech
		}
		if npc := m.GetStringOrDefault("gossip", ""); npc != "" {
			gossip[npc] = append(gossip[npc], id)
		}
	}
	return texts, sfx, gossip, nil
}

func (d *Data) MonsterByID(id string) int {
	for i, m := range d.Monsters {
		if m.ID == id {
			return i
		}
	}
	return -1
}

// Load reads the tables.
func Load(fsys fs.FS) (*Data, error) {
	d, err := load(fsys)
	if d != nil {
		d.Items = hellfireItemOrder(d.Items)
		d.ItemIdx = make(map[string]int, len(d.Items))
		for i, it := range d.Items {
			if _, seen := d.ItemIdx[it.InternalName]; !seen {
				d.ItemIdx[it.InternalName] = i
			}
		}
	}
	return d, err
}

func load(fsys fs.FS) (*Data, error) {
	d := &Data{}
	var err error
	// The xl_* tables are Diablo's own: its unique monsters, its affixes, its unique items, its spell slots and
	// its object list. A mod that is not Diablo ships none of them and has none of those things - no affix table
	// means items roll plain, no spell table means no spell book - so a missing file is an absent feature here,
	// not an error. Diablo itself ships all six, so nothing about D1 changes.
	tables := map[string][]row{}
	hasD1Tables := true
	for _, n := range []string{"xl_umon", "xl_ipre", "xl_isuf", "xl_uitem", "xl_spell", "xl_obj"} {
		rows, err := readTable(fsys, n+".txt")
		if errors.Is(err, fs.ErrNotExist) {
			hasD1Tables = false
			continue
		}
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
	var bad []string
	logErr := func(s string) { bad = append(bad, s) }
	if f, err := fsys.Open(dataPath("effects", "targeted.rec")); err == nil {
		d.TargetedDefs = defs.ReadTargetedDefinitions(f, logErr)
		f.Close()
	}
	if f, err := fsys.Open(dataPath("effects", "untargeted.rec")); err == nil {
		d.UntargetedDefs = defs.ReadUntargetedDefinitions(f, logErr)
		f.Close()
	}
	if len(bad) > 0 {
		return nil, fmt.Errorf("effects: %s", strings.Join(bad, "; "))
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
	if hasD1Tables && (len(d.Monsters) != D1MonsterCount || len(d.Items) < D1ItemCount || len(d.Spells) > NumSpells) {
		return nil, fmt.Errorf("d1 tables: unexpected sizes monsters=%d items=%d spells=%d", len(d.Monsters), len(d.Items), len(d.Spells))
	}
	// Spell slots are indexed by SPL_ value all over the engine. A mod with no spell table gets the full run of
	// empty slots rather than a short slice, so "this game has no spells" reads as every spell being blank
	// instead of as an index panic on the first lookup.
	d.SpellCount = len(d.Spells)
	for len(d.Spells) < NumSpells {
		d.Spells = append(d.Spells, SpellData{})
	}
	if d.Texts, d.TextSfx, d.Gossip, err = loadTextRec(fsys); err != nil {
		return nil, err
	}
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
	if err = loadStoreTables(fsys, d); err != nil {
		return err
	}
	if err = loadMonsterTables(fsys); err != nil {
		return err
	}
	return nil
}
