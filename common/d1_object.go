package common

import (
	"fmt"
	"image/color"
	"rpg/d1"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
	"strings"
)

// D1Object is a D1 dungeon object (decision 12), operated by bumping into it.
type D1Object struct {
	O   *d1.Object
	f   *Factory
	pos geometry.Point
}

type d1ObjectLook struct {
	name         string
	icon, usedIc int32
	tint         color.RGBA
}

var (
	white   = color.RGBA{255, 255, 255, 255}
	usedObj = color.RGBA{110, 110, 110, 255}
)

// ponytail: icons are first guesses from the H&H tileset, like the monster icons in milestone 1.
var d1ObjectLooks = map[int]d1ObjectLook{
	d1.ObjChest1: {"Small Chest", 128, 127, white}, d1.ObjChest2: {"Chest", 128, 127, white},
	d1.ObjChest3:     {"Large Chest", 128, 127, color.RGBA{255, 220, 120, 255}},
	d1.ObjSarc:       {"Sarcophagus", 152, 152, white},
	d1.ObjBarrel:     {"Barrel", 125, 125, white},
	d1.ObjBarrelEx:   {"Barrel", 126, 126, color.RGBA{255, 140, 120, 255}},
	d1.ObjShrineL:    {"Shrine", 75, 75, white},
	d1.ObjShrineR:    {"Shrine", 75, 75, white},
	d1.ObjSkelBook:   {"Skeleton Tome", 154, 154, white},
	d1.ObjBookcaseL:  {"Bookcase", 181, 182, white},
	d1.ObjBookcaseR:  {"Bookcase", 181, 182, white},
	d1.ObjBookstand:  {"Library Book", 154, 154, white},
	d1.ObjBloodFtn:   {"Blood Fountain", 153, 153, color.RGBA{230, 40, 40, 255}},
	d1.ObjDecap:      {"Decapitated Body", 135, 135, white},
	d1.ObjPurifyFtn:  {"Purifying Spring", 153, 153, white},
	d1.ObjArmorStand: {"Armor", 199, 199, white},
	d1.ObjGoatShrine: {"Goat Shrine", 75, 75, color.RGBA{255, 120, 120, 255}},
	d1.ObjCauldron:   {"Cauldron", 75, 75, color.RGBA{120, 255, 120, 255}},
	d1.ObjMurkyFtn:   {"Murky Pool", 153, 153, color.RGBA{90, 140, 90, 255}},
	d1.ObjTearFtn:    {"Fountain of Tears", 153, 153, color.RGBA{220, 240, 255, 255}},
	d1.ObjWeaponRack: {"Weapon Rack", 646, 646, white},
	d1.ObjLever:      {"Lever", 154, 154, color.RGBA{255, 120, 120, 255}},
	d1.ObjLazStand:   {"Staff Stand", 140, 199, white},
}

func (o *D1Object) Name() string {
	if k := o.O.Kind; k == d1.ObjShrineL || k == d1.ObjShrineR {
		return d1.ShrineNames[o.O.Var1] + " Shrine"
	}
	return d1ObjectLooks[o.O.Kind].name
}

func (o *D1Object) broken() bool {
	return !o.O.Active && (o.O.Kind == d1.ObjBarrel || o.O.Kind == d1.ObjBarrelEx)
}

func (o *D1Object) Icon(uint64) int32 {
	if o.broken() {
		return 214 // debris
	}
	if l := d1ObjectLooks[o.O.Kind]; o.O.Active {
		return l.icon
	} else {
		return l.usedIc
	}
}

func (o *D1Object) TintColor() color.RGBA {
	if !o.O.Active && o.O.Kind != d1.ObjBloodFtn && o.O.Kind != d1.ObjPurifyFtn {
		return usedObj
	}
	return d1ObjectLooks[o.O.Kind].tint
}

func (o *D1Object) MapPosition() geometry.Point       { return o.pos }
func (o *D1Object) SetMapPosition(p geometry.Point)   { o.pos = p }
func (o *D1Object) IsWalkable(*Actor) bool            { return o.broken() }
func (o *D1Object) IsTransparent() bool               { return true }
func (o *D1Object) IsPassableForProjectile() bool     { return true }
func (o *D1Object) Turn()                             {}
func (o *D1Object) ActorWalkedOn(*Actor, func() bool) {}

func (o *D1Object) ActorBumpedInto(actor *Actor, _ func() bool) {
	if actor == o.f.state.GetPlayer() && actor.hero != nil {
		o.f.operateD1Object(o, actor)
	}
}

func d1ObjectAt(m *gridmap.GridMap[*Actor, *Item, Object], p geometry.Point) *D1Object {
	if !m.Contains(p) || !m.IsObjectAt(p) {
		return nil
	}
	o, _ := m.ObjectAt(p).(*D1Object)
	return o
}

// d1World places generated objects, theme monsters and items on a level map.
type d1World struct {
	f *Factory
	m *gridmap.GridMap[*Actor, *Item, Object]
}

func (w *d1World) Occupied(x, y int) bool {
	p := geometry.Point{X: x, Y: y}
	return !w.m.Contains(p) || w.m.IsActorAt(p) || w.m.IsObjectAt(p) || w.m.IsItemAt(p) || !w.m.IsSpecialAt(p, gridmap.SpecialTileNone)
}

func (w *d1World) AddObject(o *d1.Object) {
	p := geometry.Point{X: o.X, Y: o.Y}
	w.m.AddObject(&D1Object{O: o, f: w.f, pos: p}, p)
}

func (w *d1World) AddMonster(mt, x, y int) {
	p := geometry.Point{X: x, Y: y}
	if len(w.m.Actors()) >= 200 || !w.m.CanPlaceActorHere(p) { // MAXMONSTERS
		return
	}
	a := w.f.NewD1Monster(mt)
	a.d1Monster.SpawnIndex = len(w.m.Actors())
	a.SetDrawPosition(p.ToPointF())
	w.m.AddActor(a, p)
}

// AddItem drops an item at the cell or the nearest free one (GetSuperItemLoc), deterministically.
func (w *d1World) AddItem(it *d1.Item, x, y int) {
	if it == nil {
		return
	}
	p := geometry.Point{X: x, Y: y}
	free := func(q geometry.Point) bool { return w.m.CanPlaceItemHere(q) && !w.m.IsObjectAt(q) }
	if !free(p) {
		cells := freeCellsNear(w.m, p, 1, free)
		if len(cells) == 0 {
			return
		}
		p = cells[0]
	}
	w.m.AddItem(w.f.NewD1Item(it), p)
}

// operateD1Object ports OperateObject/BreakBarrel for the player.
// ponytail: operating takes no time, like H&H's other bump objects; traps are deferred (see D1_PLAN.md).
func (f *Factory) operateD1Object(o *D1Object, player *Actor) {
	obj, gen, st := o.O, f.d1, player.GetStats()
	dlvl := d1DungeonLevel(f.state)
	hell := dlvl > 12
	drop := func(it *d1.Item) {
		if it != nil {
			f.state.AddItemToMapWithDistribution(f.NewD1Item(it), o.pos)
		}
	}
	msg := f.logError
	switch obj.Kind { // fountains that can be used again and again
	case d1.ObjBloodFtn, d1.ObjPurifyFtn:
		res := st.GetResource(stats.Health)
		if obj.Kind == d1.ObjPurifyFtn {
			res = st.GetResource(stats.Mana)
		}
		if res.IsAtMax() {
			msg(fmt.Sprintf("You drink from the %s, but nothing happens.", o.Name()))
			return
		}
		res.Increase(1)
		msg(fmt.Sprintf("You drink from the %s.", o.Name()))
		return
	}
	if !obj.Active {
		return
	}
	obj.Active = false
	switch obj.Kind {
	case d1.ObjChest1, d1.ObjChest2, d1.ObjChest3:
		for j := 0; j < obj.Var1; j++ {
			if obj.Var2 != 0 {
				drop(gen.FloorItem(dlvl, false, hell))
			} else {
				drop(gen.UsefulItem(dlvl))
			}
		}
		if obj.Var1 == 0 {
			msg(fmt.Sprintf("The %s is empty.", o.Name()))
		}
	case d1.ObjSarc:
		if obj.Var1 <= 2 {
			drop(gen.FloorItem(dlvl, false, hell))
		}
		if obj.Var1 >= 8 {
			f.d1Skeleton(obj.Skeleton, o.pos, player)
		}
	case d1.ObjBarrel, d1.ObjBarrelEx:
		f.breakD1Barrel(o, player)
	case d1.ObjShrineL, d1.ObjShrineR:
		f.d1Shrine(player, obj.Var1, o.pos)
	case d1.ObjGoatShrine, d1.ObjCauldron:
		f.d1Shrine(player, gen.RandomShrine(dlvl, true), o.pos)
	case d1.ObjSkelBook, d1.ObjBookstand:
		if gen.Rng.Intn(5) != 0 {
			drop(gen.TypeItem(dlvl, d1.ITypeMisc, "IMISC_SCROLL", false))
		} else {
			drop(gen.TypeItem(dlvl, d1.ITypeMisc, "IMISC_BOOK", false))
		}
	case d1.ObjBookcaseL, d1.ObjBookcaseR:
		drop(gen.TypeItem(dlvl, d1.ITypeMisc, "IMISC_BOOK", false))
	case d1.ObjDecap:
		drop(gen.FloorItem(dlvl, false, hell))
	case d1.ObjArmorStand:
		unique := gen.Rng.Intn(2) != 0
		switch {
		case dlvl <= 5:
			drop(gen.TypeItem(dlvl, d1.ITypeLArmor, "", true))
		case dlvl <= 9:
			drop(gen.TypeItem(dlvl, d1.ITypeMArmor, "", unique))
		case dlvl <= 12:
			drop(gen.TypeItem(dlvl, d1.ITypeHArmor, "", false))
		default:
			drop(gen.TypeItem(dlvl, d1.ITypeHArmor, "", true))
		}
	case d1.ObjWeaponRack:
		t := [4]int{d1.ITypeSword, d1.ITypeAxe, d1.ITypeBow, d1.ITypeMace}[gen.Rng.Intn(4)]
		drop(gen.TypeItem(dlvl, t, "", dlvl > 4))
	case d1.ObjMurkyFtn:
		player.AddStatusEffect(StatusInfravision, d1.InfravisionTurns(2*((dlvl-1)/4+1)))
		msg("You drink from the Murky Pool. You can see in the dark.")
	case d1.ObjLazStand:
		for i, b := range gen.D.Items {
			if b.Name == d1StaffName {
				drop(gen.PlainItem(i))
			}
		}
		msg("The Staff of Lazarus falls from its stand.")
	case d1.ObjLever:
		msg("You pull the lever. Something rumbles in the distance.")
		UpdateD1Quests(f.state, f.state.GetMap())
	case d1.ObjTearFtn:
		from := gen.Rng.Intn(4)
		to := gen.Rng.Intn(3)
		if to >= from {
			to++
		}
		f.d1ModifyStat(player, from, -1)
		f.d1ModifyStat(player, to, 1)
		msg("You drink from the Fountain of Tears.")
	}
}

// SpawnD1Skeleton ports M_SpawnSkel for the Skeleton King: one of the level's skeletons rises next to pos.
func (f *Factory) SpawnD1Skeleton(pos geometry.Point) {
	mt := f.d1.D.MonsterByID("MT_WSKELAX")
	for _, a := range f.state.GetMap().Actors() {
		if mon := a.d1Monster; mon != nil && mon.Unique == nil && strings.HasPrefix(mon.Data.Family, "Skel") {
			mt = mon.Type
			break
		}
	}
	f.d1Skeleton(mt, pos, f.state.GetPlayer())
}

func (f *Factory) d1Skeleton(mt int, pos geometry.Point, player *Actor) {
	m := f.state.GetMap()
	if mt < 0 {
		return
	}
	cells := freeCellsNear(m, pos, 1, m.CanPlaceActorHere)
	if len(cells) == 0 {
		return
	}
	a := f.NewD1Monster(mt)
	a.d1Monster.SpawnIndex = -1 // saved in LevelSnapshot.Spawned
	a.SetDrawPosition(cells[0].ToPointF())
	m.AddActor(a, cells[0])
	f.state.OnActorSpawned(a)
	d1Alert(a, player)
}

// breakD1Barrel ports BreakBarrel: a normal barrel holds an item or a skeleton, an exploding one burns its 3×3 area
// and sets off exploding barrels next to it.
func (f *Factory) breakD1Barrel(o *D1Object, player *Actor) {
	obj, gen, m := o.O, f.d1, f.state.GetMap()
	obj.Active = false // the debris stays, walkable
	if obj.Kind == d1.ObjBarrel {
		dlvl := d1DungeonLevel(f.state)
		if obj.Var2 <= 1 {
			it := gen.UsefulItem(dlvl)
			if obj.Var3 != 0 {
				it = gen.FloorItem(dlvl, false, dlvl > 12)
			}
			f.state.AddItemToMapWithDistribution(f.NewD1Item(it), o.pos)
		}
		if obj.Var2 >= 8 {
			f.d1Skeleton(obj.Skeleton, o.pos, player)
		}
		return
	}
	cause := NewActorCause("Exploding barrel", player)
	for y := o.pos.Y - 1; y <= o.pos.Y+1; y++ {
		for x := o.pos.X - 1; x <= o.pos.X+1; x++ {
			p := geometry.Point{X: x, Y: y}
			if a, ok := m.TryGetActorAt(p); ok && a.IsAlive() {
				dam := 0
				switch {
				case a.hero != nil && d1Rng.Intn(100) < 40: // PlayerMHit with a fire missile from no monster
					dam64 := 8<<6 + d1Rng.Intn(8<<6+1)
					dam64 -= dam64 * a.HeroSheet().FR / 100
					dam = dam64 >> 6
				case a.d1Monster != nil: // MonsterTrapHit
					mon := a.d1Monster
					if mon.MagicRes&d1.ImmuneFire != 0 || d1Rng.Intn(100) >= min(max(90-mon.AC, 5), 95) {
						break
					}
					dam = d1Rng.Intn(4) + 1
					if mon.MagicRes&d1.ResistFire != 0 {
						dam >>= 2
					}
					d1Alert(a, player)
				}
				if dam > 0 {
					f.actionBroker.QueueForExecution(NewChangeResourceAction(cause, a, stats.Health, -float64(dam)))
				}
			}
			if other := d1ObjectAt(m, p); other != nil && other.O.Kind == d1.ObjBarrelEx && other.O.Active {
				f.breakD1Barrel(other, player)
			}
		}
	}
}

// d1ModifyStat ports ModifyPlr{Str,Mag,Dex,Vit} and CheckStats.
func (f *Factory) d1ModifyStat(player *Actor, attr, n int) {
	dhp, dmana := player.hero.AddBaseStat(attr, n)
	SyncHero(player)
	player.GetStats().GetResource(stats.Health).Increase(float64(dhp >> 6))
	player.GetStats().GetResource(stats.Mana).Increase(float64(dmana >> 6))
}

// d1OwnedItems lists the player's D1 items: equipped first, then the pack.
func d1OwnedItems(player *Actor) (equipped, pack []*Item) {
	eq, inv := player.GetEquipment(), player.GetInventory()
	for _, it := range eq.AllItems() {
		if it.d1 != nil {
			equipped = append(equipped, it)
		}
	}
	for _, it := range inv.Items() {
		if it.d1 != nil && !eq.IsEquipped(it) {
			pack = append(pack, it)
		}
	}
	return
}

func isD1Weapon(it *d1.Item) bool {
	switch it.Type {
	case d1.ITypeSword, d1.ITypeAxe, d1.ITypeBow, d1.ITypeMace, d1.ITypeStaff:
		return true
	}
	return false
}

func isD1Armor(it *d1.Item) bool {
	switch it.Type {
	case d1.ITypeShield, d1.ITypeLArmor, d1.ITypeHelm, d1.ITypeMArmor, d1.ITypeHArmor:
		return true
	}
	return false
}

// d1Shrine ports OperateShrine for single player.
func (f *Factory) d1Shrine(player *Actor, shrine int, pos geometry.Point) {
	h, gen, m, st := player.hero, f.d1, f.state.GetMap(), player.GetStats()
	dlvl := d1DungeonLevel(f.state)
	lt := (dlvl-1)/4 + 1
	hp, mana := st.GetResource(stats.Health), st.GetResource(stats.Mana)
	equipped, pack := d1OwnedItems(player)
	all := append(append([]*Item(nil), equipped...), pack...)
	learn := func(spell int) { // Fascinating, Sacred, Ornate: +2 spell levels for 10% of base mana
		h.SpellLevels[spell] = min(h.SpellLevels[spell]+2, 15)
		cut := h.MaxManaBase / 10
		h.MaxManaBase -= cut
		h.ManaBase = max(h.ManaBase-cut, 0)
		SyncHero(player)
		mana.Decrease(cut >> 6)
	}
	switch shrine {
	case 0: // Mysterious
		for attr := d1.AttrStr; attr <= d1.AttrVit; attr++ {
			f.d1ModifyStat(player, attr, -1)
		}
		f.d1ModifyStat(player, gen.Rng.Intn(4), 6)
	case 1: // Hidden
		var durable []*Item
		for _, it := range equipped {
			if d := it.d1; d.MaxDur != 0 && d.MaxDur != 255 {
				d.Durability, d.MaxDur = min(d.Durability+10, d.MaxDur+10), d.MaxDur+10
				durable = append(durable, it)
			}
		}
		if len(durable) > 0 {
			d := durable[gen.Rng.Intn(len(durable))].d1
			d.Durability, d.MaxDur = max(d.Durability-20, 1), max(d.MaxDur-20, 1)
		}
	case 2: // Gloomy
		for _, it := range all {
			if d := it.d1; isD1Armor(d) {
				d.AC += 2
			} else if isD1Weapon(d) {
				d.MaxDam = max(d.MaxDam-1, d.MinDam)
			}
		}
	case 3: // Weird
		for _, it := range all {
			if isD1Weapon(it.d1) {
				it.d1.MaxDam++
			}
		}
	case 4, 11: // Magical
		player.AddStatusEffect(StatusManaShield, -1)
	case 5: // Stone
		for _, it := range all {
			if it.d1.Type == d1.ITypeStaff {
				it.d1.Charges = it.d1.MaxCharges
			}
		}
	case 6: // Religious
		for _, it := range all {
			it.d1.Durability = it.d1.MaxDur
		}
	case 7: // Enchanted
		var known []int
		for s, l := range h.SpellLevels {
			if l > 0 {
				known = append(known, s)
			}
		}
		if len(known) > 1 {
			for _, s := range known {
				h.SpellLevels[s] = min(h.SpellLevels[s]+1, 15)
			}
			s := known[gen.Rng.Intn(len(known))]
			h.SpellLevels[s] = max(h.SpellLevels[s]-2, 0)
		}
	case 8: // Thaumaturgic
		for _, obj := range m.Objects() {
			if o, ok := obj.(*D1Object); ok && o.O.Kind >= d1.ObjChest1 && o.O.Kind <= d1.ObjChest3 {
				o.O.Active = true
			}
		}
	case 9: // Fascinating
		learn(1)
	case 10: // Cryptic
		f.actionBroker.Execute(NewSpellAction(player, d1.SplNova, 2*lt, player.MapPosition()))
		mana.SetToMax()
	case 12: // Eldritch
		for _, it := range pack {
			rejuv := map[string]string{"IMISC_HEAL": "IMISC_REJUV", "IMISC_MANA": "IMISC_REJUV",
				"IMISC_FULLHEAL": "IMISC_FULLREJUV", "IMISC_FULLMANA": "IMISC_FULLREJUV"}[it.d1.MiscID]
			if rejuv == "" {
				continue
			}
			for idx, b := range gen.D.Items {
				if b.Drop != "IDROP_NEVER" && b.MiscID == rejuv {
					player.GetInventory().Remove(it)
					player.GetInventory().Add(f.NewD1Item(gen.PlainItem(idx)))
					break
				}
			}
		}
	case 13: // Eerie
		f.d1ModifyStat(player, d1.AttrMag, 2)
	case 14: // Divine
		drop := func(misc string) {
			if it := gen.TypeItem(dlvl, d1.ITypeMisc, misc, false); it != nil {
				f.state.AddItemToMapWithDistribution(f.NewD1Item(it), pos)
			}
		}
		if 2*dlvl < 7 {
			drop("IMISC_FULLMANA")
			drop("IMISC_FULLHEAL")
		} else {
			drop("IMISC_FULLREJUV")
			drop("IMISC_FULLREJUV")
		}
		hp.SetToMax()
		mana.SetToMax()
	case 15: // Holy
		f.d1RandomTeleport(player)
	case 16: // Sacred
		learn(14)
	case 17: // Spiritual
		inv := player.GetInventory()
		for n := inv.maxItemStacks - len(inv.StackedItems()); n > 0; n-- {
			st.IncrementBy(stats.Gold, 5*lt+gen.Rng.Intn(10*lt))
		}
	case 19: // Abandoned
		f.d1ModifyStat(player, d1.AttrDex, 2)
	case 20: // Creepy
		f.d1ModifyStat(player, d1.AttrStr, 2)
	case 21: // Quiet
		f.d1ModifyStat(player, d1.AttrVit, 2)
	case 22: // Secluded
		m.SetAllExplored()
	case 23: // Ornate
		learn(31)
	case 24: // Glimmering
		for _, it := range all {
			if it.d1.Magical != 0 {
				it.d1.Identified = true
			}
		}
	}
	for _, it := range all {
		it.RefreshD1()
	}
	SyncHero(player)
	f.RefreshD1Spells(player)
	f.logError(d1.ShrineMessages[shrine])
}
