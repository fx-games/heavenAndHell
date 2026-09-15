package d1

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// MPQ reads files from D1's DIABDAT.MPQ (format v1, PKWARE implode, encrypted tables and files).
type MPQ struct {
	f          *os.File
	sectorSize int
	hashes     []uint32 // 4 words per entry: nameA, nameB, locale|platform, block index
	blocks     []uint32 // 4 words per entry: offset, compressed size, size, flags
}

// MPQPaths are where the game looks for DIABDAT.MPQ (you bring your own copy of Diablo).
var MPQPaths = []string{"DIABDAT.MPQ", "reference/DIABDAT.MPQ", "../reference/DIABDAT.MPQ"}

var (
	mpqOnce sync.Once
	mpqArc  *MPQ
	mpqErr  error
)

// DiabDat opens the first DIABDAT.MPQ found on MPQPaths, once.
func DiabDat() (*MPQ, error) {
	mpqOnce.Do(func() {
		mpqErr = fmt.Errorf("DIABDAT.MPQ not found (looked in %s)", strings.Join(MPQPaths, ", "))
		for _, p := range MPQPaths {
			if _, err := os.Stat(p); err == nil {
				mpqArc, mpqErr = OpenMPQ(p)
				return
			}
		}
	})
	return mpqArc, mpqErr
}

var cryptTable = func() (t [0x500]uint32) {
	seed := uint32(0x00100001)
	for i1 := 0; i1 < 0x100; i1++ {
		for i, i2 := 0, i1; i < 5; i, i2 = i+1, i2+0x100 {
			seed = (seed*125 + 3) % 0x2AAAAB
			hi := (seed & 0xFFFF) << 16
			seed = (seed*125 + 3) % 0x2AAAAB
			t[i2] = hi | seed&0xFFFF
		}
	}
	return
}()

func hashString(s string, kind uint32) uint32 {
	seed1, seed2 := uint32(0x7FED7FED), uint32(0xEEEEEEEE)
	for _, c := range []byte(strings.ToUpper(s)) {
		seed1 = cryptTable[kind<<8+uint32(c)] ^ (seed1 + seed2)
		seed2 = uint32(c) + seed1 + seed2 + seed2<<5 + 3
	}
	return seed1
}

func decrypt(data []uint32, key uint32) {
	seed := uint32(0xEEEEEEEE)
	for i := range data {
		seed += cryptTable[0x400+key&0xFF]
		c := data[i] ^ (key + seed)
		key = (^key<<21 + 0x11111111) | key>>11
		seed = c + seed + seed<<5 + 3
		data[i] = c
	}
}

func OpenMPQ(path string) (*MPQ, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var h struct {
		Magic, HeaderSize, ArchiveSize uint32
		Version, BlockShift            uint16
		HashPos, BlockPos              uint32
		HashCount, BlockCount          uint32
	}
	if err := binary.Read(f, binary.LittleEndian, &h); err != nil || h.Magic != 0x1A51504D {
		f.Close()
		return nil, fmt.Errorf("%s: not an MPQ archive", path)
	}
	a := &MPQ{f: f, sectorSize: 512 << h.BlockShift}
	if a.hashes, err = a.table(h.HashPos, h.HashCount, "(hash table)"); err == nil {
		a.blocks, err = a.table(h.BlockPos, h.BlockCount, "(block table)")
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return a, nil
}

func (a *MPQ) table(pos, count uint32, name string) ([]uint32, error) {
	t := make([]uint32, 4*count)
	if err := binary.Read(io.NewSectionReader(a.f, int64(pos), int64(16*count)), binary.LittleEndian, t); err != nil {
		return nil, err
	}
	decrypt(t, hashString(name, 3))
	return t, nil
}

const (
	mpqImplode    = 0x100
	mpqCompress   = 0x200
	mpqEncrypted  = 0x10000
	mpqFixKey     = 0x20000
	mpqSingleUnit = 0x1000000
)

// ReadFile returns a file's contents by its archive name, e.g. `Levels\L1Data\L1.TIL`.
func (a *MPQ) ReadFile(name string) ([]byte, error) {
	n := uint32(len(a.hashes) / 4)
	start := hashString(name, 0) & (n - 1)
	nameA, nameB := hashString(name, 1), hashString(name, 2)
	for i := start; ; {
		e := a.hashes[4*i:]
		if e[3] == 0xFFFFFFFF {
			break
		}
		if e[0] == nameA && e[1] == nameB && e[3] < uint32(len(a.blocks)/4) {
			return a.readBlock(name, a.blocks[4*e[3]:])
		}
		if i = (i + 1) & (n - 1); i == start {
			break
		}
	}
	return nil, fmt.Errorf("%s: not in MPQ", name)
}

func (a *MPQ) readBlock(name string, b []uint32) ([]byte, error) {
	pos, csize, size, flags := b[0], b[1], b[2], b[3]
	raw := make([]byte, csize)
	if _, err := a.f.ReadAt(raw, int64(pos)); err != nil {
		return nil, err
	}
	key := hashString(filepath.Base(strings.ReplaceAll(name, `\`, "/")), 3)
	if flags&mpqFixKey != 0 {
		key = (key + pos) ^ size
	}
	if flags&mpqSingleUnit != 0 || flags&(mpqImplode|mpqCompress) == 0 {
		if flags&mpqEncrypted != 0 {
			decryptBytes(raw, key)
		}
		return a.unpack(raw, int(size), flags)
	}
	nSectors := (int(size) + a.sectorSize - 1) / a.sectorSize
	offsets := make([]uint32, nSectors+1)
	for i := range offsets {
		offsets[i] = binary.LittleEndian.Uint32(raw[4*i:])
	}
	if flags&mpqEncrypted != 0 {
		decrypt(offsets, key-1)
	}
	out := make([]byte, 0, size)
	for i := 0; i < nSectors; i++ {
		if offsets[i+1] > csize || offsets[i] > offsets[i+1] {
			return nil, fmt.Errorf("%s: bad sector table", name)
		}
		sec := raw[offsets[i]:offsets[i+1]]
		if flags&mpqEncrypted != 0 {
			decryptBytes(sec, key+uint32(i))
		}
		data, err := a.unpack(sec, min(a.sectorSize, int(size)-len(out)), flags)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out = append(out, data...)
	}
	return out, nil
}

func decryptBytes(b []byte, key uint32) {
	words := make([]uint32, len(b)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(b[4*i:])
	}
	decrypt(words, key)
	for i, w := range words {
		binary.LittleEndian.PutUint32(b[4*i:], w)
	}
}

// unpack decompresses one sector; a sector as long as its content is stored raw.
func (a *MPQ) unpack(b []byte, size int, flags uint32) ([]byte, error) {
	if len(b) >= size {
		return b[:size], nil
	}
	if flags&mpqCompress != 0 {
		if len(b) == 0 || b[0] != 0x08 { // D1 only uses PKWARE implode
			return nil, errors.New("unsupported MPQ compression")
		}
		b = b[1:]
	}
	return explode(b, size)
}

// explode decompresses PKWARE DCL "implode" data (after Mark Adler's blast.c).
func explode(in []byte, size int) ([]byte, error) {
	if len(in) < 2 || in[0] > 1 || in[1] < 4 || in[1] > 6 {
		return nil, errors.New("bad implode header")
	}
	lit, dict := in[0] == 1, uint(in[1])
	pos, bitbuf, bitcnt := 2, 0, uint(0)
	bits := func(need uint) (int, bool) {
		for bitcnt < need {
			if pos >= len(in) {
				return 0, false
			}
			bitbuf |= int(in[pos]) << bitcnt
			pos++
			bitcnt += 8
		}
		v := bitbuf & (1<<need - 1)
		bitbuf >>= need
		bitcnt -= need
		return v, true
	}
	decode := func(h *huffman) (int, bool) {
		code, first, index := 0, 0, 0
		for l := 1; l <= 13; l++ {
			b, ok := bits(1)
			if !ok {
				return 0, false
			}
			code |= b ^ 1 // codes are stored bit-inverted
			count := h.count[l]
			if code < first+count {
				return h.symbol[index+code-first], true
			}
			index += count
			first = (first + count) << 1
			code <<= 1
		}
		return 0, false
	}
	out := make([]byte, 0, size)
	for len(out) < size {
		b, ok := bits(1)
		if !ok {
			break
		}
		if b == 0 {
			var c int
			if lit {
				c, ok = decode(&litCode)
			} else {
				c, ok = bits(8)
			}
			if !ok {
				break
			}
			out = append(out, byte(c))
			continue
		}
		sym, ok := decode(&lenCode)
		if !ok {
			break
		}
		extra, ok := bits(uint(lenExtra[sym]))
		if !ok {
			break
		}
		length := lenBase[sym] + extra
		if length == 519 { // end code
			break
		}
		var s uint = 2
		if length != 2 {
			s = dict
		}
		d, ok := decode(&distCode)
		if !ok {
			break
		}
		lo, ok := bits(s)
		if !ok {
			break
		}
		dist := d<<s + lo + 1
		if dist > len(out) {
			return nil, errors.New("implode distance too far back")
		}
		for ; length > 0; length-- {
			out = append(out, out[len(out)-dist])
		}
	}
	if len(out) != size {
		return nil, fmt.Errorf("imploded data: got %d bytes, want %d", len(out), size)
	}
	return out, nil
}

type huffman struct {
	count  [14]int
	symbol []int
}

// newHuffman builds a canonical code from blast.c's compact run-length form.
func newHuffman(rep []byte) huffman {
	var length []int
	for _, r := range rep {
		n := int(r>>4) + 1
		for ; n > 0; n-- {
			length = append(length, int(r&15))
		}
	}
	h := huffman{symbol: make([]int, len(length))}
	for _, l := range length {
		h.count[l]++
	}
	var offs [14]int
	for l := 1; l < 13; l++ {
		offs[l+1] = offs[l] + h.count[l]
	}
	for sym, l := range length {
		h.symbol[offs[l]] = sym
		offs[l]++
	}
	return h
}

var (
	litCode = newHuffman([]byte{11, 124, 8, 7, 28, 7, 188, 13, 76, 4, 10, 8, 12, 10, 12, 10, 8, 23, 8, 9, 7, 6, 7, 8, 7, 6, 55, 8,
		23, 24, 12, 11, 7, 9, 11, 12, 6, 7, 22, 5, 7, 24, 6, 11, 9, 6, 7, 22, 7, 11, 38, 7, 9, 8, 25, 11, 8, 11, 9, 12, 8, 12,
		5, 38, 5, 38, 5, 11, 7, 5, 6, 21, 6, 10, 53, 8, 7, 24, 10, 27, 44, 253, 253, 253, 252, 252, 252, 13, 12, 45, 12, 45,
		12, 61, 12, 45, 44, 173})
	lenCode  = newHuffman([]byte{2, 35, 36, 53, 38, 23})
	distCode = newHuffman([]byte{2, 20, 53, 230, 247, 151, 248})
	lenBase  = [16]int{3, 2, 4, 5, 6, 7, 8, 9, 10, 12, 16, 24, 40, 72, 136, 264}
	lenExtra = [16]int{0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8}
)
