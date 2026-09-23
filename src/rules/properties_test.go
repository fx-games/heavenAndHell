package rules

import (
	"reflect"
	"testing"
)

func TestCompare(t *testing.T) {
	d := testData(t)
	shop := &Item{MinDam: 6, MaxDam: 9, PLStr: 5, PLFR: 10, PLLR: 10, PLMR: 10, Flags: FlagStealHealth5 | FlagNoHealPlayer,
		FMinDam: 1, FMaxDam: 4}
	shop.Flags |= FlagFireDamage
	worn := &Item{MinDam: 4, MaxDam: 12, PLStr: 10, PLFR: 10, PLLR: 5, PLMR: 10, PLGetHit: 2, Flags: FlagStealHealth3}

	want := map[string]int{KeyDamage: -1, "str": -1, "fr": 0, "lr": 1, "mr": 0, "stealslife": 1, "nohealplr": -1, "firedam": 1}
	if got := d.Compare(shop, worn, true); !reflect.DeepEqual(got, want) {
		t.Fatalf("shop vs worn = %v", got)
	}
	want = map[string]int{KeyDamage: 1, "str": 1, "fr": 0, "lr": -1, "mr": 0, "gethit": -1, "stealslife": -1}
	if got := d.Compare(worn, shop, true); !reflect.DeepEqual(got, want) {
		t.Fatalf("worn vs shop = %v", got)
	}
	if got := d.Compare(shop, worn, false); !reflect.DeepEqual(got, map[string]int{KeyDamage: -1}) {
		t.Fatalf("unidentified: affixes must not be compared: %v", got)
	}

	// resists only merge into Resist All when they also compare alike
	if lines := d.PropertyLines(shop, nil); lines[0].Text != "Resist All: +10%" {
		t.Fatalf("lines = %v", lines)
	}
	if lines := d.PropertyLines(shop, d.Compare(shop, worn, true)); lines[0].Text != "Resist Fire: +10%" || lines[1] != (PropertyLine{"Resist Lightning: +10%", 1}) {
		t.Fatalf("lines = %v", lines)
	}
}
