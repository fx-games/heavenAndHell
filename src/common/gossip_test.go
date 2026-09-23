package common

import "testing"

func TestGossipRepeatsUntilCleared(t *testing.T) {
	g := map[string][]string{"x": {"hi", "a", "b", "c", "d", "e", "f", "g"}}
	seeds := map[VendorType]int64{}
	first := gossip(g, seeds, "x")
	for range 20 {
		if gossip(g, seeds, "x") != first {
			t.Fatal("gossip changed within one town visit")
		}
	}
}
