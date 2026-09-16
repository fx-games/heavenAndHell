package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

// scale is the nearest-neighbour upscale baked into every sprite, so PDF viewers never smooth the pixels.
const scale = 4

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

// render turns bottom-up palette indices into an image cropped to its opaque pixels, recolored by each table in
// turn and upscaled by scale.
func render(px []int, width int, pal []byte, recolor ...func(int) int) *image.NRGBA {
	h := (len(px) + width - 1) / width
	at := func(k int) image.Point { return image.Pt(k%width, h-1-k/width) }
	var r image.Rectangle
	for k, p := range px {
		if p >= 0 {
			r = r.Union(image.Rectangle{at(k), at(k).Add(image.Pt(1, 1))})
		}
	}
	img := image.NewNRGBA(image.Rect(0, 0, r.Dx()*scale, r.Dy()*scale))
	for k, p := range px {
		if p < 0 {
			continue
		}
		for _, f := range recolor {
			p = f(p)
		}
		c := color.NRGBA{pal[p*3], pal[p*3+1], pal[p*3+2], 255}
		o := at(k).Sub(r.Min).Mul(scale)
		for y := 0; y < scale; y++ {
			for x := 0; x < scale; x++ {
				img.SetNRGBA(o.X+x, o.Y+y, c)
			}
		}
	}
	return img
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
