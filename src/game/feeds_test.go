package game

import "testing"

func TestANSI(t *testing.T) {
	got := ansi("[:red]a\n[:1,2,3]b[:nope]c")
	want := "\x1b[31ma\x1b[0m\n\x1b[38;2;1;2;3mb\x1b[0mc\x1b[0m"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestHeadings(t *testing.T) {
	got := headings([]string{"Title", "", " Life: 10"})
	if got[0] != "[:lightText]Title[:white]" || got[1] != "" || got[2] != "[:darkText] Life:[:white] 10" {
		t.Fatalf("got %q", got)
	}
}
