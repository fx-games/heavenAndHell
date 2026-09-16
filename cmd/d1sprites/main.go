// Command d1sprites renders Diablo's and Hellfire's monster, item and spell graphics out of the MPQs into
// reference PDFs for picking icons: every monster type and unique in the game's own TRN recolors (the stand
// animation's front-facing frame), every base and unique item's inventory graphic, and every spell icon, each
// labelled with the icon it is currently mapped to in files/assets/data/icons.
//
//	go run ./cmd/d1sprites -out /tmp
package main

import (
	"flag"
	"fmt"
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
	out := flag.String("out", ".", "directory to write d1_monsters.pdf, d1_items.pdf and d1_spells.pdf to")
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
		file, title string
		build       func(reader, []byte) ([]section, error)
	}{
		{"d1_monsters.pdf", "D1 + Hellfire monsters and uniques (front view)", monsters},
		{"d1_items.pdf", "D1 + Hellfire items and unique items", items},
		{"d1_spells.pdf", "D1 + Hellfire spell and skill icons", spells},
	} {
		sections, err := doc.build(read, pal)
		if err == nil {
			err = writePDF(filepath.Join(*out, doc.file), doc.title, sections)
		}
		if err != nil {
			log.Fatalf("%s: %v", doc.file, err)
		}
		fmt.Println("wrote", filepath.Join(*out, doc.file))
	}
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
		g := m.cl2[le.Uint32(m.cl2):] // direction 0 faces the viewer
		f, err := celFrame(g, 0, true)
		if err != nil {
			log.Fatal(err)
		}
		var recolor []func(int) int
		if m.trn != nil {
			recolor = append(recolor, monsterTrn(m.trn))
		}
		if uniqueTrn != nil {
			recolor = append(recolor, drawTrn(uniqueTrn))
		}
		c.img = render(decodeCl2(f), m.width, pal, recolor...)
		return c
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
		c.lines = []string{icons.name(defs.IconID("monster", id), r.s("mName")), id, "TRN: " + trn, "icon: " + icons.icon(defs.IconID("monster", id))}
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
		c.unique = true
		c.lines = []string{name, "unique " + id, "TRN: " + r.s("mTrnName") + " on top", "icon: " + icons.icon(defs.IconID("unique", name))}
		cards[familyOf[id]] = append(cards[familyOf[id]], *c)
	}
	var sections []section
	for _, fam := range order {
		sections = append(sections, section{title: fam, cards: cards[fam]})
	}
	return sections, nil
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
		return &card{img: render(decodeCel(f), sheets[sheet].widths[frame], pal)}, nil
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
		c.unique = true
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
		return &card{img: render(decodeCel(f), 56, pal)}, nil
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
		c.lines = []string{r.s("sNameText"), id, fmt.Sprintf("frame %d", frame), "icon: " + icons.icon(defs.IconID("spell", id))}
		used.cards = append(used.cards, *c)
	}
	for frame := 0; frame < int(le.Uint32(cel)); frame++ {
		if !seen[frame] {
			c, err := sprite(frame)
			if err != nil {
				return nil, err
			}
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

// table reads files/assets/data/d1/<name>.txt. Column names drop their "(i)"-style type and a leading "#";
// values lose their quotes and the doubled backslashes.
func table(name string) []row {
	b, err := os.ReadFile("files/assets/data/d1/" + name + ".txt")
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

func baseName(path string) string {
	return strings.TrimSuffix(filepath.Base(strings.ReplaceAll(path, `\`, "/")), filepath.Ext(path))
}
