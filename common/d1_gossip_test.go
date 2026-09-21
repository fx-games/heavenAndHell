package common

import "testing"

func TestD1GossipRepeatsUntilCleared(t *testing.T) {
	g := map[string][]string{"x": {"hi", "a", "b", "c", "d", "e", "f", "g"}}
	first := d1Gossip(g, "x")
	for range 20 {
		if d1Gossip(g, "x") != first {
			t.Fatal("gossip changed within one town visit")
		}
	}
	clear(d1GossipSeeds)
}
