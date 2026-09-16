package d1

import (
	"os"
	"strings"
	"testing"
)

// TestParseDunText checks the round trip and that a corrupted file fails loudly instead of building a subtly
// different dungeon.
func TestParseDunText(t *testing.T) {
	text := `# test  3x2 megatiles
[tiles] 3 2
 1  2  3
 4  5  0

[monsters]
x	y	id
1	2	42

[transparency] 6 4
0 0 0 0 0 0
0 1 1 1 0 0
0 1 1 1 0 0
0 0 0 0 0 0
`
	d, err := parseDunText([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	if d.w() != 3 || d.h() != 2 {
		t.Fatalf("%dx%d, want 3x2", d.w(), d.h())
	}
	for i, want := range []byte{1, 2, 3, 4, 5, 0} {
		if got := d.tile(i%3, i/3); got != want {
			t.Errorf("tile %d,%d = %d, want %d", i%3, i/3, got, want)
		}
	}
	if got := d.layer(1, 1, 2); got != 42 {
		t.Errorf("monster at 1,2 = %d, want 42", got)
	}
	if got := d.layer(3, 1, 1); got != 1 {
		t.Errorf("transparency at 1,1 = %d, want 1", got)
	}
	if got := d.layer(3, 0, 0); got != 0 {
		t.Errorf("transparency at 0,0 = %d, want 0", got)
	}
	// re-rendering what we parsed must produce the same set piece
	again, err := parseDunText([]byte(renderDun("test", d)))
	if err != nil {
		t.Fatal(err)
	}
	if err := sameDun(d, again); err != nil {
		t.Error(err)
	}

	// a row stripped of trailing whitespace still parses: the missing values are zeros
	short, err := parseDunText([]byte("[tiles] 3 2\n1 2 3\n4\n"))
	if err != nil {
		t.Fatal(err)
	}
	if short.tile(0, 1) != 4 || short.tile(1, 1) != 0 || short.tile(2, 1) != 0 {
		t.Errorf("short row padded wrong: %d %d %d", short.tile(0, 1), short.tile(1, 1), short.tile(2, 1))
	}

	for _, bad := range []struct{ name, text string }{
		{"missing row", "[tiles] 3 2\n1 2 3\n"},
		{"extra row", "[tiles] 3 2\n1 2 3\n4 5 6\n7 8 9\n"},
		{"too many columns", "[tiles] 3 2\n1 2 3 4\n5 6 7\n"},
		{"not a number", "[tiles] 3 2\n1 2 x\n4 5 6\n"},
		{"no tiles section", "[monsters]\nx\ty\tid\n1\t2\t3\n"},
		{"unknown section", "[tiles] 1 1\n1\n\n[goblins]\nx\ty\tid\n0\t0\t1\n"},
		{"cell out of range", "[tiles] 1 1\n1\n\n[monsters]\nx\ty\tid\n99\t0\t1\n"},
		{"missing tsv header", "[tiles] 1 1\n1\n\n[monsters]\n0\t0\t1\n"},
		{"transparency wrong size", "[tiles] 1 1\n1\n\n[transparency] 3 3\n0 0 0\n0 0 0\n0 0 0\n"},
	} {
		if _, err := parseDunText([]byte(bad.text)); err == nil {
			t.Errorf("%s: parsed without error", bad.name)
		}
	}
}

func TestParseTilSol(t *testing.T) {
	til, err := parseTil([]byte("id\tnw\tne\tsw\tse\n1\t10\t11\t12\t13\n2\t20\t21\t22\t23\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := megaPieces(til, 1); got != [4]int{20, 21, 22, 23} {
		t.Errorf("megatile 2 = %v, want [20 21 22 23]", got)
	}
	sol, err := parseSol([]byte("piece\tflags\n0\t0x00\n1\t0x2F\n2\t0x80\n"))
	if err != nil {
		t.Fatal(err)
	}
	if sol[1] != 0x2F || sol[2] != 0x80 {
		t.Errorf("sol = %v, want [0 0x2F 0x80]", sol)
	}
	for _, bad := range []struct{ name, text string }{
		{"til rows out of order", "id\tnw\tne\tsw\tse\n2\t1\t2\t3\t4\n"},
		{"til short row", "id\tnw\tne\tsw\tse\n1\t1\t2\t3\n"},
		{"til wrong header", "id\ta\tb\tc\td\n1\t1\t2\t3\t4\n"},
		{"sol rows out of order", "piece\tflags\n1\t0x00\n"},
		{"sol flags too big", "piece\tflags\n0\t0x1FF\n"},
		{"no header", "1\t2\t3\t4\t5\n"},
	} {
		parse := parseTil
		if strings.HasPrefix(bad.name, "sol") {
			parse = parseSol
		}
		if _, err := parse([]byte(bad.text)); err == nil {
			t.Errorf("%s: parsed without error", bad.name)
		}
	}
}

// TestLevelDataLoads checks every imported file parses and the tables are the sizes D1's are.
func TestLevelDataLoads(t *testing.T) {
	ld, err := loadLevelData(os.DirFS("../files"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ld.dun) != 29 { // D1's 27 and the Crypt's two set rooms
		t.Errorf("%d set pieces, want 29", len(ld.dun))
	}
	for lt, want := range map[int][2]int{1: {206, 453}, 2: {160, 559}, 3: {156, 560}, 4: {137, 456}} {
		if got := len(ld.til[lt]) / 8; got != want[0] {
			t.Errorf("L%d: %d megatiles, want %d", lt, got, want[0])
		}
		if got := len(ld.sol[lt]); got != want[1] {
			t.Errorf("L%d: %d pieces in sol, want %d", lt, got, want[1])
		}
	}
	if d := ld.dun["l1/sklkng1"]; d.w() != 37 || d.h() != 25 {
		t.Errorf("l1/sklkng1 is %dx%d, want 37x25", d.w(), d.h())
	}
}
