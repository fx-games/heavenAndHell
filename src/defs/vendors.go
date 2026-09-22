package defs

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"rpg/src/recfile"
	"rpg/src/util"
)

// ItemTypeNames names the item_type numbers a mod writes in items.rec, in that order, so a store record can
// say "sword" instead of 1. It is the vocabulary of the type field, not a mod's list.
var ItemTypeNames = []string{"misc", "sword", "axe", "bow", "mace", "shield", "larmor", "helm", "marmor",
	"harmor", "staff", "gold", "ring", "amulet", "food"}

// Vendor is the unified store record every mod supplies in assets/data/stores.rec, read by one ReadVendors the
// way monsters and items go through one ReadMonsters/ReadItems.
//
// The two games stocked vendors in two ways and this record is the union, not a compromise:
//
//   - an EXPLICIT LIST: always/sometimes name items by internal_name (Heaven&Hell's model).
//   - a FILTER over the mod's whole item table: types, misc_ids, spells and a depth window pick the
//     candidates and the vendor rolls among them (Diablo's model).
//
// A list is a filter that names its items, so both live here: `items` adds named items to whatever the
// criteria select, and a vendor that sets no criteria at all is a pure list.
//
// Everything below the criteria is a knob on how the vendor rolls, in data because it is content: how many
// items, how dear, how many tries before it settles, the fixed slots of a rotating premium stock, and the item
// types it keeps away from a class. HOW it rolls stays in the engine.
type Vendor struct {
	InternalName string

	// Who runs it and what they do, all named from the engine's vocabulary. Several records may name the same
	// npc: its menu is their services in file order, its stock their stock rolls in file order.
	NPCs     []string // the town NPCs' internal_names; several shopkeepers may keep one stock list
	Stock    string   // the stock strategy: one of VendorStocks, or "" for a vendor that sells nothing
	Buys     string   // what it buys from the hero: one of VendorBuyRules, or "" for nothing
	Services []string // menu entries: VendorServices names, some with an argument, e.g. "identify(100)"
	When     string   // an expr-lang condition over quests (as in quests.rec) for the record to count at all

	Purse          int
	ItemCount      util.Interval // how many items the vendor stocks, the always list included
	TurnoverItems  int
	SometimesCount util.Interval
	Always         []string // item internal_names always in stock
	Sometimes      []string // item internal_names it may stock

	// Criteria. Empty means "no opinion": an empty allow list allows everything, an empty deny list denies
	// nothing. A named item in Items is in regardless.
	Types, ExcludeTypes     []string
	MiscIDs, ExcludeMiscIDs []string
	ExcludeSpells           []string
	ExcludeChargedTypes     []string // types whose spell-carrying members this vendor will not touch
	Items                   []string
	MinDLvlDivisor          int // 0: any item shallow enough. n: also at least dlvl/n deep.

	MaxValue        int    // an item worth more is rolled again; 0 = no cap
	Tries           int    // 0: roll until the item is under max_value (1.09). n: also fit the hero, settling after n rolls (Hellfire)
	DoubleDrops     bool   // items marked IDROP_DOUBLE count twice in the candidate list
	SlotLevels      []int  // fixed stock slots, each an item level offset (a rotating premium stock)
	RotateBy        int    // premium: slots each new level shifts the stock down by (1.09: 2 of 6, Hellfire: 3 of 15)
	FeaturedMiscID  string // a kind of item the vendor puts out first, before the rest (Hellfire's witch books)
	FeaturedCount   util.Interval
	Shuns           map[string][]string // class name -> item types it keeps away from that class
	ShunTries       int                 // how long it keeps shunning before it gives up
	UsableElixirs   bool                // only elixirs of a stat still under the class maximum (Hellfire's HealerItemOk)
	PriceAdjust     [2]int              // {sign, divisor}: sells at value + sign*value/divisor (1.09 Wirt +1/2, Hellfire -1/4)
	RechargeUniques bool                // recharges uniques with charges too (Hellfire's WitchRechargeOk)

	program *vm.Program
}

// Cond is the compiled When, nil when the record has none.
func (v Vendor) Cond() *vm.Program { return v.program }

// The engine's vendor vocabulary. The algorithms behind each name live in the engine; a mod picks by name.
var (
	VendorStocks   = []string{"smith", "premium", "witch", "healer", "boy", "list"}
	VendorBuyRules = []string{"smith", "witch", "hellfire_smith", "hellfire_witch", "any"}
	VendorServices = []string{"buy", "buy_basic", "buy_premium", "sell", "repair", "recharge", "identify", "heal", "heal_on_visit",
		"donate", "peek", "stash", "transfer"}
)

// Accepts reports whether a vendor's criteria let it stock this base item. Depth and drop rate are the
// caller's, since they depend on the level being stocked for.
func (v Vendor) Accepts(it Item) bool {
	if slices.Contains(v.Items, it.InternalName) {
		return true
	}
	name := ""
	if it.Type >= 0 && it.Type < len(ItemTypeNames) {
		name = ItemTypeNames[it.Type]
	}
	spell := it.Spell != "" && it.Spell != "SPL_NULL"
	switch {
	case len(v.Types) > 0 && !slices.Contains(v.Types, name):
	case slices.Contains(v.ExcludeTypes, name):
	case len(v.MiscIDs) > 0 && !slices.Contains(v.MiscIDs, it.MiscID):
	case slices.Contains(v.ExcludeMiscIDs, it.MiscID):
	case spell && slices.Contains(v.ExcludeSpells, it.Spell):
	case spell && slices.Contains(v.ExcludeChargedTypes, name):
	default:
		return true
	}
	return false
}

// ReadVendors reads a mod's stores.rec. Order is the file's, so anything that walks the vendors is reproducible.
func ReadVendors(fsys fs.FS) ([]Vendor, error) {
	f, err := fsys.Open(path.Join("assets", "data", "stores.rec"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil // a game without shops
	}
	if err != nil {
		return nil, fmt.Errorf("stores.rec: %w", err)
	}
	defer f.Close()
	var out []Vendor
	for i, rec := range recfile.Read(f) {
		v, err := vendorFromRecord(rec)
		if err != nil {
			return nil, fmt.Errorf("stores.rec record %d: %w", i+1, err)
		}
		out = append(out, v)
	}
	return out, nil
}

func vendorFromRecord(rec recfile.Record) (Vendor, error) {
	v := Vendor{}
	for _, f := range rec {
		switch f.Name {
		case "internal_name":
			v.InternalName = f.Value
		case "npc":
			v.NPCs = append(v.NPCs, f.Value)
		case "stock":
			v.Stock = f.Value
		case "buys":
			v.Buys = f.Value
		case "service":
			v.Services = append(v.Services, f.Value)
		case "when":
			v.When = f.Value
		case "purse":
			v.Purse = f.AsInt()
		case "item_count":
			v.ItemCount = util.NewIntervalFromString(f.Value)
		case "turnover_items":
			v.TurnoverItems = f.AsInt()
		case "sometimes_count":
			v.SometimesCount = util.NewIntervalFromString(f.Value)
		case "always":
			v.Always = append(v.Always, f.Value)
		case "sometimes":
			v.Sometimes = append(v.Sometimes, f.Value)
		case "types":
			v.Types = append(v.Types, strings.Fields(f.Value)...)
		case "exclude_types":
			v.ExcludeTypes = append(v.ExcludeTypes, strings.Fields(f.Value)...)
		case "misc_ids":
			v.MiscIDs = append(v.MiscIDs, strings.Fields(f.Value)...)
		case "exclude_misc_ids":
			v.ExcludeMiscIDs = append(v.ExcludeMiscIDs, strings.Fields(f.Value)...)
		case "exclude_spells":
			v.ExcludeSpells = append(v.ExcludeSpells, strings.Fields(f.Value)...)
		case "exclude_charged_types":
			v.ExcludeChargedTypes = append(v.ExcludeChargedTypes, strings.Fields(f.Value)...)
		case "items":
			v.Items = append(v.Items, strings.Fields(f.Value)...)
		case "min_dlvl_divisor":
			v.MinDLvlDivisor = f.AsInt()
		case "max_value":
			v.MaxValue = f.AsInt()
		case "tries":
			v.Tries = f.AsInt()
		case "double_drops":
			v.DoubleDrops = f.AsBool()
		case "slot_levels":
			for _, s := range strings.Fields(f.Value) {
				n, err := strconv.Atoi(s)
				if err != nil {
					return v, fmt.Errorf("slot_levels: %w", err)
				}
				v.SlotLevels = append(v.SlotLevels, n)
			}
		case "featured_misc_id":
			v.FeaturedMiscID = f.Value
		case "featured_count":
			v.FeaturedCount = util.NewIntervalFromString(f.Value)
		case "rotate_by":
			v.RotateBy = f.AsInt()
		case "usable_elixirs_only":
			v.UsableElixirs = f.AsBool()
		case "recharge_uniques":
			v.RechargeUniques = f.AsBool()
		case "price_adjust":
			sign, div, ok := strings.Cut(f.Value, "/")
			n, err1 := strconv.Atoi(sign)
			d, err2 := strconv.Atoi(div)
			if !ok || err1 != nil || err2 != nil || d <= 0 || (n != 1 && n != -1) {
				return v, fmt.Errorf("price_adjust: want +1/n or -1/n, got %q", f.Value)
			}
			v.PriceAdjust = [2]int{n, d}
		case "shun_tries":
			v.ShunTries = f.AsInt()
		case "shuns":
			fields := strings.Fields(f.Value)
			if len(fields) < 2 {
				return v, fmt.Errorf("shuns: want a class and at least one item type, got %q", f.Value)
			}
			if v.Shuns == nil {
				v.Shuns = map[string][]string{}
			}
			v.Shuns[fields[0]] = append(v.Shuns[fields[0]], fields[1:]...)
		default:
			return v, fmt.Errorf("unknown field %q", f.Name)
		}
	}
	for _, t := range slices.Concat(v.Types, v.ExcludeTypes, v.ExcludeChargedTypes) {
		if !slices.Contains(ItemTypeNames, t) {
			return v, fmt.Errorf("unknown item type %q", t)
		}
	}
	if v.Stock != "" && !slices.Contains(VendorStocks, v.Stock) {
		return v, fmt.Errorf("unknown stock %q (have %v)", v.Stock, VendorStocks)
	}
	if v.Buys != "" && !slices.Contains(VendorBuyRules, v.Buys) {
		return v, fmt.Errorf("unknown buys %q (have %v)", v.Buys, VendorBuyRules)
	}
	for _, sv := range v.Services {
		name, _, _ := strings.Cut(sv, "(")
		if !slices.Contains(VendorServices, name) {
			return v, fmt.Errorf("unknown service %q (have %v)", sv, VendorServices)
		}
	}
	if v.When != "" {
		p, err := expr.Compile(v.When, expr.Env(QuestEnv{}), expr.AsBool())
		if err != nil {
			return v, fmt.Errorf("when: %w", err)
		}
		v.program = p
	}
	for _, types := range v.Shuns {
		for _, t := range types {
			if !slices.Contains(ItemTypeNames, t) {
				return v, fmt.Errorf("shuns: unknown item type %q", t)
			}
		}
	}
	return v, nil
}
