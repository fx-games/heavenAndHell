package d1

import (
	"fmt"
	"io/fs"
)

// SfxFiles is every speech and quest sound id D1 has (_sfx_id, effects.cpp's sgSFX), mapped to the file it was in
// Diablo (the .wav path lowercased, as .ogg). The table lives in mods/d1/data/sfx.rec and is filled by Load. D1's
// own audio is not in this game: the ids are the records of assets/data/voices.rec, which is what the game plays,
// and the file names are only the key into reference/d1_voice_transcripts.tsv, the transcriptions of the cut lines.
var SfxFiles map[string]string

func loadSfxFiles(fsys fs.FS) (map[string]string, error) {
	recs, err := readRec(fsys, "sfx.rec")
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
