// Package d1 holds Diablo 1's data tables (loaded verbatim from assets/data) and its generation rules.
// Enum columns stay strings (e.g. "to_hit", "AI_ZOMBIE"); only bitfields are numbers.
package rules

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"slices"
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

// DataPath locates one of the active mod's data files. Diablo 1 is a mod like any other: the engine never names
// it, so every read goes to assets/data and the overlay in package files resolves it to mods/<active>/data.
func DataPath(elem ...string) string {
	return path.Join(append([]string{"assets", "data"}, elem...)...)
}

func readTable(fsys fs.FS, name string) ([]row, error) {
	f, err := fsys.Open(DataPath(name))
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
	Type                              string // MT_ name of base monster
	Name, Trn                         string
	Level, MaxHP                      int
	AI                                string
	AILevel, MinDam, MaxDam, MagicRes int
	UnqAttr, UnqVar1, UnqVar2         int
	TalkMsg                           string
}

// ItemCount is how many entries D1's table has; everything past it is Hellfire's (see AGENTS.md).
const ItemCount = 143

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

// PLT_ affix item-type bits.
const (
	PLTMisc  = 0x1
	PLTBow   = 0x10
	PLTStaff = 0x100
	PLTWeap  = 0x1000
	PLTShld  = 0x10000
	PLTArmo  = 0x100000
)

// PrefixCount, SuffixCount and UniqueItemCount are how many D1 1.09 has; the rest are Hellfire's, appended.
const (
	PrefixCount     = 83
	SuffixCount     = 95
	UniqueItemCount = 90
)

type Affix struct {
	ID, Name, Power         string
	P1, P2, MinLvl, IType   int
	GOE                     int
	Double, Ok              bool
	MinVal, MaxVal, MultVal int
}

type Power struct {
	Power  string
	P1, P2 int
}

// Item is the item with that internal_name, nil if the mod has none.
func (d *Data) Item(name string) *ItemData { return d.ItemsByName[name] }

// UniqueItemIdx is the uid of the unique item with that id, or -1.
func (d *Data) UniqueItemIdx(id string) int {
	return slices.IndexFunc(d.UniqueItems, func(u UniqueItemData) bool { return u.ID == id })
}

type UniqueItemData struct {
	ID, Name, Base       string // ID: its UIInternalName, what records refer to it by
	MinLvl, NumPL, Value int
	Powers               [6]Power
}

type SpellData struct {
	ID, Type, Name                          string
	Mana, BookLvl, StaffLvl, MinInt         int
	StaffMin, StaffMax, BookCost, StaffCost int
	ManaAdj, MinMana                        int
	Town                                    bool
	// From spells.rec: Description is one short item-tooltip sentence; Resist the resistance kind of its
	// damaging missile; Targeted whether it needs a target cell; Sfx what D1 plays for a hero's cast as _sfx_id
	// names (the cast, the launch of its missile, the impact where its bolt stops; "" is silent); Effect and
	// HitZone how it is expressed in the engine's effect model (no Effect: it still runs from code);
	// HealthCost what a cast takes out of the caster's own life, beside its mana.
	Description string
	Resist      int
	Targeted    bool
	SelfTarget  bool // a targeted spell that may also be aimed at the caster's own cell
	Sfx         [3]string
	Effect      []string
	HitZone     string
	HealthCost  int
	// Named skills only (rows past NumSpells): Targeting how a hero aims it (targeting rules, "" = like a spell),
	// Sustained a toggle that stays up until cast again, Icon the tile drawn when the mod's icons/skills.rec
	// has no spell.<id>.
	Targeting string
	Sustained bool
	Icon      int32
}

// MonsterCount is how many entries D1 1.09's table has; everything past it is Hellfire's, appended.
const MonsterCount = 112

type Data struct {
	Missiles map[string]MissileData // missiles.rec, by MIS_ id
	tables

	Monsters       []MonsterData
	UniqueMonsters []UniqueMonsterData
	Items          []ItemData
	ItemsByName    map[string]*ItemData // item internal_name -> its row of Items
	DropTables     map[string]DropTable // drops.rec: monster, floor, useful and level drops
	Vendors        map[string]Vendor    // store internal_name -> its record (assets/data/stores.rec)
	VendorList     []Vendor             // the same records in file order, which is the order an NPC's menu and stock take

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
	QuestDefs      []defs.Quest        // the mod's quests: QuestKeys' at their quest ids ("" if absent), then its own
	QuestTriggers  []defs.Trigger      // its triggers, conditions already compiled
	Texts          map[string]string   // TEXT_* id -> gossip, quest and book text, paragraphs split by "\n\n"
	TextSfx        map[string]string   // TEXT_* id -> the speech D1 plays with it (alltext's sfxnr, see SfxFiles)
}

// loadTextRec reads texts.rec: every quest, book and gossip text by its TEXT_* id, with its speech cue, and each
// town NPC's gossip ids in file order (greeting first). A mod without the file has nothing to say.
func loadTextRec(fsys fs.FS) (texts, sfx map[string]string, gossip map[string][]string, err error) {
	texts, sfx, gossip = map[string]string{}, map[string]string{}, map[string][]string{}
	f, err := fsys.Open(DataPath("texts.rec"))
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

// singleTags are the enemies.rec tags the engine looks one monster up by, so at most one monster may carry each.
var singleTags = []string{"golem", "raised_by_default"}

// MonsterByTag is the monster that carries tag, -1 for none. For singleTags only.
func (d *Data) MonsterByTag(tag string) int {
	for i, m := range d.Monsters {
		if m.Has(tag) {
			return i
		}
	}
	return -1
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
		d.ItemsByName = make(map[string]*ItemData, len(d.Items))
		for i := range d.Items {
			it := &d.Items[i]
			if _, seen := d.ItemsByName[it.InternalName]; seen {
				return nil, fmt.Errorf("items.rec: internal_name %q is used twice", it.InternalName)
			}
			d.ItemsByName[it.InternalName] = it
		}
		if err := d.readDropTables(fsys); err != nil {
			return nil, err
		}
	}
	return d, err
}

func load(fsys fs.FS) (*Data, error) {
	d := &Data{}
	var err error
	// The xl_* tables are Diablo's own: its unique monsters, its spell slots and
	// its object list. A mod that is not Diablo ships none of them and has none of those things - no spell table
	// means no spell book - so a missing file is an absent feature here,
	// not an error. Diablo itself ships all of them, so nothing about D1 changes.
	tables := map[string][]row{}
	hasD1Tables := true
	for _, n := range []string{"xl_umon", "xl_spell", "xl_obj"} {
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
	if err = loadQuests(fsys, d); err != nil {
		return nil, err
	}
	if err = loadMissiles(fsys, d); err != nil {
		return nil, err
	}
	for _, tag := range singleTags {
		if n := len(slices.DeleteFunc(slices.Clone(d.Monsters), func(m MonsterData) bool { return !m.Has(tag) })); n > 1 {
			return nil, fmt.Errorf("enemies.rec: %d monsters are tagged %s, want at most one", n, tag)
		}
	}
	for _, r := range tables["xl_umon"] {
		if r.s("mtype") == "-1" {
			break
		}
		d.UniqueMonsters = append(d.UniqueMonsters, UniqueMonsterData{
			Type: r.s("mtype"), Name: r.s("mName"), Trn: r.s("mTrnName"), Level: r.i("mlevel"), MaxHP: r.i("mmaxhp"),
			AI: r.s("mAi"), AILevel: r.i("mint"), MinDam: r.i("mMinDamage"), MaxDam: r.i("mMaxDamage"), MagicRes: r.i("mMagicRes"),
			UnqAttr: r.i("mUnqAttr"), UnqVar1: r.i("mUnqVar1"), UnqVar2: r.i("mUnqVar2"), TalkMsg: r.s("mtalkmsg"),
		})
	}
	if d.Items, err = defs.ReadItems(fsys); err != nil {
		return nil, err
	}
	var bad []string
	logErr := func(s string) { bad = append(bad, s) }
	if f, err := fsys.Open(DataPath("effects", "targeted.rec")); err == nil {
		d.TargetedDefs = defs.ReadTargetedDefinitions(f, logErr)
		f.Close()
	}
	if f, err := fsys.Open(DataPath("effects", "untargeted.rec")); err == nil {
		d.UntargetedDefs = defs.ReadUntargetedDefinitions(f, logErr)
		f.Close()
	}
	if len(bad) > 0 {
		return nil, fmt.Errorf("effects: %s", strings.Join(bad, "; "))
	}
	if d.Prefixes, d.Suffixes, err = readAffixes(fsys); err != nil {
		return nil, err
	}
	if d.UniqueItems, err = readUniqueItems(fsys); err != nil {
		return nil, err
	}
	orders, err := readRollOrders(fsys)
	if err != nil {
		return nil, err
	}
	if err := sortByRollOrder(d.Items, orders["items"], func(it ItemData) string { return it.InternalName }); err != nil {
		return nil, fmt.Errorf("items: %w", err)
	}
	for _, t := range []struct {
		name string
		s    []Affix
	}{{"prefixes", d.Prefixes}, {"suffixes", d.Suffixes}} {
		if err := sortByRollOrder(t.s, orders[t.name], func(a Affix) string { return a.ID }); err != nil {
			return nil, fmt.Errorf("%s: %w", t.name, err)
		}
	}
	if err := sortByRollOrder(d.UniqueItems, orders["unique_items"], func(u UniqueItemData) string { return u.ID }); err != nil {
		return nil, fmt.Errorf("unique items: %w", err)
	}
	for _, r := range tables["xl_spell"] { // names come from spells.rec, in loadSpellTables
		d.Spells = append(d.Spells, SpellData{
			ID: r.s("sName"), Type: r.s("sType"), Mana: r.i("sManaCost"),
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
	if hasD1Tables && (len(d.Monsters) != MonsterCount || len(d.Items) < ItemCount || len(d.Spells) > NumSpells) {
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
	if err := loadRecTables(fsys, d); err != nil {
		return nil, err
	}
	return d, nil
}

// loadRecTables fills d's rule tables (tables) from the mod's data/*.rec files.
func loadRecTables(fsys fs.FS, d *Data) error {
	var err error
	if d.oils, err = loadOils(fsys); err != nil {
		return err
	}
	if d.itemCAnimTbl, d.itemDropSnds, d.itemInvSnds, err = loadItemSounds(fsys); err != nil {
		return err
	}
	if d.SfxFiles, err = loadSfxFiles(fsys); err != nil {
		return err
	}
	if err = loadObjectTables(fsys, d); err != nil {
		return err
	}
	if err = loadSpellTables(fsys, d); err != nil { // before the classes, whose start_spells name them
		return err
	}
	if err = loadHeroTables(fsys, d); err != nil {
		return err
	}
	if d.propertyDefs, err = loadPropertyDefs(fsys); err != nil {
		return err
	}
	if err = loadStoreTables(fsys, d); err != nil {
		return err
	}
	if err = loadItemNames(fsys, d); err != nil {
		return err
	}
	if err = loadMonsterTables(fsys, d); err != nil {
		return err
	}
	return nil
}
