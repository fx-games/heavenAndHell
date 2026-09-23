// mapedit is Heaven & Hell's map editor: it draws the levels the game reads (.hhm), set pieces included, and
// nothing else. It is a development tool, kept out of the game binary.
//
//	go run ./tools/mapedit              # the default mod
//	go run ./tools/mapedit -mod d1      # another mod under files/mods
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	topH    = 26  // the tool bar
	leftW   = 200 // the level list
	rightW  = 268 // palette and inspector
	statusH = 20
)

type app struct {
	u      ui
	modDir string // files/mods/<mod>
	status string
	err    string

	maps *mapEditor

	w, h int
}

func main() {
	mod := flag.String("mod", "heavenandhell", "the mod to edit, a name under files/mods or a path")
	flag.Parse()

	dir, err := modDir(*mod)
	if err != nil {
		log.Fatal(err)
	}
	a := &app{modDir: dir}
	if a.maps, err = newMapEditor(dir); err != nil {
		log.Fatal(err)
	}
	a.status = "editing " + dir

	ebiten.SetWindowSize(1440, 900)
	ebiten.SetWindowTitle("mapedit - " + filepath.Base(dir))
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetScreenClearedEveryFrame(true)
	if err := ebiten.RunGame(a); err != nil {
		log.Fatal(err)
	}
}

// modDir finds the mod directory, whether the tool was started from the repo root or from its own folder.
func modDir(mod string) (string, error) {
	if info, err := os.Stat(mod); err == nil && info.IsDir() {
		return filepath.Clean(mod), nil
	}
	dir, err := filepath.Abs(".")
	if err != nil {
		return "", err
	}
	for {
		if p := filepath.Join(dir, "files", "mods", mod); isDir(p) {
			return p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no mod %q: run this from the repo, or pass -mod with a path", mod)
		}
		dir = parent
	}
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func (a *app) Layout(w, h int) (int, int) { a.w, a.h = w, h; return w, h }

func (a *app) Update() error {
	a.u.update() // every edge the widgets react to is collected here, and read in Draw
	return nil
}

func (a *app) Draw(screen *ebiten.Image) {
	a.u.beginFrame()
	screen.Fill(colBG)

	body := image.Rect(0, topH, a.w, a.h-statusH)
	a.maps.draw(a, screen, body)
	a.drawTopBar(screen)
	a.drawStatus(screen)
	a.u.endFrame()
	if debugFPS {
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%.0f fps", ebiten.ActualFPS()), a.w-70, topH+4)
	}
}

var debugFPS = os.Getenv("MAPEDIT_FPS") != ""

func (a *app) drawTopBar(dst *ebiten.Image) {
	r := rect(0, 0, a.w, topH)
	fill(dst, r, colPanel)
	fill(dst, rect(0, topH-1, a.w, 1), colLine)

	a.maps.topBar(a, dst, padding, a.w)
}

func (a *app) drawStatus(dst *ebiten.Image) {
	r := rect(0, a.h-statusH, a.w, statusH)
	fill(dst, r, colPanel)
	fill(dst, rect(0, a.h-statusH, a.w, 1), colLine)
	msg, c := a.status, colTextDim
	if a.u.tooltip != "" {
		msg, c = a.u.tooltip, colText
	}
	if a.err != "" {
		msg, c = a.err, colDanger
	}
	labelClipped(dst, padding, a.h-statusH+3, a.w-2*padding, msg, c)
}

// say reports what just happened, in the status bar.
func (a *app) say(format string, args ...any) {
	a.status, a.err = fmt.Sprintf(format, args...), ""
}

// fail reports a problem, in the status bar.
func (a *app) fail(format string, args ...any) {
	a.err = fmt.Sprintf(format, args...)
}

