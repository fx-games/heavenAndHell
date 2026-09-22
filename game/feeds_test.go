package game

import "testing"

func TestANSI(t *testing.T) {
	got := ansi("[:red]a\n[:1,2,3]b[:nope]c")
	want := "\x1b[31ma\x1b[0m\n\x1b[38;2;1;2;3mb\x1b[0mc\x1b[0m"
	if got != want {
		t.Fatalf("got %q", got)
	}
}
