package common

import "testing"

func TestD1GossipRepeatsUntilCleared(t *testing.T) {
	g := map[string][]string{"x": {"hi", "a", "b", "c", "d", "e", "f", "g"}}
	seeds := map[VendorType]int64{}
	first := d1Gossip(g, seeds, "x")
	for range 20 {
		if d1Gossip(g, seeds, "x") != first {
			t.Fatal("gossip changed within one town visit")
		}
	}
}
