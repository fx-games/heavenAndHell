package gui

import (
	"testing"

	"rpg/src/common"
	"rpg/src/geometry"
)

type fakeRules struct{ targets []geometry.Point }

func (f fakeRules) OnTargetChanged(p geometry.Point) common.TargetInfo {
	return common.TargetInfo{AdjustedTarget: p, IsValid: true}
}
func (f fakeRules) GetPreSelectedTarget() geometry.Point { return f.targets[0] }
func (f fakeRules) GetAttacker() *common.Actor           { return nil }
func (f fakeRules) Targets() []geometry.Point            { return f.targets }

func TestTargeterCyclesAndWraps(t *testing.T) {
	a, b, c := geometry.Point{X: 1}, geometry.Point{X: 2}, geometry.Point{X: 3}
	tg := &Targeter{rules: fakeRules{[]geometry.Point{a, b, c}}}
	tg.updateTarget(a)
	for _, step := range []struct {
		dir  int
		want geometry.Point
	}{{1, b}, {1, c}, {1, a}, {-1, c}, {-1, b}} {
		tg.Cycle(step.dir)
		if got := tg.targetInfo.AdjustedTarget; got != step.want {
			t.Fatalf("cycle %+d landed on %v, want %v", step.dir, got, step.want)
		}
	}
	tg.updateTarget(geometry.Point{X: 9}) // cursor moved off every target
	if tg.Cycle(-1); tg.targetInfo.AdjustedTarget != c {
		t.Fatalf("shift+tab off-target should pick the farthest, got %v", tg.targetInfo.AdjustedTarget)
	}
}
