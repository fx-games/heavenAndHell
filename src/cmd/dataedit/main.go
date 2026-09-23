// Command dataedit browses and edits a mod's data/*.rec files: pick a file, pick a record, edit its fields.
// Edits rewrite single lines in place, so comments, order and %rec sections survive a save.
//
//	go run ./src/cmd/dataedit <mod>
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"strconv"

	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	xdraw "golang.org/x/image/draw"

	"rpg/src/recfile"

	. "modernc.org/tk9.0"
)

var fieldLine = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9_]*):[\t ]?(.*)$`)

type recFile struct {
	path  string
	lines []string
	dirty bool
}

// record is a block of consecutive non-blank lines holding at least one field; fields are its field line indices.
// ponytail: "+" continuation lines (texts.rec) are left untouched and not editable here.
type record struct {
	title  string
	fields []int
}

func (f *recFile) records() []record {
	var out []record
	section := ""
	for i := 0; i < len(f.lines); {
		if strings.HasPrefix(f.lines[i], "%rec:") {
			section = strings.TrimSpace(f.lines[i][5:])
		}
		if strings.TrimSpace(f.lines[i]) == "" {
			i++
			continue
		}
		var r record
		vals := map[string]string{}
		for ; i < len(f.lines) && strings.TrimSpace(f.lines[i]) != ""; i++ {
			if m := fieldLine.FindStringSubmatch(f.lines[i]); m != nil {
				r.fields = append(r.fields, i)
				if _, ok := vals[m[1]]; !ok {
					vals[m[1]] = m[2]
				}
			}
		}
		if len(r.fields) == 0 {
			continue
		}
		for _, k := range []string{"internal_name", "name", "id", "map"} {
			if vals[k] != "" {
				r.title = vals[k]
				break
			}
		}
		if r.title == "" {
			r.title = f.lines[r.fields[0]]
		}
		if section != "" && section != "default" {
			r.title = section + ": " + r.title
		}
		out = append(out, r)
	}
	return out
}

type icon struct {
	index int
	ref   string
}

var (
	icons = map[string]icon{} // icons/*.rec: internal_name -> atlas index and reference_image
	sheet image.Image
)

func loadIcons(modDir string) {
	recs, _ := filepath.Glob(filepath.Join(modDir, "data", "icons", "*.rec"))
	for _, p := range recs {
		fh, err := os.Open(p)
		if err != nil {
			continue
		}
		for _, r := range recfile.Read(fh) {
			m := r.ToMap("|")
			icons[m["internal_name"]] = icon{recfile.StrInt(m["icon"]), m["reference_image"]}
		}
		fh.Close()
	}
	if fh, err := os.Open(filepath.Join(modDir, "classic_roguelike.png")); err == nil {
		sheet, _ = png.Decode(fh)
		fh.Close()
	}
}

// iconFor finds a record's picture: its icon field, else any icon named <kind>.<internal_name>.
func iconFor(f *recFile, r record) (icon, bool) {
	var id string
	for _, i := range r.fields {
		m := fieldLine.FindStringSubmatch(f.lines[i])
		if m[1] == "icon" {
			if ic, ok := icons[m[2]]; ok {
				return ic, true
			}
		}
		if m[1] == "internal_name" || m[1] == "id" {
			id = strings.ToLower(m[2])
		}
	}
	for k, ic := range icons {
		if id != "" && strings.HasSuffix(k, "."+id) {
			return ic, true
		}
	}
	return icon{}, false
}

// tilePNG cuts one 8x8 tile from the sheet, scaled up nearest-neighbour.
func tilePNG(index, scale int) []byte {
	if sheet == nil {
		return nil
	}
	cols := sheet.Bounds().Dx() / 8
	src := image.Rect(index%cols*8, index/cols*8, index%cols*8+8, index/cols*8+8)
	tile := image.NewRGBA(image.Rect(0, 0, 8, 8))
	draw.Draw(tile, tile.Bounds(), sheet, src.Min, draw.Src)
	out := image.NewRGBA(image.Rect(0, 0, 8*scale, 8*scale))
	for y := range 8 * scale {
		for x := range 8 * scale {
			out.Set(x, y, tile.At(x/scale, y/scale))
		}
	}
	var b bytes.Buffer
	png.Encode(&b, out)
	return b.Bytes()
}

// fitPNG scales a PNG down to at most maxH pixels tall, so a big reference card leaves the field list room.
func fitPNG(b []byte, maxH int) []byte {
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil || img.Bounds().Dy() <= maxH {
		return b
	}
	w := img.Bounds().Dx() * maxH / img.Bounds().Dy()
	out := image.NewRGBA(image.Rect(0, 0, max(w, 1), maxH))
	xdraw.ApproxBiLinear.Scale(out, out.Bounds(), img, img.Bounds(), draw.Src, nil)
	var buf bytes.Buffer
	png.Encode(&buf, out)
	return buf.Bytes()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: dataedit <mod>")
		os.Exit(2)
	}
	modDir := filepath.Join("files", "mods", os.Args[1])
	paths, _ := filepath.Glob(filepath.Join(modDir, "data", "*.rec"))
	more, _ := filepath.Glob(filepath.Join(modDir, "data", "effects", "*.rec"))
	paths = append(paths, more...)
	if len(paths) == 0 {
		fmt.Printf("no data/*.rec under %s (run from the repo root)\n", modDir)
		os.Exit(1)
	}
	loadIcons(modDir)
	var files []*recFile
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			panic(err)
		}
		files = append(files, &recFile{path: p, lines: strings.Split(string(b), "\n")})
	}

	App.WmTitle("Data Editor: " + os.Args[1])
	WmGeometry(App, "1100x700+40+40") // on screen at start; the widgets' own sizes add up wider than a laptop
	right := App.TFrame()             // preview, fields and form: its fixed rows never push the lists off screen
	fileFrame, fileTree := scrolled(App, Show("tree"), Selectmode("browse"))
	recFrame, recTree := scrolled(App, Show("tree"), Selectmode("browse"))
	fieldFrame, fieldTree := scrolled(right.Window, Show("headings"), Selectmode("browse"), Columns("field value"))
	fieldTree.Heading("field", Txt("Field"))
	fieldTree.Heading("value", Txt("Value"))
	fieldTree.Column("field", Width(160), Stretch(false))
	fieldTree.Column("value", Width(420))
	pics := right.TFrame()
	tileLbl := pics.TLabel()
	refLbl := pics.TLabel()
	Grid(tileLbl, Row(0), Column(0), Padx(4))
	Grid(refLbl, Row(0), Column(1), Padx(4))

	form := right.TFrame()
	keyEntry := form.TEntry(Textvariable(""), Width(20))
	valEntry := form.TEntry(Textvariable(""), Width(60))
	status := App.TLabel(Txt(""))

	var cur *recFile
	curRec := -1
	editLine := -1 // field line being edited; -1: the form adds a new field
	fileIDs, recIDs, fieldIDs := map[string]*recFile{}, map[string]int{}, map[string]int{}

	title := func() {
		n := 0
		for _, f := range files {
			if f.dirty {
				n++
			}
		}
		status.Configure(Txt(fmt.Sprintf("%d unsaved file(s)   Select a field to edit, Enter applies, empty value deletes, Ctrl-S saves", n)))
	}
	var refPNG []byte // the shown record's reference image, unscaled
	var refImg *Img
	refH := 0           // the height it was last scaled to
	showRef := func() { // scales the reference image to a third of the window; on every resize
		h, _ := strconv.Atoi(WinfoHeight(App))
		if refPNG == nil || max(h/3, 64) == refH {
			return
		}
		refH = max(h/3, 64)
		old := refImg
		refImg = NewPhoto(Data(fitPNG(refPNG, refH)))
		refLbl.Configure(Image(refImg))
		if old != nil {
			old.Delete()
		}
	}
	showRecord := func() {
		fieldTree.Delete(fieldTree.Children(""))
		clear(fieldIDs)
		editLine = -1
		keyEntry.Configure(Textvariable(""))
		valEntry.Configure(Textvariable(""))
		tileLbl.Configure(Image(""))
		refLbl.Configure(Image(""), Txt(""))
		refPNG, refH = nil, 0
		recs := cur.records()
		if curRec < 0 || curRec >= len(recs) {
			return
		}
		r := recs[curRec]
		for _, i := range r.fields {
			m := fieldLine.FindStringSubmatch(cur.lines[i])
			fieldIDs[fieldTree.Insert("", "end", Values([]string{m[1], m[2]}))] = i
		}
		if ic, ok := iconFor(cur, r); ok {
			if b := tilePNG(ic.index, 8); b != nil {
				tileLbl.Configure(Image(NewPhoto(Data(b))))
			}
			if b, err := os.ReadFile(ic.ref); err == nil {
				refPNG = b
				showRef()
			}
		}
	}
	showRecords := func() {
		recTree.Delete(recTree.Children(""))
		clear(recIDs)
		for i, r := range cur.records() {
			recIDs[recTree.Insert("", "end", Txt(r.title))] = i
		}
		curRec = -1
		showRecord()
	}
	apply := func() {
		if cur == nil || curRec < 0 {
			return
		}
		key, val := strings.TrimSpace(keyEntry.Textvariable()), strings.TrimSpace(valEntry.Textvariable())
		if !fieldLine.MatchString(key + ":") {
			MessageBox(Msg("Field names are letters, digits and _"), Title("Bad field"))
			return
		}
		switch {
		case editLine >= 0 && val == "":
			cur.lines = slices.Delete(cur.lines, editLine, editLine+1)
		case editLine >= 0:
			cur.lines[editLine] = key + ": " + val
		case val != "":
			fields := cur.records()[curRec].fields
			at := fields[len(fields)-1] + 1
			cur.lines = slices.Insert(cur.lines, at, key+": "+val)
		default:
			return
		}
		cur.dirty = true
		title()
		// ponytail: the record list is rebuilt from lines; a title edit shows after reselecting the file
		showRecord()
	}
	save := func() {
		for _, f := range files {
			if !f.dirty {
				continue
			}
			if err := os.WriteFile(f.path, []byte(strings.Join(f.lines, "\n")), 0o644); err != nil {
				MessageBox(Msg(err.Error()), Title("Save failed"))
				return
			}
			f.dirty = false
		}
		title()
	}

	for _, f := range files {
		rel, _ := filepath.Rel(filepath.Join(modDir, "data"), f.path)
		fileIDs[fileTree.Insert("", "end", Txt(strings.TrimSuffix(rel, ".rec")))] = f
	}
	Bind(fileTree, "<<TreeviewSelect>>", Command(func() {
		if sel := fileTree.Selection(""); len(sel) > 0 {
			cur = fileIDs[sel[0]]
			showRecords()
		}
	}))
	Bind(recTree, "<<TreeviewSelect>>", Command(func() {
		if sel := recTree.Selection(""); len(sel) > 0 {
			curRec = recIDs[sel[0]]
			showRecord()
		}
	}))
	Bind(fieldTree, "<<TreeviewSelect>>", Command(func() {
		if sel := fieldTree.Selection(""); len(sel) > 0 {
			editLine = fieldIDs[sel[0]]
			m := fieldLine.FindStringSubmatch(cur.lines[editLine])
			keyEntry.Configure(Textvariable(m[1]))
			valEntry.Configure(Textvariable(m[2]))
			Focus(valEntry)
		}
	}))
	Bind(valEntry, "<Return>", Command(apply))
	Bind(App, "<Control-s>", Command(save))
	Bind(App, "<Configure>", Command(showRef)) // fires for every child too; showRef skips unchanged heights
	Bind(App, "<Command-s>", Command(save))

	Grid(form.TLabel(Txt("Field")), Row(0), Column(0))
	Grid(keyEntry, Row(0), Column(1))
	Grid(form.TLabel(Txt("Value")), Row(0), Column(2))
	Grid(valEntry, Row(0), Column(3), Sticky("ew"))
	Grid(form.TButton(Txt("Apply"), Command(apply)), Row(0), Column(4))
	Grid(form.TButton(Txt("New field"), Command(func() {
		editLine = -1
		fieldTree.Selection("remove", fieldTree.Selection(""))
		keyEntry.Configure(Textvariable(""))
		valEntry.Configure(Textvariable(""))
		Focus(keyEntry)
	})), Row(0), Column(5))
	Grid(form.TButton(Txt("Save"), Command(save)), Row(0), Column(6))
	GridColumnConfigure(form, 3, Weight(1))

	Grid(pics, Row(0), Column(0), Sticky("w"))
	Grid(fieldFrame, Row(1), Column(0), Sticky("nsew"))
	Grid(form, Row(2), Column(0), Sticky("ew"))
	GridRowConfigure(right, 1, Weight(1))
	GridColumnConfigure(right, 0, Weight(1))
	Grid(fileFrame, Row(0), Column(0), Sticky("ns"))
	Grid(recFrame, Row(0), Column(1), Sticky("ns"))
	Grid(right, Row(0), Column(2), Sticky("nsew"))
	Grid(status, Row(1), Column(0), Columnspan(3), Sticky("w"))
	GridRowConfigure(App, 0, Weight(1))
	GridColumnConfigure(App, 2, Weight(1))
	title()

	WmProtocol(App, "WM_DELETE_WINDOW", func() {
		if slices.ContainsFunc(files, func(f *recFile) bool { return f.dirty }) &&
			MessageBox(Msg("Save changes before quitting?"), Type("yesno")) == "yes" {
			save()
		}
		Destroy(App)
	})
	App.Wait()
}

// scrolled makes a tree with a scrollbar in its own frame, so each list grids independently.
func scrolled(parent *Window, opts ...Opt) (*TFrameWidget, *TTreeviewWidget) {
	fr := parent.TFrame()
	tree := fr.TTreeview(opts...)
	sb := fr.TScrollbar(Orient("vertical"))
	tree.Configure(Yscrollcommand(func(e *Event) { e.ScrollSet(sb) }))
	sb.Configure(Command(func(e *Event) { e.Yview(tree) }))
	Grid(tree, Row(0), Column(0), Sticky("nsew"))
	Grid(sb, Row(0), Column(1), Sticky("ns"))
	GridRowConfigure(fr, 0, Weight(1))
	GridColumnConfigure(fr, 0, Weight(1))
	return fr, tree
}
