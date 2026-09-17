package main

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"os"
	"strings"
)

// card is one sprite with its caption lines; unique cards get a brown backdrop. file is where main saves the
// sprite as a PNG, relative to the document's image folder; png, when set, is saved there instead of img.
type card struct {
	img    *image.NRGBA // nil draws a "no graphics" placeholder
	png    *image.NRGBA
	file   string
	lines  []string
	unique bool
}

// section is a titled grid of cards, with optional note lines under the title.
type section struct {
	title string
	notes []string
	cards []card
}

const (
	pageW, pageH, margin = 595.0, 842.0, 36.0
	cellW, imgH, lineH   = 130.0, 110.0, 10.0
	pxPt                 = 2.0 // points per sprite pixel
)

// writePDF lays sections out as a grid of cards on A4 pages. It is a bare PDF 1.4 writer: base-14 Helvetica
// and Flate-compressed RGB images with an alpha soft mask, which is all a sprite sheet needs.
func writePDF(path, title string, sections []section) error {
	w := &pdfWriter{}
	w.obj("<< /Type /Catalog /Pages 2 0 R >>")
	w.obj("") // pages, filled in at the end
	w.obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	w.obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>")

	var pages []int
	var page strings.Builder
	var images []string
	y := pageH - margin
	flush := func() {
		var xo strings.Builder
		for _, name := range images {
			fmt.Fprintf(&xo, "/%s %s 0 R ", name, name[2:])
		}
		content := w.stream("", []byte(page.String()))
		pages = append(pages, w.obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %g %g] /Contents %d 0 R "+
			"/Resources << /Font << /F1 3 0 R /F2 4 0 R >> /XObject << %s>> >> >>", pageW, pageH, content, xo.String())))
		page.Reset()
		images = nil
		y = pageH - margin
	}
	text := func(x, y float64, bold bool, size float64, s string) {
		font := "F1"
		if bold {
			font = "F2"
		}
		s = strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`).Replace(s)
		fmt.Fprintf(&page, "0 g BT /%s %g Tf %g %g Td (%s) Tj ET\n", font, size, x, y, s)
	}

	text(margin, y-16, true, 16, title)
	y -= 30
	cols := 4
	for _, s := range sections {
		lines := 0
		for _, c := range s.cards {
			lines = max(lines, len(c.lines))
		}
		rowH := imgH + lineH*float64(lines) + 6
		head := 22 + lineH*float64(len(s.notes))
		rows := (len(s.cards) + cols - 1) / cols
		// a section longer than a page starts on a fresh one and breaks row by row from there
		if y-head-rowH < margin || (y-head-float64(rows)*rowH < margin && float64(rows)*rowH < pageH-2*margin-head) {
			flush()
		}
		text(margin, y-12, true, 12, s.title)
		for i, n := range s.notes {
			text(margin, y-22-lineH*float64(i)-6, false, 8, n)
		}
		y -= head
		for i, c := range s.cards {
			if i%cols == 0 && i > 0 {
				y -= rowH
				if y-rowH < margin {
					flush()
				}
			}
			x := margin + float64(i%cols)*cellW
			bg := "0.16 0.16 0.16"
			if c.unique {
				bg = "0.25 0.13 0.05"
			}
			fmt.Fprintf(&page, "%s rg %g %g %g %g re f\n", bg, x, y-imgH, cellW-6, imgH)
			if c.img == nil {
				fmt.Fprintf(&page, "0.7 g BT /F1 8 Tf %g %g Td (no graphics in the MPQs) Tj ET\n", x+8, y-imgH/2)
			} else {
				b := c.img.Bounds()
				iw, ih := float64(b.Dx())*pxPt, float64(b.Dy())*pxPt
				if f := min((cellW-12)/iw, (imgH-6)/ih, 1); f < 1 {
					iw, ih = iw*f, ih*f
				}
				name := fmt.Sprintf("Im%d", w.image(c.img))
				images = append(images, name)
				fmt.Fprintf(&page, "q %g 0 0 %g %g %g cm /%s Do Q\n", iw, ih, x+(cellW-6-iw)/2, y-imgH+(imgH-ih)/2, name)
			}
			for j, l := range c.lines {
				if len(l) > 31 {
					l = l[:29] + "..."
				}
				size := 7.5
				if j == 0 {
					size = 8.5
				}
				text(x, y-imgH-lineH*float64(j+1), j == 0, size, l)
			}
		}
		y -= rowH + 8
	}
	flush()

	var kids strings.Builder
	for _, p := range pages {
		fmt.Fprintf(&kids, "%d 0 R ", p)
	}
	w.objs[1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids.String(), len(pages))
	return os.WriteFile(path, w.bytes(), 0o644)
}

type pdfWriter struct {
	objs []string // object bodies, numbered from 1
}

func (w *pdfWriter) obj(body string) int {
	w.objs = append(w.objs, body)
	return len(w.objs)
}

func (w *pdfWriter) stream(dict string, data []byte) int {
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	zw.Write(data)
	zw.Close()
	return w.obj(fmt.Sprintf("<< %s /Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream", dict, z.Len(), z.String()))
}

// image adds img as an RGB XObject with its alpha as a soft mask and returns the XObject's object number. Small
// sprites get a nearest-neighbour upscale baked in, so viewers never smooth their pixels when enlarging them.
func (w *pdfWriter) image(img *image.NRGBA) int {
	b := img.Bounds()
	scale := min(4, max(1, 256/max(b.Dx(), b.Dy())))
	rgb := make([]byte, 0, b.Dx()*b.Dy()*3*scale*scale)
	alpha := make([]byte, 0, b.Dx()*b.Dy()*scale*scale)
	for y := 0; y < b.Dy()*scale; y++ {
		for x := 0; x < b.Dx()*scale; x++ {
			c := img.NRGBAAt(b.Min.X+x/scale, b.Min.Y+y/scale)
			rgb = append(rgb, c.R, c.G, c.B)
			alpha = append(alpha, c.A)
		}
	}
	dims := fmt.Sprintf("/Type /XObject /Subtype /Image /Width %d /Height %d /BitsPerComponent 8 /Interpolate false", b.Dx()*scale, b.Dy()*scale)
	mask := w.stream(dims+" /ColorSpace /DeviceGray", alpha)
	return w.stream(fmt.Sprintf("%s /ColorSpace /DeviceRGB /SMask %d 0 R", dims, mask), rgb)
}

func (w *pdfWriter) bytes() []byte {
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(w.objs))
	for i, body := range w.objs {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(w.objs)+1)
	for _, o := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(w.objs)+1, xref)
	return out.Bytes()
}
