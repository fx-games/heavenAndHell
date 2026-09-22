package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"rpg/files"
	"rpg/src/game"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// debugTints writes a PNG table of every tinted tile: debug_tints [out.png].
func debugTints(theGame *game.GameInstance, spriteSheet string, args []string) {
	out := "debug-tints.png"
	if len(args) > 0 {
		out = args[0]
	}
	f, err := files.Open(spriteSheet)
	if err != nil {
		log.Fatal(err)
	}
	atlas, err := png.Decode(f)
	f.Close()
	if err != nil {
		log.Fatal(err)
	}
	const tile, scale, cols, cellW = 8, 6, 8, 230
	const cellH = tile*scale + 60
	tints := theGame.GetFactory().DebugTints()
	img := image.NewRGBA(image.Rect(0, 0, cols*cellW, (len(tints)+cols-1)/cols*cellH))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{24, 24, 28, 255}), image.Point{}, draw.Src)
	atlasCols := int32(atlas.Bounds().Dx() / tile)
	face := basicfont.Face7x13
	for n, t := range tints {
		cx, cy := n%cols*cellW, n/cols*cellH
		sx, sy := int(t.Icon%atlasCols)*tile, int(t.Icon/atlasCols)*tile
		ox := cx + (cellW-tile*scale)/2
		for y := range tile * scale {
			for x := range tile * scale {
				r, g, b, a := atlas.At(sx+x/scale, sy+y/scale).RGBA() // ebiten's ColorScale: multiply by the tint
				if a > 0 {
					img.Set(ox+x, cy+4+y, color.RGBA{uint8(r >> 8 * uint32(t.Color.R) / 255), uint8(g >> 8 * uint32(t.Color.G) / 255),
						uint8(b >> 8 * uint32(t.Color.B) / 255), uint8(a >> 8)})
				}
			}
		}
		for i, line := range []string{t.ID, t.Cause, fmt.Sprintf("#%02x%02x%02x", t.Color.R, t.Color.G, t.Color.B)} {
			w := font.MeasureString(face, line).Ceil()
			d := font.Drawer{Dst: img, Src: image.NewUniform(color.RGBA{230, 230, 230, 255}), Face: face,
				Dot: fixed.P(cx+max((cellW-w)/2, 2), cy+tile*scale+20+i*14)}
			d.DrawString(line)
		}
	}
	o, err := os.Create(out)
	if err != nil {
		log.Fatal(err)
	}
	defer o.Close()
	if err := png.Encode(o, img); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d tinted tiles -> %s\n", len(tints), out)
}
