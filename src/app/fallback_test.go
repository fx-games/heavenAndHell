package app

import "testing"

// There is no engine fallback game: a missing mod is an error that names it, and every shipped mod is there.
func TestMissingModIsAnError(t *testing.T) {
	if err := modError("no_such_mod"); err == nil {
		t.Error("a missing mod passed")
	}
	for _, mod := range []string{"d1", "heavenandhell"} {
		if err := modError(mod); err != nil {
			t.Error(err)
		}
	}
}
