package rules

import (
	"errors"
	"fmt"
	"io/fs"
)

func loadSfxFiles(fsys fs.FS) (map[string]string, error) {
	recs, err := ReadRec(fsys, "sfx.rec")
	// A mod that does not have this Diablo feature ships no file for it, and an absent feature is not an
	// error: what the table would have filled stays empty.
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(recs))
	for _, r := range recs {
		m := r.ToMap("")
		id := m.GetStringOrDefault("id", "")
		if id == "" {
			return nil, fmt.Errorf("sfx.rec: record with no id")
		}
		out[id] = m.GetStringOrDefault("file", "")
	}
	return out, nil
}
