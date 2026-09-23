package app

import (
	"strings"
	"testing"
)

// Every H&H potion and scroll (bar the letters, name-only by design) says what it does in its tooltip, not just its name.
func TestHeavenAndHellConsumablesDescribeTheirEffect(t *testing.T) {
	g := newHeavenAndHellGame(t, NewDummyUI())
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	f := g.GetFactory()
	for i := range f.Generator.D.Items {
		b := &f.Generator.D.Items[i]
		if !b.IsPotion() && !b.IsScroll() || b.UntargetedAction+b.TargetedAction == "" || strings.HasPrefix(b.UntargetedAction, "read_message") {
			continue
		}
		it := f.NewRolledItem(f.Generator.PlainItem(b))
		lines := it.GetTooltipLines(nil)
		if len(lines) < 3 {
			t.Errorf("%s: tooltip has no effect line: %v", b.Name, lines)
		}
	}
}
