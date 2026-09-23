package rules

import (
	"errors"
	"fmt"
	"io/fs"
)

// MissileData is a missiles.rec record: a monster or trap missile's resistance, sounds and speed (misdat).
type MissileData struct {
	Resist               int    // Resist* its damage is checked against
	LaunchSfx, ImpactSfx string // sfx.rec ids; blank is silent
	Velocity             int    // AddMissile's velocity; the flight speed comes from it
}

func loadMissiles(fsys fs.FS, d *Data) error {
	d.Missiles = map[string]MissileData{}
	recs, err := ReadRec(fsys, "missiles.rec")
	if errors.Is(err, fs.ErrNotExist) {
		return nil // a mod without monster missiles
	}
	if err != nil {
		return err
	}
	for _, r := range recs {
		m := r.ToMap("")
		id := m.GetStringOrDefault("id", "")
		resist, ok := resistKindByName[m.GetStringOrDefault("resist", "none")]
		if id == "" || !ok {
			return fmt.Errorf("missiles.rec: %q: want an id and a resist of none, fire, lightning or magic", id)
		}
		d.Missiles[id] = MissileData{Resist: resist, LaunchSfx: m.GetStringOrDefault("launch_sfx", ""),
			ImpactSfx: m.GetStringOrDefault("impact_sfx", ""), Velocity: m.GetIntOrDefault("velocity", 16)}
	}
	return nil
}
