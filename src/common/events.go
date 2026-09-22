package common

import "rpg/src/geometry"

// An Event is something that happened, as the rules report it through Broker.Emit. The rules never build an
// animation: the broker's presenter (present.go) turns each event into what the player sees and hears, at the
// point the action that emitted it finishes, which is where the action's own animations used to start.
type Event interface{ event() }

// MeleeSwing is a melee blow from From at To.
type MeleeSwing struct {
	Attacker *Actor
	From, To geometry.Point
}

// HealthChanged is a change to an actor's health (Amount < 0: damage). Before is the health it had, to draw
// while the blow is still on its way, and Blood is where a hit stains the floor - the cell is rolled here
// because it is saved with the map, while when it appears is the presenter's business.
type HealthChanged struct {
	Actor  *Actor
	At     geometry.Point
	Amount float64
	Before int
	Cause  ActionName
	Blood  geometry.Point
}

// Moved is a step (or push, or teleport) from From to To. The player sees it animated, over Delay seconds, when
// it is theirs or they can see either end; Quiet ones just appear there.
type Moved struct {
	Actor    *Actor
	From, To geometry.Point
	Delay    float64
	ByPlayer bool
	Quiet    bool
}

// Swapped is two actors trading places.
type Swapped struct {
	One, Two *Actor
	Delay    float64
	ByPlayer bool
	Quiet    bool
}

// Shot is a missile loosed by Attacker from From at To (a bow, or a D1 monster's ranged attack).
type Shot struct {
	Attacker *Actor
	From, To geometry.Point
}

// Thrown is Item flying from From to To.
type Thrown struct {
	Thrower  *Actor
	Item     *Item
	From, To geometry.Point
}

// Fireball is a damage spell's missile flying from Caster to To, bursting over Hits.
type Fireball struct {
	Caster *Actor
	To     geometry.Point
	Icon   int32
	Frames int
	Hits   []geometry.Point
}

// KnockedBack is an actor pushed from From to To, Delay seconds per tile.
type KnockedBack struct {
	Actor    *Actor
	From, To geometry.Point
	Delay    float64
	Quiet    bool
}

// Missile is a missile's flight along Path, SecPerTile a tile; ArriveSfx, when set, plays where it lands. By is
// who loosed it (nil for a trap).
type Missile struct {
	By         *Actor
	Path       []geometry.Point
	SecPerTile float64
	Icon       int32
	ArriveSfx  string
}

// Flash is a hit or spell effect on At for Seconds.
type Flash struct {
	At      geometry.Point
	Seconds float64
	Icon    int32
}

// Said is a line an actor calls out on coming into view.
type Said struct {
	Actor *Actor
	Line  string
}

func (Missile) event()       {}
func (Flash) event()         {}
func (Said) event()          {}
func (KnockedBack) event()   {}
func (Shot) event()          {}
func (Thrown) event()        {}
func (Fireball) event()      {}
func (Moved) event()         {}
func (Swapped) event()       {}
func (MeleeSwing) event()    {}
func (HealthChanged) event() {}
