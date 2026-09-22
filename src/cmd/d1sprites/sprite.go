package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

var le = binary.LittleEndian

// celFrame returns frame n (0-based) of a CEL/CL2 frame table starting at b, minus its skip header if it has one.
func celFrame(b []byte, n int, header bool) ([]byte, error) {
	if len(b) < 4 || n >= int(le.Uint32(b)) {
		return nil, fmt.Errorf("frame %d out of range", n)
	}
	f := b[le.Uint32(b[4+4*n:]):le.Uint32(b[8+4*n:])]
	if header {
		f = f[le.Uint16(f):]
	}
	return f, nil
}

// decodeCel expands CEL RLE: a positive control byte copies that many pixels, a negative one skips.
func decodeCel(f []byte) []int {
	var px []int
	for i := 0; i < len(f); {
		v := int(int8(f[i]))
		i++
		if v < 0 {
			px = appendN(px, -1, -v)
			continue
		}
		for _, p := range f[i : i+v] {
			px = append(px, int(p))
		}
		i += v
	}
	return px
}

// decodeCl2 expands CL2 RLE: positive skips, -1..-65 copies, below -65 repeats the next byte (-v-65) times.
func decodeCl2(f []byte) []int {
	var px []int
	for i := 0; i < len(f); {
		v := int(int8(f[i]))
		i++
		switch {
		case v >= 0:
			px = appendN(px, -1, v)
		case -v > 65:
			px = appendN(px, int(f[i]), -v-65)
			i++
		default:
			for _, p := range f[i : i-v] {
				px = append(px, int(p))
			}
			i -= v
		}
	}
	return px
}

func appendN(px []int, p, n int) []int {
	for ; n > 0; n-- {
		px = append(px, p)
	}
	return px
}

// render turns bottom-up palette indices into an image, recolored by each table in turn; -1 is transparent.
func render(px []int, width int, pal []byte, recolor ...func(int) int) *image.NRGBA {
	h := (len(px) + width - 1) / width
	img := image.NewNRGBA(image.Rect(0, 0, width, h))
	for k, p := range px {
		if p < 0 {
			continue
		}
		for _, f := range recolor {
			p = f(p)
		}
		img.SetNRGBA(k%width, h-1-k/width, color.NRGBA{pal[p*3], pal[p*3+1], pal[p*3+2], 255})
	}
	return img
}

// crop cuts img down to its opaque pixels.
func crop(img *image.NRGBA) *image.NRGBA {
	r := opaque(img)
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Bounds(), img, r.Min, draw.Src)
	return out
}

func opaque(img *image.NRGBA) image.Rectangle {
	var r image.Rectangle
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.NRGBAAt(x, y).A != 0 {
				r = r.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return r
}

// sideBySide crops same-sized frames to their opaque pixels and lays them out left to right, gap pixels apart,
// keeping their shared ground line.
func sideBySide(gap int, frames ...*image.NRGBA) *image.NRGBA {
	var rs []image.Rectangle
	var y image.Rectangle
	w := gap * (len(frames) - 1)
	for _, f := range frames {
		r := opaque(f)
		rs = append(rs, r)
		y = y.Union(image.Rect(0, r.Min.Y, 1, r.Max.Y))
		w += r.Dx()
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, y.Dy()))
	x := 0
	for i, f := range frames {
		draw.Draw(out, image.Rect(x, 0, x+rs[i].Dx(), y.Dy()), f, image.Pt(rs[i].Min.X, y.Min.Y), draw.Src)
		x += rs[i].Dx() + gap
	}
	return out
}

// poster scales img up by scale and writes lines centred under it at twice the size of basicfont, white with a
// black outline so they read on any background.
func poster(img *image.NRGBA, scale int, lines []string) *image.NRGBA {
	face := basicfont.Face7x13
	tw := 0
	for _, l := range lines {
		tw = max(tw, font.MeasureString(face, l).Ceil())
	}
	text := image.NewNRGBA(image.Rect(0, 0, tw+2, len(lines)*face.Height+2))
	d := font.Drawer{Dst: text, Face: face}
	for i, l := range lines {
		for _, o := range []image.Point{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}, {}} {
			d.Src = image.Black
			if o == (image.Point{}) {
				d.Src = image.White
			}
			d.Dot = fixed.P(1+o.X, 1+face.Ascent+i*face.Height+o.Y)
			d.DrawString(l)
		}
	}
	const textScale, pad = 2, 16
	sw, sh := img.Bounds().Dx()*scale, img.Bounds().Dy()*scale
	tw, th := text.Bounds().Dx()*textScale, text.Bounds().Dy()*textScale
	w := max(sw, tw)
	out := image.NewNRGBA(image.Rect(0, 0, w, sh+pad+th))
	xdraw.NearestNeighbor.Scale(out, image.Rect((w-sw)/2, 0, (w-sw)/2+sw, sh), img, img.Bounds(), draw.Over, nil)
	xdraw.NearestNeighbor.Scale(out, image.Rect((w-tw)/2, sh+pad, (w-tw)/2+tw, sh+pad+th), text, text.Bounds(), draw.Over, nil)
	return out
}

// monsterTrn is a monster type's TRN as InitMonsterTRN bakes it into the sprite: entries of 255 become 0.
func monsterTrn(t []byte) func(int) int {
	return func(p int) int {
		if t[p] == 255 {
			return 0
		}
		return int(t[p])
	}
}

// drawTrn is a TRN applied at draw time, as uniques' are: a straight lookup.
func drawTrn(t []byte) func(int) int {
	return func(p int) int { return int(t[p]) }
}
