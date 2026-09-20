// Command d1sprites renders Diablo's and Hellfire's graphics out of the MPQs into reference PDFs for picking
// icons, one per icons/*.rec: every monster type and unique in the game's own TRN recolors (the stand animation's
// front and right-facing frames), every base and unique item's inventory graphic, every spell and skill icon, and every
// object and dungeon megatile, each labelled with the icon it is currently mapped to. Every sprite is also saved
// at its original size as a PNG under <out>/d1_sprites/<document>/, named after its icon ID where it has one;
// monsters at 4x with their name, ID and weapon written under them.
//
//	go run -tags development ./cmd/d1sprites
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"rpg/cmd/internal/mpq"
	"rpg/defs"
)

type reader func(string) ([]byte, error)

func main() {
	out := flag.String("out", "reference", "directory for the PDFs and the d1_sprites image folders")
	flag.Parse()

	dat, err := mpq.DiabDat()
	if err != nil {
		log.Fatal(err)
	}
	// Hellfire first: it replaces some of Diablo's files, the spell icons among them
	read := reader(mpq.ReadFrom(append(mpq.HellfireArchives("reference"), dat)))
	pal, err := read(`Levels\L4Data\L4_1.PAL`)
	if err != nil {
		log.Fatal(err)
	}
	for _, doc := range []struct {
		name, title, rec string
		build            func(reader, []byte) ([]section, error)
	}{
		{"monsters", "D1 + Hellfire monsters and uniques (front and side views)", "actors", monsters},
		{"items", "D1 + Hellfire items and unique items", "items", items},
		{"skills", "D1 + Hellfire spell and skill icons", "skills", spells},
		{"tiles", "D1 + Hellfire objects and dungeon megatiles", "tiles", tiles},
	} {
		pdf := filepath.Join(*out, "d1_"+doc.name+".pdf")
		sections, err := doc.build(read, pal)
		dir := filepath.Join(*out, "d1_sprites", doc.name)
		if err == nil {
			err = savePNGs(dir, sections)
		}
		if err == nil {
			err = setReferenceImages(doc.rec, dir, sections)
		}
		if err == nil {
			err = writePDF(pdf, doc.title, sections)
		}
		if err != nil {
			log.Fatalf("%s: %v", pdf, err)
		}
		fmt.Println("wrote", pdf, "and", filepath.Join(*out, "d1_sprites", doc.name))
	}
}

// savePNGs writes every card's sprite to dir/card.file, replacing what an earlier run left there.
func savePNGs(dir string, sections []section) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	for _, s := range sections {
		for _, c := range s.cards {
			if c.img == nil {
				continue
			}
			path := filepath.Join(dir, c.file)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			img := c.img
			if c.png != nil {
				img = c.png
			}
			err = png.Encode(f, img)
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// monsters groups the monster types by sprite family, each unique after the types sharing its graphics.
func monsters(read reader, pal []byte) ([]section, error) {
	icons := loadIcons("actors")
	type monster struct {
		width int
		cl2   []byte // nil for the cut Wyrms, whose graphics never shipped
		trn   []byte
	}
	sprite := func(m monster, uniqueTrn []byte) *card {
		c := &card{}
		if m.cl2 == nil {
			return c
		}
		var recolor []func(int) int
		if m.trn != nil {
			recolor = append(recolor, monsterTrn(m.trn))
		}
		if uniqueTrn != nil {
			recolor = append(recolor, drawTrn(uniqueTrn))
		}
		frame := func(dir int) *image.NRGBA {
			f, err := celFrame(m.cl2[le.Uint32(m.cl2[4*dir:]):], 0, true)
			if err != nil {
				log.Fatal(err)
			}
			return render(decodeCl2(f), m.width, pal, recolor...)
		}
		c.img = sideBySide(12, frame(0), frame(6)) // DIR_S faces the viewer, DIR_E faces right
		return c
	}
	// the PNG is the PDF card at 4x with its name, ID and weapon written under it
	withPoster := func(c *card, fam string) {
		if c.img == nil {
			return
		}
		lines := c.lines[:2]
		if w := weapons[fam]; w != "" {
			lines = append(lines, "weapon: "+w)
		}
		c.png = poster(c.img, 4, lines)
	}

	types := map[string]monster{}
	familyOf := map[string]string{}
	cards := map[string][]card{}
	var order []string
	for _, r := range table("xl_monst") {
		id, gfx := r.s(""), r.s("GraphicType")
		m := monster{width: r.i("flags")} // the first column is the sprite width, whatever its header says
		anim := "n"
		if strings.HasPrefix(r.s("Frames"), "0") { // the Golem has no stand animation
			anim = "w"
		}
		m.cl2, _ = read(strings.Replace(gfx, "%c", anim, 1))
		trn := "base"
		if r.i("has_trans") == 1 {
			var err error
			if m.trn, err = read(r.s("TransFile")); err != nil {
				return nil, fmt.Errorf("%s: %v", id, err)
			}
			trn = baseName(r.s("TransFile"))
		}
		types[id] = m
		fam := strings.Split(gfx, `\`)[1]
		familyOf[id] = fam
		if cards[fam] == nil {
			order = append(order, fam)
		}
		c := sprite(m, nil)
		c.file, c.keys = defs.IconID("monster", id)+".png", []string{defs.IconID("monster", id)}
		c.lines = []string{icons.name(defs.IconID("monster", id), r.s("mName")), id, "TRN: " + trn, "icon: " + icons.icon(defs.IconID("monster", id))}
		withPoster(c, fam)
		cards[fam] = append(cards[fam], *c)
	}
	for _, r := range table("xl_umon") {
		id, name := r.s("mtype"), r.s("mName")
		if id == "-1" {
			break
		}
		t, err := read(`Monsters\Monsters\` + r.s("mTrnName") + ".TRN")
		if err != nil {
			return nil, fmt.Errorf("%s: %v", name, err)
		}
		c := sprite(types[id], t)
		c.unique, c.file, c.keys = true, defs.IconID("unique", name)+".png", []string{defs.IconID("unique", name)}
		c.lines = []string{name, "unique " + id, "TRN: " + r.s("mTrnName") + " on top", "icon: " + icons.icon(defs.IconID("unique", name))}
		withPoster(c, familyOf[id])
		cards[familyOf[id]] = append(cards[familyOf[id]], *c)
	}
	var sections []section
	for _, fam := range order {
		sections = append(sections, section{title: fam, cards: cards[fam]})
	}
	return sections, nil
}

// weapons is what each sprite family attacks with, as its graphics (and missiles) show it; the game data doesn't
// record it.
var weapons = map[string]string{
	"Zombie": "fists", "FalSpear": "spear", "SkelAxe": "axe", "FalSword": "sword and shield", "Scav": "claws",
	"SkelBow": "bow", "SkelSd": "sword and shield", "TSneak": "claws", "Sneak": "claws", "GoatLord": "polearm",
	"GoatMace": "mace", "Bat": "claws", "GoatBow": "bow", "Acid": "acid spit", "SKing": "two-handed sword",
	"FatC": "cleaver", "Fat": "fists", "Magma": "fists, fire bolts", "Rhino": "horns", "Demskel": "claws",
	"Thin": "claws, lightning", "Fireman": "flames", "BigFall": "club", "Gargoyle": "claws", "Mega": "claws, fire",
	"Snake": "blades", "Black": "sword", "Unrav": "claws", "Succ": "magic bolts", "Mage": "staff, spells",
	"Golem": "fists", "Diablo": "claws, spells", "DarkMage": "staff, spells", "Fork": "tusks", "Scorp": "stinger",
	"Eye": "magic bolts", "Spider": "fangs", "Spawn": "bite", "WScorp": "stinger", "Eye2": "magic bolts",
	"bSpidr": "fangs, acid", "Clasp": "pincers", "AntWorm": "fire spit", "HorkD": "fists", "Hellbug": "mandibles",
	"Gravdg": "shovel", "Rat": "claws, bite", "Hellbat": "fire bolts", "Lich": "magic bolts", "Bubba": "claws",
	"Hellbat2": "claws", "Lich2": "magic bolts", "Byclps": "axe", "Flesh": "claws", "Reaper": "scythe",
	"Nkr": "claws, spells",
}

var iTypeNames = []string{"ITYPE_MISC", "ITYPE_SWORD", "ITYPE_AXE", "ITYPE_BOW", "ITYPE_MACE", "ITYPE_SHIELD",
	"ITYPE_LARMOR", "ITYPE_HELM", "ITYPE_MARMOR", "ITYPE_HARMOR", "ITYPE_STAFF", "ITYPE_GOLD", "ITYPE_RING", "ITYPE_AMULET"}

// items groups base items by type, each unique after the other items of its base's type.
func items(read reader, pal []byte) ([]section, error) {
	icons := loadIcons("items")
	// Cursor graphics: ICSTART (12) cursors precede the items; Hellfire's past ICLAST (179) are in Objcurs2.
	// The frames don't record their widths, so they come from devilutionX's tables.
	var sheets [2]struct {
		cel    []byte
		widths []int
	}
	for i, name := range []string{"Objcurs", "Objcurs2"} {
		var err error
		if sheets[i].cel, err = read(`Data\Inv\` + name + ".CEL"); err != nil {
			return nil, err
		}
		dir := []string{"assets", "mods/hf"}[i]
		b, err := os.ReadFile("reference/devilutionX/" + dir + "/data/inv/" + strings.ToLower(name) + "-widths.txt")
		if err != nil {
			return nil, err
		}
		for _, w := range strings.Fields(string(b)) {
			n, _ := strconv.Atoi(w)
			sheets[i].widths = append(sheets[i].widths, n)
		}
	}
	sprite := func(curs int) (*card, error) {
		sheet, frame := 0, curs+12-1
		if curs+12 > 179 {
			sheet, frame = 1, curs+12-179-1
		}
		f, err := celFrame(sheets[sheet].cel, frame, true)
		if err != nil {
			return nil, fmt.Errorf("cursor %d: %v", curs, err)
		}
		return &card{img: crop(render(decodeCel(f), sheets[sheet].widths[frame], pal))}, nil
	}

	cards := make([][]card, len(iTypeNames))
	baseOf := map[string]row{}
	for _, r := range table("xl_item") {
		typ := iTypeNames[r.i("itype")]
		if _, ok := baseOf[r.s("iItemId")]; !ok {
			baseOf[r.s("iItemId")] = r
		}
		// the game's d1ItemIconName, minus the magic variants
		key := defs.IconID("item", typ)
		switch {
		case icons.has(defs.IconID("item", r.s("iName"))):
			key = defs.IconID("item", r.s("iName"))
		case icons.has(defs.IconID("item", r.s("iMiscId"))):
			key = defs.IconID("item", r.s("iMiscId"))
		case typ == "ITYPE_GOLD":
			key = "item.gold_*"
		}
		c, err := sprite(r.i("iCurs"))
		if err != nil {
			return nil, fmt.Errorf("%s: %v", r.s("iName"), err)
		}
		c.file = fmt.Sprintf("%s.curs%d.png", defs.IconID("item", r.s("iName")), r.i("iCurs"))
		c.keys = []string{key, key + ".magic", defs.IconID("item", typ)} // the type's fallback icon last
		if typ == "ITYPE_GOLD" {
			c.keys = []string{"item.gold_small", "item.gold_medium", "item.gold_large"}
		}
		c.lines = []string{r.s("iName"), fmt.Sprintf("curs %d  %s", r.i("iCurs"), r.s("iMiscId")), key, "icon: " + icons.icon(key)}
		cards[r.i("itype")] = append(cards[r.i("itype")], *c)
	}
	for _, r := range table("xl_uitem") {
		if r.s("UIItemId") == "UITYPE_INVALID" {
			break
		}
		base := baseOf[r.s("UIItemId")]
		curs := base.i("iCurs")
		for p := 1; p <= 6; p++ { // IPL_INVCURS swaps in its own graphic
			if r.s(fmt.Sprintf("UIPower%d", p)) == "IPL_INVCURS" {
				curs = r.i(fmt.Sprintf("UIParam%d", 2*p-1))
			}
		}
		key := defs.IconID("item", base.s("iName")) + ".unique"
		if !icons.has(key) {
			key = defs.IconID("item", iTypeNames[base.i("itype")])
		}
		c, err := sprite(curs)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", r.s("UIName"), err)
		}
		c.unique, c.file, c.keys = true, defs.IconID("item", r.s("UIName"))+".unique.png", []string{key}
		c.lines = []string{r.s("UIName"), fmt.Sprintf("curs %d  unique %s", curs, base.s("iName")), key, "icon: " + icons.icon(key)}
		cards[base.i("itype")] = append(cards[base.i("itype")], *c)
	}
	var sections []section
	for t, cs := range cards {
		sections = append(sections, section{title: iTypeNames[t], cards: cs})
	}
	return sections, nil
}

// spellIcons is SpellITbl: the SpelIcon.CEL frame of each spell, in xl_spell's order (devilutionX's
// panels/spell_icons.cpp, the same table as Hellfire's CONTROL.CPP).
var spellIcons = []int{26, 0, 1, 2, 3, 4, 5, 6, 7, 8, 27, 12, 11, 17, 15, 13, 17, 18, 10, 19, 14, 20, 22, 23, 24, 21, 25,
	28, 36, 37, 38, 41, 40, 39, 9, 35, 29, 50, 50, 49, 45, 46, 42, 44, 47, 48, 43, 34, 34, 34, 34, 34}

func spells(read reader, pal []byte) ([]section, error) {
	icons := loadIcons("skills")
	cel, err := read(`Data\SpelIcon.CEL`)
	if err != nil {
		return nil, err
	}
	sprite := func(frame int) (*card, error) {
		f, err := celFrame(cel, frame, false)
		if err != nil {
			return nil, err
		}
		return &card{img: crop(render(decodeCel(f), 56, pal))}, nil
	}
	var used, unused section
	used.title, unused.title = "Spells and skills", "SpelIcon.CEL frames no spell uses"
	seen := map[int]bool{}
	rows := table("xl_spell")
	if len(rows) != len(spellIcons) {
		return nil, fmt.Errorf("xl_spell has %d spells, spellIcons %d", len(rows), len(spellIcons))
	}
	for i, r := range rows {
		frame := spellIcons[i]
		seen[frame] = true
		c, err := sprite(frame)
		if err != nil {
			return nil, err
		}
		id := r.s("sName")
		c.file, c.keys = defs.IconID("spell", id)+".png", []string{defs.IconID("spell", id)}
		c.lines = []string{r.s("sNameText"), id, fmt.Sprintf("frame %d", frame), "icon: " + icons.icon(defs.IconID("spell", id))}
		used.cards = append(used.cards, *c)
	}
	for frame := 0; frame < int(le.Uint32(cel)); frame++ {
		if !seen[frame] {
			c, err := sprite(frame)
			if err != nil {
				return nil, err
			}
			c.file = fmt.Sprintf("unused/frame%02d.png", frame)
			c.lines = []string{fmt.Sprintf("frame %d", frame)}
			unused.cards = append(unused.cards, *c)
		}
	}
	return []section{used, unused}, nil
}

// row is one line of an xl_*.txt table, by column name.
type row map[string]string

func (r row) s(col string) string { return r[col] }

func (r row) i(col string) int {
	n, _ := strconv.Atoi(r[col])
	return n
}

// table reads files/mods/d1/data/<name>.txt. Column names drop their "(i)"-style type and a leading "#";
// values lose their quotes and the doubled backslashes.
func table(name string) []row {
	b, err := os.ReadFile("files/mods/d1/data/" + name + ".txt")
	if err != nil {
		log.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	header := strings.Split(lines[0], "\t")
	for i, h := range header {
		if j := strings.IndexByte(h, '('); j >= 0 {
			h = h[:j]
		}
		header[i] = strings.TrimPrefix(h, "#")
	}
	var rows []row
	for _, l := range lines[1:] {
		r := row{}
		for i, v := range strings.Split(l, "\t") {
			if i < len(header) {
				r[header[i]] = strings.ReplaceAll(strings.Trim(v, `"`), `\\`, `\`)
			}
		}
		rows = append(rows, r)
	}
	return rows
}

// icons is an icons/<name>.rec: icon number and display name by internal_name.
type icons map[string][2]string

func loadIcons(name string) icons {
	b, err := os.ReadFile("files/assets/data/icons/" + name + ".rec")
	if err != nil {
		log.Fatal(err)
	}
	m := icons{}
	var key string
	for _, l := range strings.Split(string(b), "\n") {
		k, v, _ := strings.Cut(l, ": ")
		switch k {
		case "internal_name":
			key = v
		case "icon":
			e := m[key]
			e[0] = v
			m[key] = e
		case "name":
			e := m[key]
			e[1] = v
			m[key] = e
		}
	}
	return m
}

func (m icons) has(key string) bool { _, ok := m[key]; return ok }

func (m icons) icon(key string) string {
	if e, ok := m[key]; ok {
		return e[0]
	}
	return "-"
}

func (m icons) name(key, fallback string) string {
	if e := m[key]; e[1] != "" {
		return e[1]
	}
	return fallback
}

// setReferenceImages points each entry of icons/<name>.rec at its sprite under dir (reference_image, which the
// remapper shows beside the atlas): the first card listing the entry's internal_name among its keys.
func setReferenceImages(name, dir string, sections []section) error {
	refs := map[string]string{}
	for _, s := range sections {
		for _, c := range s.cards {
			for _, k := range c.keys {
				if _, ok := refs[k]; !ok && c.img != nil {
					refs[k] = filepath.ToSlash(filepath.Join(dir, c.file))
				}
			}
		}
	}
	path := "files/assets/data/icons/" + name + ".rec"
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "reference_image: ") {
			continue
		}
		out = append(out, l)
		if key, ok := strings.CutPrefix(l, "internal_name: "); ok && refs[key] != "" {
			out = append(out, "reference_image: "+refs[key])
		}
	}
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644)
}

func baseName(path string) string {
	return strings.TrimSuffix(filepath.Base(strings.ReplaceAll(path, `\`, "/")), filepath.Ext(path))
}
