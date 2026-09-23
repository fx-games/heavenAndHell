package hhm

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// parsed caches projects by the bytes they were read from, so a level that is rebuilt does not re-parse the
// mod's whole map set, and a different mod's maps of the same name are never mistaken for it. Reading the
// files again is cheap; parsing them is what this saves. Projects are only read.
var parsed sync.Map // [sha256.Size]byte -> *Project

// Load reads a whole maps directory: the tileset, then every level file in it, in name order.
func Load(fsys fs.FS, dir string) (*Project, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("maps %s: %w", dir, err)
	}
	names := []string{TilesetFile}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), Ext) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names[1:])
	blobs := make([][]byte, len(names))
	h := sha256.New()
	for i, name := range names {
		if blobs[i], err = fs.ReadFile(fsys, path.Join(dir, name)); err != nil {
			return nil, fmt.Errorf("maps %s: %w", dir, err)
		}
		h.Write([]byte(name))
		h.Write(blobs[i])
	}
	var key [sha256.Size]byte
	copy(key[:], h.Sum(nil))
	if p, ok := parsed.Load(key); ok {
		return p.(*Project), nil
	}
	ts, err := ReadTileset(blobs[0])
	if err != nil {
		return nil, fmt.Errorf("%s: %w", TilesetFile, err)
	}
	p := &Project{Tileset: ts}
	for i, name := range names[1:] {
		lvl, err := ReadLevel(blobs[i+1])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		p.Levels = append(p.Levels, lvl)
	}
	parsed.Store(key, p)
	return p, nil
}

// Save writes the whole project into dir: the tileset and one file per level.
func Save(dir string, p *Project) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, TilesetFile), WriteTileset(p.Tileset), 0o644); err != nil {
		return err
	}
	for _, l := range p.Levels {
		if err := SaveLevel(dir, l); err != nil {
			return err
		}
	}
	return nil
}

// SaveLevel writes one level file into dir.
func SaveLevel(dir string, l *Level) error {
	return os.WriteFile(filepath.Join(dir, l.Name+Ext), WriteLevel(l), 0o644)
}

// ReadTileset parses a tileset.hht.
func ReadTileset(b []byte) (*Tileset, error) {
	ts := &Tileset{Flags: map[int]Flags{}}
	sec := ""
	for _, ln := range lines(b) {
		n, line := ln.n, ln.s
		if head, rest, ok := header(line); ok {
			if sec = head; sec == "tileset" {
				ts.Name = rest
			}
			continue
		}
		f := strings.Split(line, "\t")
		switch sec {
		case "tileset":
			if len(f) < 2 {
				return nil, fmt.Errorf("line %d: %q needs a value", n, f[0])
			}
			switch f[0] {
			case "image":
				ts.Image = f[1]
			case "grid":
				ts.GridSize = atoi(f[1])
			case "size":
				if len(f) < 3 {
					return nil, fmt.Errorf("line %d: size needs a width and a height", n)
				}
				ts.Width, ts.Height = atoi(f[1]), atoi(f[2])
			}
		case "flags":
			if len(f) < 2 {
				continue
			}
			id, err := strconv.Atoi(strings.TrimSpace(f[0]))
			if err != nil {
				return nil, fmt.Errorf("line %d: tile id %q", n, f[0])
			}
			var fl Flags
			for _, name := range strings.Split(f[1], ",") {
				fl.set(name)
			}
			ts.Flags[id] = fl
		}
	}
	if ts.Name == "" || ts.Image == "" || ts.GridSize <= 0 || ts.Width <= 0 || ts.Height <= 0 {
		return nil, fmt.Errorf("needs a name, an image, a grid size and a size")
	}
	return ts, nil
}

// WriteTileset renders a tileset.hht.
func WriteTileset(ts *Tileset) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# the tileset every level draws from\n[tileset] %s\nimage\t%s\ngrid\t%d\nsize\t%d\t%d\n\n",
		ts.Name, filepath.ToSlash(ts.Image), ts.GridSize, ts.Width, ts.Height)
	fmt.Fprintf(&b, "# per-tile markers: %s\n[flags]\n", strings.Join(FlagNames, ", "))
	ids := make([]int, 0, len(ts.Flags))
	for id, f := range ts.Flags {
		if f.any() {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	for _, id := range ids {
		fmt.Fprintf(&b, "%d\t%s\n", id, strings.Join(ts.Flags[id].Names(), ","))
	}
	return []byte(b.String())
}

// ReadLevel parses one .hhm.
func ReadLevel(b []byte) (*Level, error) {
	l := &Level{}
	props := map[string]string{}
	sec, row := "", 0
	for _, ln := range lines(b) {
		n, line := ln.n, ln.s
		if head, rest, ok := header(line); ok {
			sec, row = head, 0
			switch sec {
			case "map":
				l.Name = rest
			case "tiles":
				f := strings.Fields(rest)
				if len(f) != 2 {
					return nil, fmt.Errorf("line %d: [tiles] needs a width and a height", n)
				}
				l.Width, l.Height = atoi(f[0]), atoi(f[1])
				if l.Width <= 0 || l.Height <= 0 {
					return nil, fmt.Errorf("line %d: [tiles] is %dx%d", n, l.Width, l.Height)
				}
				l.Tiles = make([]int, l.Width*l.Height)
				for i := range l.Tiles {
					l.Tiles[i] = Empty
				}
			}
			continue
		}
		switch sec {
		case "map":
			k, v, _ := strings.Cut(line, "\t")
			props[k] = v
		case "tiles":
			if row >= l.Height {
				return nil, fmt.Errorf("line %d: more tile rows than the declared height %d", n, l.Height)
			}
			fields := strings.Fields(line)
			if len(fields) > l.Width {
				return nil, fmt.Errorf("line %d: %d tiles, want %d", n, len(fields), l.Width)
			}
			for x, f := range fields {
				id, err := strconv.Atoi(f)
				if err != nil {
					return nil, fmt.Errorf("line %d: tile %q", n, f)
				}
				l.Tiles[row*l.Width+x] = id
			}
			row++
		case "entities":
			e, err := readEntity(line)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", n, err)
			}
			l.Entities = append(l.Entities, e)
		}
	}
	if l.Name == "" {
		return nil, fmt.Errorf("no [map] section")
	}
	if l.Tiles == nil {
		return nil, fmt.Errorf("no [tiles] section")
	}
	l.Props = readProps(props)
	return l, nil
}

func readProps(m map[string]string) Props {
	get := func(name string) string {
		for _, f := range LevelFields {
			if f.Name == name {
				if v, ok := m[name]; ok {
					return v
				}
				return f.Default
			}
		}
		return m[name]
	}
	return Props{
		DisplayName:     get("DisplayName"),
		Weight:          atoi(get("Weight")),
		IsDecoration:    get("IsDecoration") == "true",
		UsesDecoration:  get("UsesDecoration"),
		SlotID:          atoi(get("SlotID")),
		PlugsIntoSlotID: atoi(get("PlugsIntoSlotID")),
		CanBeLit:        get("CanBeLit") == "true",
	}
}

func propValue(p Props, name string) string {
	switch name {
	case "DisplayName":
		return p.DisplayName
	case "Weight":
		return strconv.Itoa(p.Weight)
	case "IsDecoration":
		return strconv.FormatBool(p.IsDecoration)
	case "UsesDecoration":
		return p.UsesDecoration
	case "SlotID":
		return strconv.Itoa(p.SlotID)
	case "PlugsIntoSlotID":
		return strconv.Itoa(p.PlugsIntoSlotID)
	case "CanBeLit":
		return strconv.FormatBool(p.CanBeLit)
	}
	return ""
}

// SetProp writes one level field by name, so an editor can drive the form from LevelFields alone.
func SetProp(p *Props, name, value string) {
	switch name {
	case "DisplayName":
		p.DisplayName = value
	case "Weight":
		p.Weight = atoi(value)
	case "IsDecoration":
		p.IsDecoration = value == "true"
	case "UsesDecoration":
		p.UsesDecoration = value
	case "SlotID":
		p.SlotID = atoi(value)
	case "PlugsIntoSlotID":
		p.PlugsIntoSlotID = atoi(value)
	case "CanBeLit":
		p.CanBeLit = value == "true"
	}
}

// Prop reads one level field by name.
func Prop(p Props, name string) string { return propValue(p, name) }

// WriteLevel renders one .hhm.
func WriteLevel(l *Level) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "[map] %s\n", l.Name)
	for _, f := range LevelFields {
		v := propValue(l.Props, f.Name)
		if v == f.Default {
			continue // an omitted field reads back as its default
		}
		fmt.Fprintf(&b, "%s\t%s\n", f.Name, escape(v))
	}

	width := 2
	for _, v := range l.Tiles {
		width = max(width, len(strconv.Itoa(v)))
	}
	fmt.Fprintf(&b, "\n[tiles] %d %d\n", l.Width, l.Height)
	row := make([]string, l.Width)
	for y := 0; y < l.Height; y++ {
		for x := 0; x < l.Width; x++ {
			row[x] = fmt.Sprintf("%*d", width, l.Tiles[y*l.Width+x])
		}
		b.WriteString(strings.Join(row, " ") + "\n")
	}

	b.WriteString("\n[entities]\n")
	sorted := append([]*Entity(nil), l.Entities...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, c := sorted[i], sorted[j]
		if a.Y != c.Y {
			return a.Y < c.Y
		}
		return a.X < c.X
	})
	for _, e := range sorted {
		b.WriteString(writeEntity(e) + "\n")
	}
	return []byte(b.String())
}

func readEntity(line string) (*Entity, error) {
	f := strings.Split(line, "\t")
	if len(f) < 3 {
		return nil, fmt.Errorf("an entity needs a kind, an x and a y")
	}
	kind, ok := KindByName(f[0])
	if !ok {
		return nil, fmt.Errorf("unknown entity %q", f[0])
	}
	e := NewEntity(kind.Name, atoi(f[1]), atoi(f[2]))
	for _, kv := range f[3:] {
		if k, v, ok := strings.Cut(kv, "="); ok {
			e.F[k] = unescape(v)
		}
	}
	return e, nil
}

func writeEntity(e *Entity) string {
	kind, _ := KindByName(e.Kind)
	f := []string{e.Kind, strconv.Itoa(e.X), strconv.Itoa(e.Y)}
	for _, def := range kind.Fields {
		v := e.F[def.Name]
		if v == def.Default {
			continue
		}
		f = append(f, def.Name+"="+escape(v))
	}
	return strings.Join(f, "\t")
}

// numbered is one meaningful line and where it came from.
type numbered struct {
	n int
	s string
}

// lines yields the file's meaningful lines with their 1-based numbers, dropping blanks and comments.
func lines(b []byte) []numbered {
	var out []numbered
	for i, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, " \t\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, numbered{i + 1, line})
	}
	return out
}

func header(line string) (name, rest string, ok bool) {
	if !strings.HasPrefix(line, "[") {
		return "", "", false
	}
	end := strings.IndexByte(line, ']')
	if end < 0 {
		return strings.TrimSpace(line[1:]), "", true
	}
	return strings.TrimSpace(line[1:end]), strings.TrimSpace(line[end+1:]), true
}

func atoi(s string) int { v, _ := strconv.Atoi(strings.TrimSpace(s)); return v }

func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\t", `\t`).Replace(s)
}

func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
