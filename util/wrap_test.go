package util

import (
	"testing"
)

func TestWrapLabelText(t *testing.T) {
	measure := func(s string) (float64, float64) { return float64(len(s)), 1 }
	got := WrapLabelText(ToLabelText([]string{"aaa bbb ccc", "", "a  b"}), 7, measure)
	want := []string{"aaa bbb", "ccc", "", "a  b"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i].Text != want[i] {
			t.Fatalf("line %d: got %q want %q", i, got[i].Text, want[i])
		}
	}
}
