package d1

import (
	"reflect"
	"testing"
)

func TestCompare(t *testing.T) {
	shop := &Item{MinDam: 6, MaxDam: 9, PLStr: 5, PLFR: 10, PLLR: 10, PLMR: 10, Flags: ISplStealLife5 | ISplNoHealPlr,
		FMinDam: 1, FMaxDam: 4}
	shop.Flags |= ISplFireDam
	worn := &Item{MinDam: 4, MaxDam: 12, PLStr: 10, PLFR: 10, PLLR: 5, PLMR: 10, PLGetHit: 2, Flags: ISplStealLife3}

	want := map[string]int{KeyDamage: -1, "str": -1, "fr": 0, "lr": 1, "mr": 0, "stealslife": 1, "nohealplr": -1, "firedam": 1}
	if got := shop.Compare(worn, true); !reflect.DeepEqual(got, want) {
		t.Fatalf("shop vs worn = %v", got)
	}
	want = map[string]int{KeyDamage: 1, "str": 1, "fr": 0, "lr": -1, "mr": 0, "gethit": -1, "stealslife": -1}
	if got := worn.Compare(shop, true); !reflect.DeepEqual(got, want) {
		t.Fatalf("worn vs shop = %v", got)
	}
	if got := shop.Compare(worn, false); !reflect.DeepEqual(got, map[string]int{KeyDamage: -1}) {
		t.Fatalf("unidentified: affixes must not be compared: %v", got)
	}

	// resists only merge into Resist All when they also compare alike
	if lines := shop.PropertyLines(nil); lines[0].Text != "Resist All: +10%" {
		t.Fatalf("lines = %v", lines)
	}
	if lines := shop.PropertyLines(shop.Compare(worn, true)); lines[0].Text != "Resist Fire: +10%" || lines[1] != (PropertyLine{"Resist Lightning: +10%", 1}) {
		t.Fatalf("lines = %v", lines)
	}
}
